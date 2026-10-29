// encryptToken は Go の pkg/crypto（aesEncryptor.Encrypt）と同じ暗号文を作る。
// users.encrypted_access_token との完全一致で認証するため、1バイトでも違うと全員が 401 になる。
//   鍵    : DB_ENCRYPTION_KEY を hex デコードした32バイト。hex でなければ、32バイトの文字列をそのまま使う
//   nonce : SHA-256(鍵 ‖ トークン) の先頭12バイト（同じトークンから毎回同じ暗号文を作るため）
//   出力  : hex(nonce) + ":" + hex(暗号文 ‖ 認証タグ)
const encoder = new TextEncoder();

interface EncryptionKey {
  raw: Uint8Array;
  key: CryptoKey;
}

const keyCache = new Map<string, Promise<EncryptionKey>>();

export async function encryptToken(keySource: string, token: string): Promise<string> {
  const { raw, key } = await loadKey(keySource);
  const plaintext = encoder.encode(token);

  const seed = new Uint8Array(raw.length + plaintext.length);
  seed.set(raw);
  seed.set(plaintext, raw.length);
  const nonce = new Uint8Array(await crypto.subtle.digest("SHA-256", seed)).slice(0, 12);

  const ciphertext = new Uint8Array(
    await crypto.subtle.encrypt({ name: "AES-GCM", iv: nonce, tagLength: 128 }, key, plaintext),
  );
  return `${toHex(nonce)}:${toHex(ciphertext)}`;
}

function loadKey(keySource: string): Promise<EncryptionKey> {
  let cached = keyCache.get(keySource);
  if (!cached) {
    cached = importKey(keySource);
    keyCache.set(keySource, cached);
    // 失敗した鍵はキャッシュに残さない（secret を直したあとに再試行できるように）
    cached.catch(() => keyCache.delete(keySource));
  }
  return cached;
}

async function importKey(keySource: string): Promise<EncryptionKey> {
  const raw = decodeKey(keySource);
  const key = await crypto.subtle.importKey("raw", raw, "AES-GCM", false, ["encrypt"]);
  return { raw, key };
}

// decodeKey は Go の NewEncryptor と同じ規則で鍵を解釈する。
// hex として読めた場合は、長さが32バイトでなくても raw へは戻らない（Go と同じ）。
function decodeKey(keySource: string): Uint8Array {
  const hex = fromHex(keySource);
  if (hex) {
    if (hex.length !== 32) throw new Error("DB_ENCRYPTION_KEY must decode to 32 bytes");
    return hex;
  }
  const raw = encoder.encode(keySource);
  if (raw.length !== 32) throw new Error("DB_ENCRYPTION_KEY must be 64 hex chars or 32 bytes");
  return raw;
}

function fromHex(value: string): Uint8Array | null {
  if (value.length % 2 !== 0 || !/^[0-9a-fA-F]*$/.test(value)) return null;
  const bytes = new Uint8Array(value.length / 2);
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = parseInt(value.slice(i * 2, i * 2 + 2), 16);
  }
  return bytes;
}

function toHex(bytes: Uint8Array): string {
  let out = "";
  for (const byte of bytes) out += byte.toString(16).padStart(2, "0");
  return out;
}
