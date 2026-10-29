import { formatRFC3339 } from "./posts/post";
import type { ApiEnv } from "./types";

// 写真の保存先。Go の pkg/r2 と同じく、R2 の認証情報（R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY）が
// あれば実際の R2、なければスタブ（保存しない・ダミーの URL）にする。
// 保存と削除はバインディング（PHOTOS）で行い、読む URL は Go と同じ SigV4 の署名付き URL にする
// （iOS は認証ヘッダーなしの AsyncImage で読むため）。
// 書く先は PHOTOS の bucket_name、URL は R2_BUCKET と CF_ACCOUNT_ID で決まるので、wrangler.toml で必ず揃える。
// PHOTOS がない環境（今の minimal・検証環境）で認証情報だけ登録すると、アップロードは 502 になる
export interface PhotoStorage {
  presignGetURL(key: string): Promise<string>;
  put(key: string, contentType: string, data: Blob): Promise<void>;
  delete(key: string): Promise<void>;
}

// Go の feedPhotoURLTTL・アップロード直後の URL と同じ1時間
const PRESIGN_EXPIRES_SECONDS = 3600;
// R2 は SigV4 のリージョンに auto を受け付ける（Go の region と同じ）
const REGION = "auto";

// スタブで動くことを警告したか（isolate ごとに1回だけ警告する）
let warnedStub = false;

export function photoStorage(env: ApiEnv, now: () => Date = () => new Date()): PhotoStorage {
  const accessKeyID = env.R2_ACCESS_KEY_ID ?? "";
  const secretAccessKey = env.R2_SECRET_ACCESS_KEY ?? "";
  if (accessKeyID === "" || secretAccessKey === "") {
    // Go は起動時に "using stub R2 client" と出す。Worker は isolate ごとに1回出す
    if (!warnedStub) {
      warnedStub = true;
      console.warn("[r2] R2 credentials are not set; using stub storage (photos are not saved)");
    }
    return stubStorage;
  }

  const host = `${env.CF_ACCOUNT_ID ?? ""}.r2.cloudflarestorage.com`;
  const bucket = env.R2_BUCKET || "begit-photos";
  const binding = () => {
    if (!env.PHOTOS) throw new Error("R2 credentials are set but the PHOTOS binding is missing");
    return env.PHOTOS;
  };
  // 署名の鍵は日付ごとに同じなので、フィードの写真が多くても1回だけ作る
  const signingKeys = new Map<string, Promise<CryptoKey>>();
  const signingKey = (dateStamp: string) => {
    let key = signingKeys.get(dateStamp);
    if (!key) {
      key = deriveSigningKey(secretAccessKey, dateStamp);
      signingKeys.set(dateStamp, key);
    }
    return key;
  };
  return {
    presignGetURL: (key) =>
      presignGetURL({ accessKeyID, signingKey, host, uri: `/${uriEncode(bucket, false)}/${uriEncode(key, false)}` }, now()),
    put: async (key, contentType, data) => {
      await binding().put(key, data, { httpMetadata: { contentType } });
    },
    delete: async (key) => {
      await binding().delete(key);
    },
  };
}

// Go の r2.NewStubClient と同じ
const stubStorage: PhotoStorage = {
  presignGetURL: async (key) => `https://stub.r2.local/${key}?stub=1`,
  put: async () => {},
  delete: async () => {},
};

interface PresignTarget {
  accessKeyID: string;
  signingKey: (dateStamp: string) => Promise<CryptoKey>;
  host: string;
  uri: string;
}

// presignGetURL は Go の buildPresignedGetURL と同じ文字列を作る（ペイロードは署名しない）。
async function presignGetURL(target: PresignTarget, at: Date): Promise<string> {
  const amzDate = formatRFC3339(at).replace(/[-:]/g, "");
  const dateStamp = amzDate.slice(0, 8);
  const scope = `${dateStamp}/${REGION}/s3/aws4_request`;
  const query = [
    "X-Amz-Algorithm=AWS4-HMAC-SHA256",
    `X-Amz-Credential=${uriEncode(`${target.accessKeyID}/${scope}`, true)}`,
    `X-Amz-Date=${amzDate}`,
    `X-Amz-Expires=${PRESIGN_EXPIRES_SECONDS}`,
    "X-Amz-SignedHeaders=host",
  ].join("&");
  const canonicalRequest = ["GET", target.uri, query, `host:${target.host}\n`, "host", "UNSIGNED-PAYLOAD"].join("\n");
  const stringToSign = ["AWS4-HMAC-SHA256", amzDate, scope, toHex(await sha256(canonicalRequest))].join("\n");

  const signature = toHex(await crypto.subtle.sign("HMAC", await target.signingKey(dateStamp), new TextEncoder().encode(stringToSign)));
  return `https://${target.host}${target.uri}?${query}&X-Amz-Signature=${signature}`;
}

// Go の deriveSigningKey と同じ鍵を、署名に使える形（CryptoKey）で返す
async function deriveSigningKey(secretAccessKey: string, dateStamp: string): Promise<CryptoKey> {
  let key = new TextEncoder().encode(`AWS4${secretAccessKey}`).buffer as ArrayBuffer;
  for (const part of [dateStamp, REGION, "s3", "aws4_request"]) key = await hmac(key, part);
  return importHmacKey(key);
}

// uriEncode は Go の uriEncode と同じく、UTF-8 のバイトごとに RFC 3986 の unreserved 以外を %XX にする。
// encodeSlash が false なら "/" はそのまま（パス用）。
export function uriEncode(value: string, encodeSlash: boolean): string {
  let out = "";
  for (const byte of new TextEncoder().encode(value)) {
    const ch = String.fromCharCode(byte);
    if (/[A-Za-z0-9\-_.~]/.test(ch) || (ch === "/" && !encodeSlash)) out += ch;
    else out += `%${byte.toString(16).toUpperCase().padStart(2, "0")}`;
  }
  return out;
}

async function hmac(key: ArrayBuffer, data: string): Promise<ArrayBuffer> {
  return crypto.subtle.sign("HMAC", await importHmacKey(key), new TextEncoder().encode(data));
}

function importHmacKey(key: ArrayBuffer): Promise<CryptoKey> {
  return crypto.subtle.importKey("raw", key, { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
}

async function sha256(data: string): Promise<ArrayBuffer> {
  return crypto.subtle.digest("SHA-256", new TextEncoder().encode(data));
}

function toHex(buffer: ArrayBuffer): string {
  return [...new Uint8Array(buffer)].map((b) => b.toString(16).padStart(2, "0")).join("");
}
