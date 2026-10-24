import { beforeEach, describe, expect, it } from "vitest";
import { encryptToken } from "../src/api/crypto";
import { formatTimestamp } from "../src/api/posts/post";
import { handleWorkerRoute } from "../src/api/router";
import type { ApiEnv } from "../src/api/types";
import { FakeD1 } from "./fake-d1";

const KEY = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f";
const BASE = "https://begit.example";

let d1: FakeD1;
let env: ApiEnv;

// 投稿者（poster）・閲覧者（viewer）・旧 role（member）・グループ外の4人と、グループ2つを作る
beforeEach(async () => {
  d1 = new FakeD1();
  env = { DB: d1.asD1(), DB_ENCRYPTION_KEY: KEY, WORKER_ROUTES: "posts.confirm" };
  const users = ["poster", "viewer", "legacy", "outsider", "owner2"];
  for (const [i, login] of users.entries()) {
    d1.sqlite.prepare(
      "INSERT INTO users (id, github_id, github_login, encrypted_access_token) VALUES (?, ?, ?, ?)",
    ).run(i + 1, 1000 + i, login, await encryptToken(KEY, `dev_${login}`));
  }
  d1.sqlite.exec(`
    INSERT INTO groups (id, repo_full_name, owner_user_id) VALUES (1, 'o/r1', 1), (2, 'o/r2', 5);
    INSERT INTO group_members (group_id, user_id, role) VALUES
      (1, 1, 'poster'), (1, 2, 'viewer'), (1, 3, 'member'), (2, 5, 'owner');
    INSERT INTO sprints (id, group_id, ends_at) VALUES (1, 1, '2099-01-01 00:00:00');
    INSERT INTO notifications (id, sprint_id, sent_by) VALUES (1, 1, 1);
    INSERT INTO posts (id, user_id, group_id, post_type, body, is_draft, created_at) VALUES
      (10, 1, 1, 'commit', 'draft body', 1, '2026-10-20 17:15:11'),
      (11, 1, 1, 'commit', 'done', 0, '2026-10-20T17:15:11+09:00'),
      (12, 1, 1, 'memo', NULL, 1, '2026-10-20 17:15:11'),
      (13, 3, 1, 'commit', NULL, 1, '2026-10-20 17:15:11'),
      (20, 5, 2, 'commit', NULL, 1, '2026-10-20 17:15:11');
    UPDATE posts SET notification_id = 1 WHERE id = 12;
  `);
  d1.roundTrips = 0;
});

function confirm(
  path: string,
  { token = "dev_poster", body, headers = {}, contentLength = true }: { token?: string | null; body?: string; headers?: Record<string, string>; contentLength?: boolean } = {},
) {
  const init: RequestInit = { method: "POST", headers: { ...headers } };
  if (token !== null) (init.headers as Record<string, string>).Authorization = `Bearer ${token}`;
  if (body !== undefined) {
    init.body = body;
    if (contentLength) (init.headers as Record<string, string>)["Content-Length"] = String(new TextEncoder().encode(body).length);
  }
  return handleWorkerRoute(new Request(`${BASE}${path}`, init), env);
}

async function json(response: Response | null) {
  expect(response).not.toBeNull();
  return { status: response!.status, body: await response!.json() };
}

describe("振り分け", () => {
  it("WORKER_ROUTES が空ならコンテナへ回す（null を返す）", async () => {
    env.WORKER_ROUTES = "";
    expect(await confirm("/groups/1/posts/10/confirm")).toBeNull();
  });

  it("WORKER_ROUTES にない API、メソッド違い、形の違うパスはコンテナへ回す", async () => {
    env.WORKER_ROUTES = "posts.list";
    expect(await confirm("/groups/1/posts/10/confirm")).toBeNull();
    env.WORKER_ROUTES = " posts.confirm ,other";
    expect(await handleWorkerRoute(new Request(`${BASE}/groups/1/posts/10/confirm`), env)).toBeNull();
    expect(await confirm("/groups/1/posts/10/confirm/")).toBeNull();
    expect(await confirm("/internal/cron")).toBeNull();
  });

  it("パーセントエンコードを含むパスはコンテナへ回す", async () => {
    expect(await confirm("/groups/%31/posts/10/confirm")).toBeNull();
    expect(await confirm("/groups/1/posts/1%30/confirm")).toBeNull();
  });
});

