// フィード取得が Go と同じ応答を返すかを、Go に実際に送って記録した応答（fixtures/feed-go.json）で確かめる。
// 記録した条件は fixture の recorded にある。Go の挙動が変わったら、同じ seed と cases で記録し直す。expect は手で書き換えない。
import { beforeAll, describe, expect, it } from "vitest";
import { handleWorkerRoute } from "../src/api/router";
import type { ApiEnv } from "../src/api/types";
import { FakeD1 } from "./fake-d1";
import { loadFixture, seedFakeD1, TEST_KEY, toRequest } from "./go-fixture";

const fixture = loadFixture("feed-go.json");

// Worker が意図して Go と違う応答を返すケース（理由つき）。
// 前回まであった「投稿が100件を超えるグループ」は、Go も最大 50 件ずつ返すようになり（#164）、差がなくなった
const INTENDED_DIFFERENCES: Record<string, { status: number; check: (body: unknown) => void; reason: string }> = {};

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
      intended.check(body);
    } else {
      expect({ status: response!.status, body }).toEqual({ status: c.expect.status, body: c.expect.body });
    }
  });

  it("投稿・投稿者・写真・リアクションを1回の往復で取る（認証と合わせて2回。Go は6回）", async () => {
    const before = d1.roundTrips;
    await handleWorkerRoute(toRequest(fixture.cases[0]), env);
    expect(d1.roundTrips - before).toBe(2);
  });
});

// fixture にない、カーソルの id の表記。期待する値は、Go の postCursor（ID int64）に json.Unmarshal して確かめたもの
// （1.0・1e3・int64 の範囲外はエラーで 400 "invalid cursor"、int64 に収まれば大きくても読める）
describe("カーソルの id は Go の int64 と同じ規則で読む", () => {
  const feed = async (idSource: string) => {
    const json = `{"created_at":"2026-10-20T10:00:00Z","id":${idSource}}`;
    const cursor = btoa(json).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
    const request = new Request(`https://begit.example/groups/9101/posts?limit=50&cursor=${cursor}`, {
      headers: { Authorization: "Bearer dev_user001" },
    });
    const response = await handleWorkerRoute(request, env);
    return { status: response!.status, body: await response!.json() };
  };

  it.each(["915003.0", "1e3", "1E3", "1e100", "9223372036854775808", "-9223372036854775809"])("%s は 400", async (id) => {
    expect(await feed(id)).toEqual({ status: 400, body: { error: "invalid cursor" } });
  });

  it.each(["9223372036854775807", "9007199254740993"])("%s は int64 に収まるので読める（どの投稿の id より大きい）", async (id) => {
    const larger = await feed("999999999");
    expect(larger.status).toBe(200);
    expect(await feed(id)).toEqual(larger);
  });

  it("-9223372036854775808 は int64 に収まるので読める（どの投稿の id より小さい）", async () => {
    const smaller = await feed("0");
    expect(smaller.status).toBe(200);
    expect(await feed("-9223372036854775808")).toEqual(smaller);
  });
});
