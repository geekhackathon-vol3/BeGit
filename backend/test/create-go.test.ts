// 投稿作成が Go と同じ応答を返すかを、Go に実際に送って記録した応答（fixtures/create-go.json）で確かめる。
// Worker が処理するのは写真の下書きだけ。それ以外のケースは、Worker が null を返して（コンテナへ転送して）Go に任せることを確かめる。
// 記録した条件は fixture の recorded にある。Go の挙動が変わったら、同じ cases を Go に送って記録し直す。
import { beforeAll, describe, expect, it } from "vitest";
import { handleWorkerRoute } from "../src/api/router";
import type { ApiEnv } from "../src/api/types";
import type { FakeD1 } from "./fake-d1";
import { KEY, loadFixture, Normalizer, setupFixture, toRequest } from "./go-fixture";

const fixture = loadFixture("create-go.json");

// コンテナへ転送するケース（理由つき）。Go がそのまま処理するので、応答は記録と同じになる
const FORWARDED: Record<string, string> = {
  "body が数値": "Go がエラーにする本文は、エラーの文言も含めて Go に任せる",
  "notification_id が文字列": "同上",
  "notification_id が小数": "同上（JSON.parse は 9217.0 を整数として読むが、Go はエラーにする）",
  "notification_id が指数表記": "同上",
  "pull_request_number が文字列": "同上",
  "壊れた JSON": "同上",
  "ボディなし": "同上",
  "配列": "同上",
  "null": "content_source がないので GitHub の投稿",
  "先頭に BOM": "Go がエラーにする本文",
  "post_type が不正": "エラーの文言（Go の %q）を Go に任せる",
  "content_source の大文字は写真にならない": "手入力の投稿",
  "手入力の BeGit Time! は400": "手入力の投稿",
  "JSON の後ろに文字": "Go の json.Decoder は後ろのゴミを無視するが、JSON.parse は読めない。読み方がずれるので Go に任せる",
  "キーの重複は後が勝つ": "JSON.parse は重複したキーの前の値を捨てるので、Go と同じに読めると言い切れない",
  "手入力の投稿": "手入力の投稿",
  "GitHub の投稿": "GitHub の投稿",
  "GitHub の投稿をもう一度": "GitHub の投稿（Go は 500 を返す。#136 で 409 に直した Go がデプロイされれば 409）",
};

let d1: FakeD1;
let env: ApiEnv;
const normalize = new Normalizer(fixture.fixed_ids, "post");

// ケースは記録と同じ順で流す（新しい投稿の id の置き換えが、出てきた順に依存するため）。
// 転送するケースのうち Go で行を作るものは最後に置いてあるので、Worker で行ができなくても後のケースに影響しない
beforeAll(async () => {
  d1 = await setupFixture(fixture);
  env = { DB: d1.asD1(), DB_ENCRYPTION_KEY: KEY, WORKER_ROUTES: "posts.create" };
});

describe("投稿作成が Go と同じ応答を返す（fixtures/create-go.json）", () => {
  it("転送するケースは fixture にある（記録し直して名前が変わったら気づけるように）", () => {
    for (const name of Object.keys(FORWARDED)) {
      expect(fixture.cases.map((c) => c.name)).toContain(name);
    }
  });

  it.each(fixture.cases.map((c) => [c.name, c] as const))("%s", async (_, c) => {
    const request = toRequest(c);
    const response = await handleWorkerRoute(request, env);
    if (c.name in FORWARDED) {
      expect(response).toBeNull();
      // 転送には元の本文を使う（claim が clone を読んだ後も、元の本文が残っている）
      expect(new Uint8Array(await request.arrayBuffer())).toEqual(new TextEncoder().encode(c.body ?? ""));
      return;
    }

    expect(response).not.toBeNull();
    const actual: { status: number; body: unknown; rows?: unknown[] } = {
      status: response!.status,
      body: normalize.apply(await response!.json()),
    };
    if (c.rows) actual.rows = normalize.apply(d1.sqlite.prepare(c.rows).all()) as unknown[];
    expect(actual).toEqual(c.expect);
  });
});
