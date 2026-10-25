// 写真アップロードが Go と同じ応答を返すかを、Go に実際に送って記録した応答（fixtures/upload-go.json）で確かめる。
// 記録した条件は fixture の recorded にある。Go の挙動が変わったら、同じ seed と cases で記録し直す。expect は手で書き換えない。
// R2 の認証情報がない環境（記録した検証環境と同じ）なので、写真はスタブに保存し、URL もスタブになる。
import { beforeAll, describe, expect, it } from "vitest";
import { handleWorkerRoute } from "../src/api/router";
import type { ApiEnv } from "../src/api/types";
import { FakeD1 } from "./fake-d1";
import { loadFixture, normalizePhotos, seedFakeD1, targetRow, TEST_KEY, toRequest, type TargetRow } from "./go-fixture";

const fixture = loadFixture("upload-go.json");
const seedPhotoIDs = new Set((fixture.seed.photos ?? []).map((photo) => photo.id as number));

// 送ったまま何も変わらないときの行（下書きのまま、写真なし）
const UNCHANGED: TargetRow = { post: { is_draft: 1, status: null, created_at_is_now: false }, photos: [] };

// Worker が意図して Go と違う応答を返すケース（理由つき）。検証環境の Worker（workerd）でも同じ応答になる
const INTENDED_DIFFERENCES: Record<string, { status: number; body: unknown; row: TargetRow; reason: string }> = {
  "Content-Transfer-Encoding: quoted-printable": {
    status: 422,
    body: { error: "invalid photo" },
    row: UNCHANGED,
    reason: "Go の multipart は quoted-printable を戻すが、FormData は戻さない。RFC 7578 で使わないことになっていて、iOS も送らない",
  },
};

// 検証環境の Worker（workerd）だけで Go と違うケース。Node の FormData は Node のバージョンでも答えが変わる
// （26 は Go と同じ 201、22 は 413）ので、このテストでは比べない。workerd での値は検証環境で確かめた（#157 でテストにする）
const WORKERD_ONLY_DIFFERENCES: Record<string, string> = {
  "filename*（RFC 2231）": "workerd は filename* だけのパートをファイルとして扱わず 400。RFC 7578 で使わないことになっていて、iOS は filename で送る",
};

// テストを動かす Node だけで Go と違うケース（理由つき）。検証環境の Worker（workerd）では Go の記録と一致する
const NODE_ONLY_DIFFERENCES: Record<string, { want: (target: number) => { status: number; body: unknown; row: TargetRow }; reason: string }> = {
  "heic（image/HEIC）": {
    want: (target) => ({
      status: 201,
      body: { photos: [{ id: "<new>", photo_type: "main", url: `https://stub.r2.local/posts/${target}/main.heic?stub=1` }] },
      row: {
        post: { is_draft: 0, status: "on_time", created_at_is_now: true },
        photos: [{ id: "<new>", photo_type: "main", r2_key: `posts/${target}/main.heic` }],
      },
    }),
    reason: "Go と workerd はパートの Content-Type をそのまま比べるので大文字だと 422。Node の FormData は型を小文字にするので image/heic として受け付ける",
  },
  "改行が LF だけ": {
    want: () => ({ status: 413, body: { error: "request too large or invalid multipart form" }, row: UNCHANGED }),
    reason: "Go と workerd は LF だけの multipart も読む。Node の FormData は読めない",
  },
};

let d1: FakeD1;
let env: ApiEnv;

beforeAll(async () => {
  d1 = new FakeD1();
  env = { DB: d1.asD1(), DB_ENCRYPTION_KEY: TEST_KEY, WORKER_ROUTES: "photos.upload" };
  await seedFakeD1(d1, fixture);
});

describe("写真アップロードが Go と同じ応答を返す（fixtures/upload-go.json）", () => {
  it("意図した差のケースは fixture にある（記録し直して名前が変わったら気づけるように）", () => {
    for (const name of [...Object.keys(INTENDED_DIFFERENCES), ...Object.keys(WORKERD_ONLY_DIFFERENCES), ...Object.keys(NODE_ONLY_DIFFERENCES)]) {
      expect(fixture.cases.map((c) => c.name)).toContain(name);
    }
  });

  // 記録と同じく、上から順に送る（書き込むケースはそれぞれ別の投稿を使う）
  for (const name of Object.keys(WORKERD_ONLY_DIFFERENCES)) it.skip(`${name}（Node では比べない）`, () => {});

  it.each(fixture.cases.filter((c) => !(c.name in WORKERD_ONLY_DIFFERENCES)).map((c) => [c.name, c] as const))("%s", async (_, c) => {
    const started = new Date();
    const response = await handleWorkerRoute(toRequest(c), env);
    expect(response).not.toBeNull();
    const actual = { status: response!.status, body: normalizePhotos(await response!.json(), seedPhotoIDs) };

    const want = INTENDED_DIFFERENCES[c.name] ?? NODE_ONLY_DIFFERENCES[c.name]?.want(c.target!) ?? c.expect;
    expect(actual).toEqual({ status: want.status, body: want.body });
    if (c.target !== undefined) {
      expect(targetRow(d1, c.target, seedPhotoIDs, started)).toEqual(want.row);
    }
  });
});
