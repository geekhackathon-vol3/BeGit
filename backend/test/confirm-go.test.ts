// 投稿確定が Go と同じ応答を返すかを、Go に実際に送って記録した応答（fixtures/confirm-go.json）で確かめる。
// 記録した条件は fixture の recorded にある。Go の挙動が変わったら、同じ cases を Go に送って記録し直す。
// 期待値は Go のコードを読んで書いたものではなく、Go の振る舞いそのもの。expect は手で書き換えない。
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { beforeAll, describe, expect, it } from "vitest";
import { encryptToken } from "../src/api/crypto";
import { handleWorkerRoute } from "../src/api/router";
import type { ApiEnv } from "../src/api/types";
import { FakeD1 } from "./fake-d1";

interface Row {
  body: string | null;
  is_draft: number;
}

interface Case {
  name: string;
  method: string;
  path: string;
  token: string | null;
  body: string | null;
  headers: Record<string, string>;
  target: number | null;
  expect: { status: number; body: unknown; row?: Row };
}

interface Fixture {
  users: { id: number; login: string }[];
  groups: { id: number; owner_user_id: number }[];
  members: [number, number, string][];
  posts: {
    id: number;
    user_id: number;
    group_id: number;
    post_type: string;
    body: string | null;
    is_draft: number;
    notification: boolean;
    created_at: string;
  }[];
  cases: Case[];
}

const fixture: Fixture = JSON.parse(readFileSync(join(import.meta.dirname, "fixtures", "confirm-go.json"), "utf8"));
const KEY = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f";

// Worker が意図して Go と違う応答を返すケース（理由つき）
const INTENDED_DIFFERENCES: Record<string, { status: number; body: unknown; row: Row; reason: string }> = {
  "JSON の後ろに文字": {
    status: 400,
    body: { error: "invalid request body" },
    row: { body: "x", is_draft: 1 },
    reason: "Go の json.Decoder は1つ目の値の後ろを読まない。Worker は JSON.parse で全体を読む",
  },
};

let d1: FakeD1;
let env: ApiEnv;

// 記録したときの検証環境と同じデータを作る。ケースは記録と同じ順で流す（書き込むケースはそれぞれ別の投稿を使うので、順には依存しない）
beforeAll(async () => {
  d1 = new FakeD1();
  env = { DB: d1.asD1(), DB_ENCRYPTION_KEY: KEY, WORKER_ROUTES: "posts.confirm" };
  for (const user of fixture.users) {
    d1.sqlite.prepare("INSERT INTO users (id, github_id, github_login, encrypted_access_token) VALUES (?, ?, ?, ?)")
      .run(user.id, 1000 + user.id, user.login, await encryptToken(KEY, `dev_${user.login}`));
  }
  for (const group of fixture.groups) {
    d1.sqlite.prepare("INSERT INTO groups (id, repo_full_name, owner_user_id) VALUES (?, ?, ?)")
      .run(group.id, `o/r${group.id}`, group.owner_user_id);
  }
  for (const [groupID, userID, role] of fixture.members) {
    d1.sqlite.prepare("INSERT INTO group_members (group_id, user_id, role) VALUES (?, ?, ?)").run(groupID, userID, role);
  }
  d1.sqlite.exec(`
    INSERT INTO sprints (id, group_id, ends_at) VALUES (1, 1, '2099-01-01 00:00:00');
    INSERT INTO notifications (id, sprint_id, sent_by) VALUES (1, 1, 2);
  `);
  for (const post of fixture.posts) {
    d1.sqlite.prepare(
      "INSERT INTO posts (id, user_id, group_id, post_type, body, is_draft, notification_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
    ).run(post.id, post.user_id, post.group_id, post.post_type, post.body, post.is_draft, post.notification ? 1 : null, post.created_at);
  }
});

function send(c: Case) {
  const headers: Record<string, string> = { ...c.headers };
  if (c.token !== null) headers.Authorization = `Bearer ${c.token}`;
  const init: RequestInit = { method: c.method, headers };
  if (c.body !== null) {
    init.body = c.body;
    headers["Content-Length"] = String(new TextEncoder().encode(c.body).length);
  }
  return handleWorkerRoute(new Request(`https://begit.example${c.path}`, init), env);
}

describe("投稿確定が Go と同じ応答を返す（fixtures/confirm-go.json）", () => {
  it("意図した差のケースは fixture にある（記録し直して名前が変わったら気づけるように）", () => {
    for (const name of Object.keys(INTENDED_DIFFERENCES)) {
      expect(fixture.cases.map((c) => c.name)).toContain(name);
    }
  });

  it.each(fixture.cases.map((c) => [c.name, c] as const))("%s", async (_, c) => {
    const response = await send(c);
    expect(response).not.toBeNull();
    const actual = { status: response!.status, body: await response!.json() };

    const intended = INTENDED_DIFFERENCES[c.name];
    expect(actual).toEqual(intended ? { status: intended.status, body: intended.body } : { status: c.expect.status, body: c.expect.body });
    const row = intended ? intended.row : c.expect.row;
    if (row) {
      expect(d1.sqlite.prepare("SELECT body, is_draft FROM posts WHERE id = ?").get(c.target)).toEqual(row);
    }
  });
});
