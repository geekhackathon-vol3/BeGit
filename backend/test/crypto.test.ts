import { describe, expect, it } from "vitest";
import { encryptToken } from "../src/api/crypto";

// 期待値は Go の pkg/crypto（NewEncryptor → Encrypt）で作った暗号文。
// Worker と Go の暗号文が1バイトでも違うと、Worker が処理する API で全員が 401 になる。
const HEX_KEY = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f";
const RAW_KEY = "0123456789abcdefghijklmnopqrstuv";

describe("encryptToken", () => {
  it.each([
    [HEX_KEY, "dev_user001", "14537bdf55e8cd803ec76582:507da91814cbcc3ab7009e6a00e434230023e528c79d0df42a5272"],
    [HEX_KEY, "gho_テスト🐙", "4d9c1915f95051f116a945ba:1e8d9f5ac17bb773555a231f30ad70d2ce34a75ad7b39402aa8cf26f5998148f5f"],
    [RAW_KEY, "dev_user001", "60d8034e4dd7008ac6cd3ad1:bf875b8eb216a378c4b460957542db463dc111174d530ace9911c5"],
    [RAW_KEY, "gho_テスト🐙", "07c64e64fcd2470a3a39b172:f0ad3ecfa3cc292643f51e71d71d65dcfc4629c70f0babb3f2c8fc0896200f83a0"],
  ])("Go と同じ暗号文になる（鍵 %s、トークン %s）", async (key, token, expected) => {
    expect(await encryptToken(key, token)).toBe(expected);
  });

  it("大文字の hex の鍵も Go と同じく読める", async () => {
    expect(await encryptToken(HEX_KEY.toUpperCase(), "dev_user001")).toBe(
      "14537bdf55e8cd803ec76582:507da91814cbcc3ab7009e6a00e434230023e528c79d0df42a5272",
    );
  });

  it.each([
    ["32文字の hex（16バイト）", "000102030405060708090a0b0c0d0e0f"],
    ["短すぎる", "short"],
    ["空", ""],
  ])("鍵が不正なら失敗する（%s）", async (_, key) => {
    await expect(encryptToken(key, "dev_user001")).rejects.toThrow();
  });
});
