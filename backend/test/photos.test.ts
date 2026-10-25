// 写真まわりのうち、Go の記録（feed-go / upload-go）では確かめられないところ。
// 記録した検証環境には R2 の認証情報がないので、実際の R2 に保存する経路と、途中で失敗したときの片付けをここで見る。
import { beforeEach, describe, expect, it } from "vitest";
import { encryptToken } from "../src/api/crypto";
import { handleWorkerRoute } from "../src/api/router";
import type { ApiEnv } from "../src/api/types";
import { FakeD1 } from "./fake-d1";

const KEY = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f";
const BASE = "https://begit.example";
const JPEG = new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 1, 2, 3]);
const PNG = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3]);

// PHOTOS バインディングの代わり。put / delete だけを使う
class FakeBucket {
  objects = new Map<string, { data: Uint8Array; contentType?: string }>();
  failPutFor: string | null = null;
  // true なら、書いたうえで失敗を返す（応答が届かなかった場合）
  failAfterWrite = false;

  async put(key: string, data: Blob, options?: { httpMetadata?: { contentType?: string } }) {
    if (key === this.failPutFor && !this.failAfterWrite) throw new Error("R2 is down");
    this.objects.set(key, { data: new Uint8Array(await data.arrayBuffer()), contentType: options?.httpMetadata?.contentType });
    if (key === this.failPutFor) throw new Error("R2 connection lost");
  }

  async delete(key: string) {
    this.objects.delete(key);
  }
}

let d1: FakeD1;
let bucket: FakeBucket;
let env: ApiEnv;

beforeEach(async () => {
  d1 = new FakeD1();
  bucket = new FakeBucket();
  env = {
    DB: d1.asD1(),
    DB_ENCRYPTION_KEY: KEY,
    WORKER_ROUTES: "posts.list,photos.upload",
    PHOTOS: bucket as unknown as R2Bucket,
    R2_ACCESS_KEY_ID: "AKID",
    R2_SECRET_ACCESS_KEY: "secret",
    CF_ACCOUNT_ID: "acct",
    R2_BUCKET: "begit-photos",
  };
  d1.sqlite.prepare("INSERT INTO users (id, github_id, github_login, encrypted_access_token) VALUES (1, 1001, 'poster', ?)")
    .run(await encryptToken(KEY, "dev_poster"));
  d1.sqlite.exec(`
    INSERT INTO groups (id, repo_full_name, owner_user_id) VALUES (1, 'o/r1', 1);
    INSERT INTO group_members (group_id, user_id, role) VALUES (1, 1, 'poster');
    INSERT INTO sprints (id, group_id, ends_at) VALUES (1, 1, '2099-01-01 00:00:00');
    INSERT INTO notifications (id, sprint_id, sent_by, sent_at) VALUES (1, 1, 1, datetime('now', '-10 minutes'));
    INSERT INTO posts (id, user_id, group_id, post_type, is_draft, notification_id) VALUES (10, 1, 1, 'memo', 1, 1);
  `);
});

// FormData をボディにした Request には Content-Length が付かないので、上限は読みながら確かめることになる
function upload(parts: { name: string; data: Uint8Array }[]) {
  const form = new FormData();
  for (const part of parts) form.append(part.name, new File([part.data], `${part.name}.bin`, { type: "text/plain" }));
  const request = new Request(`${BASE}/groups/1/posts/10/photos`, { method: "POST", body: form, headers: { Authorization: "Bearer dev_poster" } });
  return handleWorkerRoute(request, env);
}

function postRow() {
  return d1.sqlite.prepare("SELECT is_draft, status FROM posts WHERE id = 10").get();
}

function photoRows() {
  return d1.sqlite.prepare("SELECT r2_key, photo_type FROM photos WHERE post_id = 10 ORDER BY id").all();
}

