// 署名付き URL が Go の pkg/r2 と同じ文字列になるかを、Go で作った URL（fixtures/r2-presign-go.json）で確かめる。
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { photoStorage, uriEncode } from "../src/api/r2";

interface Vector {
  account_id: string;
  access_key_id: string;
  secret_access_key: string;
  bucket: string;
  key: string;
  at: string;
  url: string;
}

const { vectors }: { vectors: Vector[] } = JSON.parse(
  readFileSync(join(import.meta.dirname, "fixtures", "r2-presign-go.json"), "utf8"),
);

describe("署名付き URL", () => {
  it.each(vectors.map((v) => [v.key, v] as const))("Go と同じ URL になる（key=%j）", async (_, v) => {
    const storage = photoStorage(
      { DB: {} as D1Database, DB_ENCRYPTION_KEY: "", R2_ACCESS_KEY_ID: v.access_key_id, R2_SECRET_ACCESS_KEY: v.secret_access_key, CF_ACCOUNT_ID: v.account_id, R2_BUCKET: v.bucket },
      () => new Date(v.at),
    );
    expect(await storage.presignGetURL(v.key)).toBe(v.url);
  });

  it("認証情報がなければ Go のスタブと同じ URL を返し、保存しない", async () => {
    const storage = photoStorage({ DB: {} as D1Database, DB_ENCRYPTION_KEY: "" });
    expect(await storage.presignGetURL("posts/1/main.jpg")).toBe("https://stub.r2.local/posts/1/main.jpg?stub=1");
    await expect(storage.put("k", "image/jpeg", new Blob([new Uint8Array([1])]))).resolves.toBeUndefined();
  });

  it("uriEncode は UTF-8 のバイトごとに大文字の %XX にする", () => {
    expect(uriEncode("a/b c~é", false)).toBe("a/b%20c~%C3%A9");
    expect(uriEncode("a/b", true)).toBe("a%2Fb");
  });
});
