// リアクションが Go と同じ応答を返すかを、Go に実際に送って記録した応答（fixtures/reaction-go.json）で確かめる。
// 記録した条件は fixture の recorded にある。Go の挙動が変わったら、同じ cases を Go に送って記録し直す。
import { beforeAll, describe, expect, it } from "vitest";
import { handleWorkerRoute } from "../src/api/router";
import type { ApiEnv } from "../src/api/types";
import type { FakeD1 } from "./fake-d1";
import { FakeQueue, KEY, loadFixture, Normalizer, setupFixture, toRequest } from "./go-fixture";

const fixture = loadFixture("reaction-go.json");

const INVALID_TYPE = {
  status: 400,
  body: { error: "invalid reaction type" },
  reason: "Go は service.ErrInvalidReactionType をハンドラーで扱っておらず 500 を返す。入力の誤りなので 400 にする（#139）",
};

// Worker が意図して Go と違う応答を返すケース（理由つき）。rows を書かないものは、行は Go と同じ
const INTENDED_DIFFERENCES: Record<string, { status: number; body: unknown; rows?: unknown[]; reason: string }> = {
  "不正な種類": INVALID_TYPE,
  "種類の大文字": INVALID_TYPE,
  "種類の前後に空白": INVALID_TYPE,
  "不正な種類で下書きの投稿": INVALID_TYPE,
  "JSON の後ろに文字": {
    status: 400,
    body: { error: "invalid request body" },
    rows: [],
    reason: "Go の json.Decoder は1つ目の値の後ろを読まない。Worker は JSON.parse で全体を読む（posts.confirm と同じ）",
  },
};

let d1: FakeD1;
let queue: FakeQueue;
let env: ApiEnv;
const normalize = new Normalizer(fixture.fixed_ids, "reaction");

// ケースは記録と同じ順で流す（リアクションの id の置き換えが、出てきた順に依存するため）
beforeAll(async () => {
  d1 = await setupFixture(fixture);
  queue = new FakeQueue();
  env = { DB: d1.asD1(), DB_ENCRYPTION_KEY: KEY, WORKER_ROUTES: "reactions.create", NOTIFICATION_QUEUE: queue.asQueue() };
});

describe("リアクションが Go と同じ応答を返す（fixtures/reaction-go.json）", () => {
  it("意図した差のケースは fixture にある（記録し直して名前が変わったら気づけるように）", () => {
    for (const name of Object.keys(INTENDED_DIFFERENCES)) {
      expect(fixture.cases.map((c) => c.name)).toContain(name);
    }
  });

  it.each(fixture.cases.map((c) => [c.name, c] as const))("%s", async (_, c) => {
    const response = await handleWorkerRoute(toRequest(c), env);
    expect(response).not.toBeNull();
    const actual: { status: number; body: unknown; rows?: unknown[] } = {
      status: response!.status,
      body: normalize.apply(await response!.json()),
    };
    if (c.rows) actual.rows = normalize.apply(d1.sqlite.prepare(c.rows).all()) as unknown[];

    const intended = INTENDED_DIFFERENCES[c.name];
    const want = intended
      ? { status: intended.status, body: intended.body, ...(c.rows ? { rows: intended.rows ?? c.expect.rows } : {}) }
      : c.expect;
    expect(actual).toEqual(want);
  });

  it("自分以外の投稿に付けたときだけ、作者への通知を Queue に積む", () => {
    // 201 のケースのうち、自分の投稿（920011）を除いたもの
    expect(queue.sent).toEqual([
      { type: "reaction", group_id: 9201, post_id: 920010, actor_user_id: 9202, author_user_id: 9203 },
      { type: "reaction", group_id: 9201, post_id: 920010, actor_user_id: 9202, author_user_id: 9203 },
      { type: "reaction", group_id: 9201, post_id: 920010, actor_user_id: 9202, author_user_id: 9203 },
      { type: "reaction", group_id: 9201, post_id: 920010, actor_user_id: 9201, author_user_id: 9203 },
      { type: "reaction", group_id: 9201, post_id: 920015, actor_user_id: 9202, author_user_id: 9203 },
      { type: "reaction", group_id: 9201, post_id: 920017, actor_user_id: 9202, author_user_id: 9203 },
      { type: "reaction", group_id: 9201, post_id: 920010, actor_user_id: 9202, author_user_id: 9203 },
      { type: "reaction", group_id: 9201, post_id: 920010, actor_user_id: 9206, author_user_id: 9203 },
    ]);
  });
});