describe("写真アップロード（実際の R2）", () => {
  it("バインディングに保存し、署名付き URL を返す。下書きを確定してポイントを付ける", async () => {
    const response = await upload([{ name: "main", data: JPEG }, { name: "front", data: PNG }]);
    expect(response!.status).toBe(201);
    const body = await response!.json() as { photos: { photo_type: string; url: string }[] };
    expect(body.photos.map((photo) => photo.photo_type)).toEqual(["main", "front"]);
    expect(body.photos[0].url).toMatch(
      /^https:\/\/acct\.r2\.cloudflarestorage\.com\/begit-photos\/posts\/10\/main\.jpg\?X-Amz-Algorithm=AWS4-HMAC-SHA256&.*&X-Amz-Signature=[0-9a-f]{64}$/,
    );

    expect([...bucket.objects].map(([key, object]) => [key, object.contentType, object.data])).toEqual([
      ["posts/10/main.jpg", "image/jpeg", JPEG],
      ["posts/10/front.png", "image/png", PNG],
    ]);
    expect(photoRows()).toEqual([
      { r2_key: "posts/10/main.jpg", photo_type: "main" },
      { r2_key: "posts/10/front.png", photo_type: "front" },
    ]);
    expect(postRow()).toEqual({ is_draft: 0, status: "on_time" });
    expect(d1.sqlite.prepare("SELECT kind, points FROM point_awards WHERE user_id = 1").all()).toEqual([{ kind: "report", points: 10 }]);
  });

  it("2枚目の保存に失敗したら 502。1枚目を消し、行も作らず、下書きのまま", async () => {
    bucket.failPutFor = "posts/10/front.png";
    const response = await upload([{ name: "main", data: JPEG }, { name: "front", data: PNG }]);
    expect({ status: response!.status, body: await response!.json() }).toEqual({ status: 502, body: { error: "photo storage error" } });
    expect(bucket.objects.size).toBe(0);
    expect(photoRows()).toEqual([]);
    expect(postRow()).toEqual({ is_draft: 1, status: null });
  });

  it("行の作成に失敗したら 500。保存した写真を消し、確定もしない", async () => {
    d1.sqlite.exec("CREATE TRIGGER fail_photo BEFORE INSERT ON photos BEGIN SELECT RAISE(ABORT, 'boom'); END;");
    const response = await upload([{ name: "main", data: JPEG }, { name: "front", data: PNG }]);
    expect({ status: response!.status, body: await response!.json() }).toEqual({ status: 500, body: { error: "internal server error" } });
    expect(bucket.objects.size).toBe(0);
    expect(postRow()).toEqual({ is_draft: 1, status: null });
  });

  it("書き込めたのに失敗と返ってきたら（応答が届かなかった）、行が指す写真は消さない", async () => {
    const batch = d1.batch.bind(d1);
    d1.batch = async (statements) => {
      const results = await batch(statements);
      if (statements.some((statement) => statement.sql.startsWith("INSERT INTO photos"))) throw new Error("Network connection lost");
      return results;
    };
    const response = await upload([{ name: "main", data: JPEG }]);
    expect(response!.status).toBe(500);
    expect(photoRows()).toEqual([{ r2_key: "posts/10/main.jpg", photo_type: "main" }]);
    expect([...bucket.objects.keys()]).toEqual(["posts/10/main.jpg"]);
  });

  it("書き込めたかを確かめる SELECT も失敗したら、行が指しているかもしれない写真は消さない", async () => {
    const batch = d1.batch.bind(d1);
    d1.batch = async (statements) => {
      if (statements.some((statement) => statement.sql.startsWith("INSERT INTO photos"))) throw new Error("D1 is down");
      return batch(statements);
    };
    const prepare = d1.prepare.bind(d1);
    d1.prepare = (sql) => {
      const statement = prepare(sql);
      if (sql.startsWith("SELECT 1 AS saved")) {
        // bind() は新しい statement を返すので、bind した後の first() を失敗させる
        const bind = statement.bind.bind(statement);
        statement.bind = (...values) => Object.assign(bind(...values), { first: async () => { throw new Error("D1 is down"); } });
      }
      return statement;
    };
    const response = await upload([{ name: "main", data: JPEG }]);
    expect(response!.status).toBe(500);
    expect([...bucket.objects.keys()]).toEqual(["posts/10/main.jpg"]);
  });

  it("保存できたのに失敗が返ってきた写真も消す", async () => {
    bucket.failPutFor = "posts/10/front.png";
    bucket.failAfterWrite = true;
    const response = await upload([{ name: "main", data: JPEG }, { name: "front", data: PNG }]);
    expect(response!.status).toBe(502);
    expect(bucket.objects.size).toBe(0);
    expect(photoRows()).toEqual([]);
  });

  it("認証情報があるのにバインディングがなければ（設定ミス）、保存できないので 502", async () => {
    env.PHOTOS = undefined;
    const response = await upload([{ name: "main", data: JPEG }]);
    expect(response!.status).toBe(502);
    expect(photoRows()).toEqual([]);
    expect(postRow()).toEqual({ is_draft: 1, status: null });
  });

  it("Content-Length がない（FormData のボディ）ときも、21MB を超えたら読むのをやめて 413", async () => {
    const big = new Uint8Array(11 * 1024 * 1024);
    big.set(JPEG);
    const response = await upload([{ name: "main", data: big }, { name: "front", data: big }]);
    expect(response!.status).toBe(413);
    expect(bucket.objects.size).toBe(0);
  });

  it("往復は、認証・投稿と写真と通知の取得・作成と確定の3回（Go は7回）", async () => {
    const before = d1.roundTrips;
    await upload([{ name: "main", data: JPEG }]);
    expect(d1.roundTrips - before).toBe(3);
  });
});

describe("フィード取得（実際の R2）", () => {
  it("写真に署名付き URL を付ける", async () => {
    d1.sqlite.exec(`
      UPDATE posts SET is_draft = 0 WHERE id = 10;
      INSERT INTO photos (post_id, r2_key, photo_type) VALUES (10, 'posts/10/main.jpg', 'main');
    `);
    const response = await handleWorkerRoute(
      new Request(`${BASE}/groups/1/posts`, { headers: { Authorization: "Bearer dev_poster" } }),
      env,
    );
    const body = await response!.json() as { posts: { photos: { url: string }[] }[] };
    expect(body.posts[0].photos[0].url).toMatch(/^https:\/\/acct\.r2\.cloudflarestorage\.com\/begit-photos\/posts\/10\/main\.jpg\?.*X-Amz-Expires=3600&/);
  });
});
