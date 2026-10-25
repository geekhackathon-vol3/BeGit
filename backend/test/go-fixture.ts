// Go に実際に送って記録した fixture（users / setup / cases）を、テスト用の D1 で Worker に流すための共通部分。
// 記録の手順は fixture の about と recorded にある。expect は手で書き換えない。
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { encryptToken } from "../src/api/crypto";
import type { NotificationQueueMessage } from "../src/api/types";
import { FakeD1 } from "./fake-d1";

export interface GoCase {
  name: string;
  method: string;
  path: string;
  token: string | null;
  body: string | null;
  headers: Record<string, string>;
  // ケースの後に読む行の SQL
  rows: string | null;
  expect: { status: number; body: unknown; rows?: unknown[] };
}

export interface GoFixture {
  users: { id: number; login: string; github_id: number; avatar_url: string | null }[];
  setup: string[];
  // setup で入れた行の id（置き換えない）
  fixed_ids: number[];
  cases: GoCase[];
}

export const KEY = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f";

export function loadFixture(name: string): GoFixture {
  return JSON.parse(readFileSync(join(import.meta.dirname, "fixtures", name), "utf8"));
}

// 記録したときの検証環境と同じデータを作る
export async function setupFixture(fixture: GoFixture): Promise<FakeD1> {
  const d1 = new FakeD1();
  for (const user of fixture.users) {
    d1.sqlite.prepare("INSERT INTO users (id, github_id, github_login, avatar_url, encrypted_access_token) VALUES (?, ?, ?, ?, ?)")
      .run(user.id, user.github_id, user.login, user.avatar_url, await encryptToken(KEY, `dev_${user.login}`));
  }
  for (const sql of fixture.setup) d1.sqlite.exec(sql);
  return d1;
}

export function toRequest(c: GoCase): Request {
  const headers: Record<string, string> = { ...c.headers };
  if (c.token !== null) headers.Authorization = `Bearer ${c.token}`;
  const init: RequestInit = { method: c.method, headers };
  if (c.body !== null) {
    init.body = c.body;
    headers["Content-Length"] = String(new TextEncoder().encode(c.body).length);
  }
  return new Request(`https://begit.example${c.path}`, init);
}

// Normalizer は、記録と同じ規則で、新しくできた行の id を出てきた順に <prefix:N> にし、その行の created_at を <now> にする。
// 子を先に、その後で自分の id を置き換える（記録のスクリプトと同じ順）
export class Normalizer {
  private ids = new Map<number, string>();

  constructor(private fixed: number[], private prefix: string) {}

  apply = (value: unknown): unknown => {
    if (Array.isArray(value)) return value.map(this.apply);
    if (typeof value !== "object" || value === null) return value;
    const source = value as Record<string, unknown>;
    const out: Record<string, unknown> = {};
    for (const [key, v] of Object.entries(source)) out[key] = this.apply(v);
    if (typeof source.id === "number" && !this.fixed.includes(source.id)) {
      if (!this.ids.has(source.id)) this.ids.set(source.id, `<${this.prefix}:${this.ids.size + 1}>`);
      out.id = this.ids.get(source.id);
      if ("created_at" in source) {
        const age = Math.abs(Date.now() - Date.parse(String(source.created_at)));
        if (!(age < 600_000)) throw new Error(`created_at of a new row is not now: ${source.created_at}`);
        out.created_at = "<now>";
      }
    }
    return out;
  };
}

// テスト用の Queue。積まれたメッセージを覚える
export class FakeQueue {
  sent: NotificationQueueMessage[] = [];

  async send(message: NotificationQueueMessage): Promise<void> {
    this.sent.push(message);
  }

  asQueue(): Queue<NotificationQueueMessage> {
    return this as unknown as Queue<NotificationQueueMessage>;
  }
}
