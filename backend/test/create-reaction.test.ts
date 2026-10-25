// 投稿作成（写真の下書き）とリアクションのうち、Go との比較（*-go.test.ts）では確かめられないこと。
// D1 の往復の回数、転送するときに D1 を使わないこと、同時に作られたとき、Queue の失敗など
import { beforeEach, describe, expect, it, vi } from "vitest";
import { decodeGoObject } from "../src/api/gojson";
import { handleWorkerRoute } from "../src/api/router";
import type { ApiEnv } from "../src/api/types";
import type { FakeD1 } from "./fake-d1";
import { FakeQueue, KEY, loadFixture, setupFixture } from "./go-fixture";

const BASE = "https://begit.example";
let d1: FakeD1;
let queue: FakeQueue;
let env: ApiEnv;

// 投稿作成の fixture のデータを使う（9202 が poster、通知 9201 は開催中、9207 には 9202 の下書き 920001 がある）
beforeEach(async () => {
  d1 = await setupFixture(loadFixture("create-go.json"));
  d1.sqlite.exec(`
    INSERT INTO posts (id, user_id, group_id, post_type, body, is_draft, created_at) VALUES
      (920100, 9203, 9201, 'commit', 'other post', 0, '2026-10-20 10:00:00'),
      (920101, 9202, 9201, 'commit', 'self post', 0, '2026-10-20 10:00:00');
  `);
  d1.roundTrips = 0;
  queue = new FakeQueue();
  env = { DB: d1.asD1(), DB_ENCRYPTION_KEY: KEY, WORKER_ROUTES: "posts.create,reactions.create", NOTIFICATION_QUEUE: queue.asQueue() };
});

function post(path: string, body: string | null, { token = "dev_pr3_poster", headers = {} }: { token?: string; headers?: Record<string, string> } = {}) {
  const init: RequestInit = { method: "POST", headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json", ...headers } };
  if (body !== null) init.body = body;
  return handleWorkerRoute(new Request(`${BASE}${path}`, init), env);
}

const photo = (nid: number, extra: Record<string, unknown> = {}) =>
  JSON.stringify({ post_type: "memo", content_source: "photo", notification_id: nid, ...extra });

describe("投稿作成（写真の下書き）", () => {
  it("新しい下書きは D1 の往復3回（認証・確認の読み取り・作成）、既にあれば2回", async () => {
    expect((await post("/groups/9201/posts", photo(9201)))!.status).toBe(201);
    expect(d1.roundTrips).toBe(3);
    d1.roundTrips = 0;
    expect((await post("/groups/9201/posts", photo(9201)))!.status).toBe(201);
    expect(d1.roundTrips).toBe(2);
  });

  it("GitHub / 手入力の投稿は、認証もせず D1 を使わずにコンテナへ転送する", async () => {
    expect(await post("/groups/9201/posts", JSON.stringify({ post_type: "commit", repo_full_name: "o/r" }))).toBeNull();
    expect(await post("/groups/9201/posts", JSON.stringify({ post_type: "memo", body: "x" }), { token: "dev_nobody" })).toBeNull();
    expect(d1.roundTrips).toBe(0);
  });

  it("64KB を超える本文は、読まずに（Content-Length がなければ読みかけで）転送する", async () => {
    const big = photo(9201, { body: "x".repeat(70 * 1024) });
    expect(await post("/groups/9201/posts", big)).toBeNull();
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(new TextEncoder().encode(big));
        controller.close();
      },
    });
    const request = new Request(`${BASE}/groups/9201/posts`, {
      method: "POST", headers: { Authorization: "Bearer dev_pr3_poster" }, body: stream, duplex: "half",
    } as RequestInit);
    expect(await handleWorkerRoute(request, env)).toBeNull();
    expect(await request.text()).toBe(big);
  });

  it("確認の後に別のリクエストが下書きを作っていたら、それを返す（UNIQUE に当たっても 500 にしない）", async () => {
    // 確認の読み取り（1回目の batch）の直後に、同じ通知の下書きを差し込む
    const batch = d1.batch.bind(d1);
    let calls = 0;
    vi.spyOn(d1, "batch").mockImplementation(async (statements) => {
      const results = await batch(statements);
      if (++calls === 1) {
        d1.sqlite.exec("INSERT INTO posts (id, notification_id, user_id, group_id, post_type, body, is_draft) VALUES (920200, 9201, 9202, 9201, 'memo', 'first', 1)");
      }
      return results;
    });
    const response = await post("/groups/9201/posts", photo(9201, { body: "second" }));
    expect(response!.status).toBe(201);
    expect(await response!.json()).toMatchObject({ id: 920200, body: "first" });
    expect(d1.sqlite.prepare("SELECT COUNT(*) AS n FROM posts WHERE notification_id = 9201").get()).toEqual({ n: 1 });
  });

  it("D1 の失敗は JSON の 500", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    vi.spyOn(d1, "batch").mockRejectedValue(new Error("D1_ERROR"));
    const response = await post("/groups/9201/posts", photo(9201));
    expect(response!.status).toBe(500);
    expect(await response!.json()).toEqual({ error: "internal server error" });
  });
});

