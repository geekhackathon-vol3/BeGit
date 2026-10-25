// Go に実際に送って記録した応答（fixtures/*-go.json）と、Worker の応答を比べるテストの共通部分。
// fixture の seed を FakeD1 に入れ、cases を Request にし、記録したときと同じ規則で値を置き換える。
// 記録は検証環境で行った（seed を入れ、cases を上から順に Go に送った）。
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { encryptToken } from "../src/api/crypto";
import type { FakeD1 } from "./fake-d1";

type Row = Record<string, unknown>;

export interface GoCase {
  name: string;
  method: string;
  path: string;
  token: string | null;
  headers: Record<string, string | null>;
  body?: string;
  multipart?: { boundary: string; line_ending?: string; omit_closing?: boolean; parts: Part[] };
  target?: number;
  expect: { status: number; body: unknown; row?: TargetRow };
}

interface Part {
  name: string;
  filename: string | null;
  content_type: string | null;
  // Content-Disposition をそのまま使う（filename* などを試すため）
  disposition?: string;
  extra_headers?: string[];
  hex?: string;
  prefix_hex?: string;
  size?: number;
}

export interface GoFixture {
  callers: { id: number; login: string; avatar_url: string }[];
  seed: Record<string, Row[]>;
  cases: GoCase[];
}

export interface TargetRow {
  post: { is_draft: number; status: string | null; created_at_is_now: boolean } | null;
  photos: { id: number | "<new>"; photo_type: string; r2_key: string }[];
}

export const TEST_KEY = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f";

// 外部キーの向きに合わせた順
const TABLES = ["users", "groups", "group_members", "sprints", "notifications", "posts", "photos"];

export function loadFixture(name: string): GoFixture {
  return JSON.parse(readFileSync(join(import.meta.dirname, "fixtures", name), "utf8"));
}

// seedFakeD1 は、記録したときの検証環境と同じデータを作る。呼び出し元のユーザーはトークン dev_<login> で引けるようにする
export async function seedFakeD1(d1: FakeD1, fixture: GoFixture, now = new Date()): Promise<void> {
  for (const caller of fixture.callers) {
    d1.sqlite.prepare("INSERT INTO users (id, github_id, github_login, avatar_url, encrypted_access_token) VALUES (?, ?, ?, ?, ?)")
      .run(caller.id, 1000 + caller.id, caller.login, caller.avatar_url, await encryptToken(TEST_KEY, `dev_${caller.login}`));
  }
  for (const table of TABLES) {
    for (const row of fixture.seed[table] ?? []) {
      const columns = Object.keys(row);
      d1.sqlite.prepare(`INSERT INTO ${table} (${columns.join(", ")}) VALUES (${columns.map(() => "?").join(", ")})`)
        .run(...columns.map((column) => resolve(row[column], now) as string | number | null));
    }
  }
}

// {"$minutes_ago": n} は、入れる時刻の n 分前（SQLite の datetime('now') の形式）
function resolve(value: unknown, now: Date): unknown {
  if (value && typeof value === "object" && "$minutes_ago" in value) {
    const at = new Date(now.getTime() - (value as { $minutes_ago: number }).$minutes_ago * 60_000);
    return at.toISOString().replace("T", " ").replace(/\.\d{3}Z$/, "");
  }
  return value;
}

export function toRequest(c: GoCase): Request {
  const headers = new Headers();
  for (const [name, value] of Object.entries(c.headers)) if (value !== null) headers.set(name, value);
  if (c.token !== null) headers.set("Authorization", `Bearer ${c.token}`);
  const body = c.multipart ? multipartBody(c.multipart) : c.body !== undefined ? new TextEncoder().encode(c.body) : null;
  if (body) headers.set("Content-Length", String(body.byteLength));
  return new Request(`https://begit.example${c.path}`, { method: c.method, headers, body });
}

// 記録に使った record_go.py と同じバイト列を組み立てる
function multipartBody({ boundary, line_ending: nl = "\r\n", omit_closing, parts }: NonNullable<GoCase["multipart"]>): Uint8Array {
  const chunks: Uint8Array[] = [];
  const text = (s: string) => chunks.push(new TextEncoder().encode(s));
  for (const part of parts) {
    let disposition = part.disposition ?? `form-data; name="${part.name}"`;
    if (part.disposition === undefined && part.filename !== null) disposition += `; filename="${part.filename}"`;
    text(`--${boundary}${nl}Content-Disposition: ${disposition}${nl}`);
    if (part.content_type !== null) text(`Content-Type: ${part.content_type}${nl}`);
    for (const header of part.extra_headers ?? []) text(`${header}${nl}`);
    text(nl);
    chunks.push(partData(part));
    text(nl);
  }
  if (!omit_closing) text(`--${boundary}--${nl}`);
  const out = new Uint8Array(chunks.reduce((n, c) => n + c.byteLength, 0));
  let offset = 0;
  for (const chunk of chunks) {
    out.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return out;
}

function partData(part: Part): Uint8Array {
  if (part.hex !== undefined) return hexBytes(part.hex);
  const data = new Uint8Array(part.size!);
  data.set(hexBytes(part.prefix_hex!));
  return data;
}

function hexBytes(hex: string): Uint8Array {
  return new Uint8Array((hex.match(/../g) ?? []).map((byte) => parseInt(byte, 16)));
}

// 記録と同じく、seed にない写真の id は "<new>" にする（検証環境とテストで採番が違うため）
export function normalizePhotos(body: unknown, seedPhotoIDs: Set<number>): unknown {
  if (!body || typeof body !== "object" || !Array.isArray((body as { photos?: unknown }).photos)) return body;
  const photos = (body as { photos: { id: number }[] }).photos;
  return { ...body, photos: photos.map((photo) => ({ ...photo, id: seedPhotoIDs.has(photo.id) ? photo.id : "<new>" })) };
}

// targetRow は記録と同じ形で、対象の投稿の行と写真を読む。created_at は、送った時刻以降に書き換わったか
export function targetRow(d1: FakeD1, target: number, seedPhotoIDs: Set<number>, started: Date): TargetRow {
  const post = d1.sqlite.prepare("SELECT is_draft, status, created_at FROM posts WHERE id = ?").get(target) as
    { is_draft: number; status: string | null; created_at: string } | undefined;
  const photos = d1.sqlite.prepare("SELECT id, photo_type, r2_key FROM photos WHERE post_id = ? ORDER BY id").all(target) as
    { id: number; photo_type: string; r2_key: string }[];
  return {
    post: post ? { is_draft: post.is_draft, status: post.status, created_at_is_now: isRecent(post.created_at, started) } : null,
    photos: photos.map((photo) => ({ ...photo, id: seedPhotoIDs.has(photo.id) ? photo.id : "<new>" })),
  };
}

function isRecent(value: string, started: Date): boolean {
  const at = Date.parse(/[zZ]|[+-]\d{2}:\d{2}$/.test(value) ? value : `${value.replace(" ", "T")}Z`);
  return at >= started.getTime() - 5_000 && at <= Date.now() + 5_000;
}