describe("認証と権限チェック（Go の BearerAuth / GroupRole と同じ）", () => {
  it.each([
    ["Authorization なし", null],
    ["トークンが空", ""],
    ["一致するユーザーがいない", "dev_nobody"],
  ])("401：%s", async (_, token) => {
    expect(await json(await confirm("/groups/1/posts/10/confirm", { token }))).toEqual({ status: 401, body: { error: "unauthorized" } });
  });

  it("401：Bearer の大文字小文字が違う", async () => {
    const res = await confirm("/groups/1/posts/10/confirm", { token: null, headers: { Authorization: "bearer dev_poster" } });
    expect((await json(res)).status).toBe(401);
  });

  it("グループ ID が不正でも、認証の失敗を先に返す", async () => {
    expect((await json(await confirm("/groups/abc/posts/10/confirm", { token: "dev_nobody" }))).status).toBe(401);
  });

  it.each(["abc", "1.5", "99999999999999999999"])("400：グループ ID が整数でない（%s）", async (id) => {
    expect(await json(await confirm(`/groups/${id}/posts/10/confirm`))).toEqual({ status: 400, body: { error: "invalid group id" } });
  });

  it.each([
    ["グループ外", "dev_outsider", "1"],
    ["viewer", "dev_viewer", "1"],
    ["存在しないグループ", "dev_poster", "999"],
    ["JavaScript の数値で表せない ID", "dev_poster", "9007199254740993"],
  ])("403：%s", async (_, token, groupID) => {
    expect(await json(await confirm(`/groups/${groupID}/posts/10/confirm`, { token }))).toEqual({ status: 403, body: { error: "forbidden" } });
  });

  it("旧 role の member は poster として通る", async () => {
    expect((await json(await confirm("/groups/1/posts/13/confirm", { token: "dev_legacy" }))).status).toBe(200);
  });
});

describe("投稿確定（Go の postService.ConfirmPost と同じ）", () => {
  it("下書きを確定し、本文を上書きする。認証・投稿の取得・batch() の3往復", async () => {
    const { status, body } = await json(await confirm("/groups/1/posts/10/confirm", { body: JSON.stringify({ body: "new" }) }));
    expect(status).toBe(200);
    expect(body).toEqual({
      id: 10,
      notification_id: null,
      user_id: 1,
      post_type: "commit",
      body: "new",
      repo_full_name: null,
      commit_count: 0,
      additions: 0,
      deletions: 0,
      latest_commit_message: null,
      status: null,
      created_at: "2026-10-20T17:15:11Z",
    });
    expect(d1.sqlite.prepare("SELECT is_draft FROM posts WHERE id = 10").get()).toEqual({ is_draft: 0 });
    expect(d1.roundTrips).toBe(3);
  });

  it.each([
    ["ボディなし", undefined],
    ["空のオブジェクト", "{}"],
    ["body が null", '{"body":null}'],
    ["null", "null"],
  ])("本文を変えずに確定する（%s）", async (_, body) => {
    const res = await json(await confirm("/groups/1/posts/10/confirm", { body }));
    expect(res.status).toBe(200);
    expect(res.body.body).toBe("draft body");
  });

  it("確定済みならそのまま返し、本文は無視する。時刻は UTC にする", async () => {
    const res = await json(await confirm("/groups/1/posts/11/confirm", { body: '{"body":"ignored"}' }));
    expect(res.status).toBe(200);
    expect(res.body.body).toBe("done");
    expect(res.body.created_at).toBe("2026-10-20T08:15:11Z");
  });

  it.each([
    ["壊れた JSON", "{"],
    ["配列", "[]"],
    ["body が文字列でない", '{"body":1}'],
  ])("400：%s", async (_, body) => {
    expect(await json(await confirm("/groups/1/posts/10/confirm", { body }))).toEqual({ status: 400, body: { error: "invalid request body" } });
  });

  it("Content-Length がなければ、読めない JSON は本文なしとして確定する（Go と同じ）", async () => {
    const res = await json(await confirm("/groups/1/posts/10/confirm", { body: "{", contentLength: false }));
    expect(res.status).toBe(200);
    expect(res.body.body).toBe("draft body");
  });

  it.each([
    ["大文字の Body", '{"Body":"upper"}', "upper"],
    ["後に出たキーを使う", '{"body":"first","BODY":"last"}', "last"],
    ["後に出た null で本文を変えない", '{"body":"first","Body":null}', "draft body"],
  ])("body のキーは大文字小文字を区別しない（%s）", async (_, body, expected) => {
    const res = await json(await confirm("/groups/1/posts/10/confirm", { body }));
    expect(res.status).toBe(200);
    expect(res.body.body).toBe(expected);
  });

  it("413：ボディが 64KB を超える", async () => {
    const body = JSON.stringify({ body: "a".repeat(64 * 1024) });
    expect(await json(await confirm("/groups/1/posts/10/confirm", { body }))).toEqual({ status: 413, body: { error: "request body too large" } });
    expect(await json(await confirm("/groups/1/posts/10/confirm", { body, contentLength: false }))).toEqual({ status: 413, body: { error: "request body too large" } });
  });

  it("400：投稿 ID が整数でない", async () => {
    expect(await json(await confirm("/groups/1/posts/x/confirm"))).toEqual({ status: 400, body: { error: "invalid post id" } });
  });

  it.each([
    ["別グループの投稿", "/groups/1/posts/20/confirm"],
    ["存在しない投稿", "/groups/1/posts/999/confirm"],
  ])("404：%s", async (_, path) => {
    expect(await json(await confirm(path))).toEqual({ status: 404, body: { error: "not found" } });
  });

  it("403：本人以外の投稿", async () => {
    expect(await json(await confirm("/groups/1/posts/13/confirm"))).toEqual({ status: 403, body: { error: "forbidden" } });
  });

  it("409：BeGit Time! の写真報告は写真のアップロードでだけ確定する", async () => {
    expect(await json(await confirm("/groups/1/posts/12/confirm"))).toEqual({ status: 409, body: { error: "photo upload required" } });
  });
});