describe("リアクション", () => {
  const react = (postID: number, type: string, token?: string) =>
    post(`/groups/9201/posts/${postID}/reactions`, JSON.stringify({ reaction_type: type }), { token });

  it("D1 の往復2回（認証と、確認・削除・追加・一覧の batch）", async () => {
    expect((await react(920100, "heart"))!.status).toBe(201);
    expect(d1.roundTrips).toBe(2);
  });

  it("付けられない投稿では、何も消さず何も足さない", async () => {
    d1.sqlite.exec("UPDATE posts SET is_draft = 1 WHERE id = 920100; INSERT INTO reactions (post_id, user_id, reaction_type) VALUES (920100, 9202, 'heart')");
    expect((await react(920100, "fire"))!.status).toBe(404);
    expect(d1.sqlite.prepare("SELECT user_id, reaction_type FROM reactions WHERE post_id = 920100").all()).toEqual([{ user_id: 9202, reaction_type: "heart" }]);
    expect(queue.sent).toEqual([]);
  });

  it("自分の投稿なら通知を積まない", async () => {
    expect((await react(920101, "heart"))!.status).toBe(201);
    expect(queue.sent).toEqual([]);
  });

  it("Queue に積めなくても、リアクションは成功にする（Go と同じくベストエフォート）", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    vi.spyOn(queue, "send").mockRejectedValue(new Error("queue down"));
    const response = await react(920100, "heart");
    expect(response!.status).toBe(201);
    expect(d1.sqlite.prepare("SELECT reaction_type FROM reactions WHERE post_id = 920100").all()).toEqual([{ reaction_type: "heart" }]);
  });

  it("Queue がない環境でも、リアクションは成功にする", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    env.NOTIFICATION_QUEUE = undefined;
    expect((await react(920100, "heart"))!.status).toBe(201);
  });

  it("64KB を超える本文は 413", async () => {
    const response = await post("/groups/9201/posts/920100/reactions", JSON.stringify({ reaction_type: "heart", pad: "x".repeat(70 * 1024) }));
    expect(response!.status).toBe(413);
  });

  it("種類の値に文字列以外が混ざれば、後に正しい値があっても 400（Go と同じ）", async () => {
    const response = await post("/groups/9201/posts/920100/reactions", '{"reaction_type":1,"Reaction_Type":"heart"}');
    expect(response!.status).toBe(400);
  });
});

describe("decodeGoObject", () => {
  const fields = { notification_id: "*int64", post_type: "string", body: "*string" } as const;

  it("Go と同じく、null はポインターなら null、そうでなければ読み飛ばす", () => {
    expect(decodeGoObject('{"notification_id":null,"post_type":null,"body":null}', fields)).toEqual({ notification_id: null, body: null });
  });

  it("Go と読み方がずれうる入力は null（転送）", () => {
    expect(decodeGoObject('{"post_type":"a","Post_Type":"b"}', fields)).toBeNull();
    expect(decodeGoObject('{"post_type":"a","post_type":"b"}', fields)).toBeNull();
    expect(decodeGoObject('{"poſt_type":"memo"}', fields)).toBeNull();
    expect(decodeGoObject('{"notification_id":9007199254740993}', fields)).toBeNull();
    expect(decodeGoObject('{"notification_id":-1}', fields)).toEqual({ notification_id: -1 });
  });

  it("入れ子のキーや文字列の中の : は、最上位のキーと取り違えない", () => {
    expect(decodeGoObject('{"body":"\\"post_type\\": x","x":{"notification_id":1.5}}', fields)).toEqual({ body: '"post_type": x' });
  });
});
