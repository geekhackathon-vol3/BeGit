// フィード取得が Go と同じ応答を返すかを、Go に実際に送って記録した応答（fixtures/feed-go.json）で確かめる。
// 記録した条件は fixture の recorded にある。Go の挙動が変わったら、同じ seed と cases で記録し直す。expect は手で書き換えない。
import { beforeAll, describe, expect, it } from "vitest";
import { handleWorkerRoute } from "../src/api/router";
import type { ApiEnv } from "../src/api/types";
import { FakeD1 } from "./fake-d1";
import { loadFixture, seedFakeD1, TEST_KEY, toRequest } from "./go-fixture";

const fixture = loadFixture("feed-go.json");

// Worker が意図して Go と違う応答を返すケース（理由つき）
const INTENDED_DIFFERENCES: Record<string, { status: number; check: (body: { posts: { id: number }[] }) => void; reason: string }> = {
  "投稿が100件を超えるグループ": {
    status: 200,
    // 一番新しい 915012 の後に、915200 から 915100 までが新しい順に並ぶ
    check: (body) => {
      expect(body.posts.map((post) => post.id)).toEqual([915012, ...Array.from({ length: 101 }, (_, i) => 915200 - i)]);
    },
    reason: "Go は写真を IN (?, …) で引くので、D1 の1つの SQL に渡せる値（100個）を超えて 500 になる。Worker はサブクエリで引く",
  },
};

let d1: FakeD1;
let env: ApiEnv;

beforeAll(async () => {
  d1 = new FakeD1();
  env = { DB: d1.asD1(), DB_ENCRYPTION_KEY: TEST_KEY, WORKER_ROUTES: "posts.list" };
  await seedFakeD1(d1, fixture);
});

describe("フィード取得が Go と同じ応答を返す（fixtures/feed-go.json）", () => {
  it("意図した差のケースは fixture にある（記録し直して名前が変わったら気づけるように）", () => {
    for (const name of Object.keys(INTENDED_DIFFERENCES)) {
      expect(fixture.cases.map((c) => c.name)).toContain(name);
    }
  });

  it.each(fixture.cases.map((c) => [c.name, c] as const))("%s", async (_, c) => {
    const response = await handleWorkerRoute(toRequest(c), env);
    expect(response).not.toBeNull();
    const body = await response!.json();

    const intended = INTENDED_DIFFERENCES[c.name];
    if (intended) {
      expect(response!.status).toBe(intended.status);
      intended.check(body as { posts: { id: number }[] });
    } else {
      expect({ status: response!.status, body }).toEqual({ status: c.expect.status, body: c.expect.body });
    }
  });

  it("投稿・投稿者・写真を1回の往復で取る（認証と合わせて2回。Go は5回）", async () => {
    const before = d1.roundTrips;
    await handleWorkerRoute(toRequest(fixture.cases[0]), env);
    expect(d1.roundTrips - before).toBe(2);
  });
});