describe("予期しない失敗", () => {
  it("D1 が失敗したら JSON の 500 を返し、書き込みは残さない", async () => {
    env.D1_METRICS = "true";
    d1.batch = async () => {
      throw new Error("D1 is down");
    };
    const res = (await confirm("/groups/1/posts/10/confirm"))!;
    expect(res.headers.get("X-D1-Status")).toBe("200,200,500");
    expect(await json(res)).toEqual({ status: 500, body: { error: "internal server error" } });
    expect(d1.sqlite.prepare("SELECT is_draft FROM posts WHERE id = 10").get()).toEqual({ is_draft: 1 });
  });

  it("ボディを読めなければ JSON の 500 を返す", async () => {
    const request = new Request(`${BASE}/groups/1/posts/10/confirm`, {
      method: "POST",
      headers: { Authorization: "Bearer dev_poster" },
      body: new ReadableStream({ pull: (controller) => controller.error(new Error("client went away")) }),
      duplex: "half",
    } as RequestInit);
    expect(await json(await handleWorkerRoute(request, env))).toEqual({ status: 500, body: { error: "internal server error" } });
  });
});

describe("formatTimestamp（Go の time.Parse と同じ）", () => {
  it.each([
    ["0099-01-01 00:00:00", "0099-01-01T00:00:00Z"],
    ["2026-10-20T00:30:00+09:00", "2026-10-19T15:30:00Z"],
    ["2026-10-20T23:30:00-01:00", "2026-10-21T00:30:00Z"],
    ["2026-10-20T17:15:11.123Z", "2026-10-20T17:15:11Z"],
  ])("%s → %s", (value, expected) => {
    expect(formatTimestamp(value)).toBe(expected);
  });

  it.each(["2026-02-30 00:00:00", "2026-13-01 00:00:00", "2026-10-20 24:00:00", "2026-10-20 00:60:00", "2026-10-20T00:00:00+24:00", "2026-10-20", null])(
    "範囲外や読めない値はゼロ時刻（%s）",
    (value) => {
      expect(formatTimestamp(value)).toBe("0001-01-01T00:00:00Z");
    },
  );
});

describe("D1 の計測（D1_METRICS=true）", () => {
  it("X-D1-* ヘッダーで往復の回数を返す", async () => {
    env.D1_METRICS = "true";
    const res = (await confirm("/groups/1/posts/10/confirm"))!;
    expect(res.headers.get("X-D1-Queries")).toBe("3");
    expect(res.headers.get("X-D1-Status")).toBe("200,200,200");
    expect(Number(res.headers.get("X-D1-Time-Ms"))).toBeGreaterThanOrEqual(0);
  });

  it("D1_METRICS がなければヘッダーを付けない", async () => {
    const res = (await confirm("/groups/1/posts/10/confirm"))!;
    expect(res.headers.get("X-D1-Queries")).toBeNull();
  });
});
