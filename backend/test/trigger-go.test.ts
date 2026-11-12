// BeGit Time! のトリガーが Go と同じ応答を返すかを、Go に実際に送って記録した応答（fixtures/trigger-go.json・trigger-once-go.json）で確かめる。
// 記録した条件は fixture の recorded と env にある。Go の挙動が変わったら、test/record/trigger_go.py で同じ cases を Go に送って記録し直す。
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { claimTrigger } from "../src/api/notifications/send";
import { handleWorkerRoute } from "../src/api/router";
import type { ApiEnv } from "../src/api/types";
import type { FakeD1 } from "./fake-d1";
import { FakeQueue, type GoFixture, KEY, setupFixture, toRequest } from "./sql-fixture";

interface TriggerCase {
  name: string;
  method: string;
  path: string;
  token: string | null;
  body: string | null;
  headers: Record<string, string>;
  // ケースの後に読む行（名前 → SQL）
  rows: Record<string, string> | null;
  expect: { status: number; body: unknown; rows?: Record<string, unknown[]> };
}

interface TriggerFixture {
  env: { BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT: string };
  users: GoFixture["users"];
  setup: string[];
  fixed_ids: { notifications: number[]; sprints: number[] };
  cases: TriggerCase[];
}

function load(name: string): TriggerFixture {
  return JSON.parse(readFileSync(join(import.meta.dirname, "fixtures", name), "utf8"));
}

const TIMESTAMP = /^(\d{4}-\d\d-\d\d)[ T](\d\d:\d\d:\d\d)(Z?)$/;

// TriggerNormalizer は記録（test/record/trigger_go.py の Normalizer）と同じ規則で値を置き換える。片方を変えたら、もう片方も同じにする。
//   新しくできた通知・スプリント・配信の仕事の id → 出てきた順に <notification:N> など
//   今から8日以内の時刻 → 5分単位で <now-30m> など（T と Z の形は末尾に " Z"）
class TriggerNormalizer {
  private ids = new Map<string, Map<number, string>>();

  constructor(private fixed: TriggerFixture["fixed_ids"], private now: number) {}

  private id(kind: string, value: unknown, fixed: number[] = []): unknown {
    if (typeof value !== "number" || fixed.includes(value)) return value;
    let table = this.ids.get(kind);
    if (!table) this.ids.set(kind, (table = new Map()));
    if (!table.has(value)) table.set(value, `<${kind}:${table.size + 1}>`);
    return table.get(value);
  }

  private time(value: unknown): unknown {
    if (typeof value !== "string") return value;
    const m = TIMESTAMP.exec(value);
    if (!m) return value;
    const minutes = (Date.parse(`${m[1]}T${m[2]}Z`) - this.now) / 60_000;
    if (Math.abs(minutes) > 8 * 24 * 60) return value;
    // Python の round と同じく、ちょうど半分は偶数へ丸める
    const rounded = roundHalfEven(minutes / 5) * 5;
    const label = rounded === 0 ? "now" : `now${rounded > 0 ? "+" : ""}${rounded}m`;
    return `<${label}${m[3] ? " Z" : ""}>`;
  }

  body(value: unknown): unknown {
    if (!value || typeof value !== "object" || !("id" in value)) return value;
    const body = value as { id: number; sprint_id: number; sent_at: string };
    return {
      ...body,
      id: this.id("notification", body.id, this.fixed.notifications),
      sprint_id: this.id("sprint", body.sprint_id, this.fixed.sprints),
      sent_at: this.time(body.sent_at),
    };
  }

  // keepAudienceOrder: member_ids を指定したケースか（指定しないと Go は対象を map の順に入れるので、並べ替えて比べる）
  rows(rows: Record<string, Record<string, unknown>[]>, keepAudienceOrder = false): Record<string, unknown[]> {
    const out: Record<string, unknown[]> = {};
    for (const [name, items] of Object.entries(rows)) {
      out[name] = items.map((source) => {
        const row = { ...source };
        if (name === "notifications") {
          if (!keepAudienceOrder && !this.fixed.notifications.includes(row.id as number) && typeof row.audience_user_ids === "string") {
            row.audience_user_ids = JSON.stringify((JSON.parse(row.audience_user_ids) as number[]).sort((a, b) => a - b));
          }
          row.id = this.id("notification", row.id, this.fixed.notifications);
          row.sprint_id = this.id("sprint", row.sprint_id, this.fixed.sprints);
        } else if (name === "sprints") {
          row.id = this.id("sprint", row.id, this.fixed.sprints);
        } else if (name === "jobs") {
          row.id = this.id("job", row.id);
          row.event_key = String(row.event_key).replace(/\d+$/, (n) => String(this.id("notification", Number(n), this.fixed.notifications)));
          row.payload_json = String(row.payload_json)
            .replace(/begit_time:(\d+)/g, (_, n) => `begit_time:${this.id("notification", Number(n), this.fixed.notifications)}`)
            .replace(/"occurred_at":"([^"]+)"/, (_, t) => `"occurred_at":"${this.time(t)}"`);
        }
        for (const key of ["sent_at", "stopped_at", "ended_at", "started_at", "ends_at"]) {
          if (key in row) row[key] = this.time(row[key]);
        }
        return row;
      });
    }
    return out;
  }
}

function roundHalfEven(x: number): number {
  const floor = Math.floor(x);
  const diff = x - floor;
  if (diff > 0.5) return floor + 1;
  if (diff < 0.5) return floor;
  return floor % 2 === 0 ? floor : floor + 1;
}

// requestMemberIDs は、ケースの本文で member_ids を指定していれば true（記録の request_member_ids と同じ）
function requestsMembers(c: TriggerCase): boolean {
  try {
    const parsed = JSON.parse(c.body ?? "null") as { member_ids?: unknown } | null;
    return Array.isArray(parsed?.member_ids) && parsed.member_ids.length > 0;
  } catch {
    return false;
  }
}

// Worker が判定せずにコンテナへ転送し、Go に任せるケース（claimTrigger が undefined を返す）。
// どれも Go が 400 にする本文で、Worker は何も書かないので、転送しても Go の応答と同じになる
const FORWARDED_TO_GO = new Set([
  "本文が JSON でない（Content-Type も JSON）",
  "本文が壊れた JSON",
  "本文が配列",
  "game_mode が数字",
  "member_ids が文字列",
  "member_ids に小数",
  "message が数字",
  "member_ids に指数の形の数",
  "member_ids に .0 つきの数",
  "member_ids に int64 を超える数",
]);

type Actual = TriggerCase["expect"] | { forwarded: true };

interface Run {
  d1: FakeD1;
  queue: FakeQueue;
  actual: Map<string, Actual>;
}

// ケースは記録と同じ順で流し、最後のケースの後の時刻を基準に置き換える（記録と同じ）
async function run(fixture: TriggerFixture): Promise<Run> {
  const d1 = await setupFixture(fixture as unknown as GoFixture);
  const queue = new FakeQueue();
  const env: ApiEnv = {
    DB: d1.asD1(),
    DB_ENCRYPTION_KEY: KEY,
    WORKER_ROUTES: "notifications.send",
    BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT: fixture.env.BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT,
    NOTIFICATION_QUEUE: queue.asQueue(),
  };
  const raw: { c: TriggerCase; status: number; body: unknown; rows?: Record<string, Record<string, unknown>[]> }[] = [];
  const forwarded = new Set<string>();
  for (const c of fixture.cases) {
    const response = await handleWorkerRoute(toRequest(c), env);
    if (!response) {
      forwarded.add(c.name);
      continue;
    }
    const rows = c.rows
      ? Object.fromEntries(Object.entries(c.rows).map(([name, sql]) => [name, d1.sqlite.prepare(sql).all() as Record<string, unknown>[]]))
      : undefined;
    raw.push({ c, status: response.status, body: await response.json(), rows });
  }
  const normalize = new TriggerNormalizer(fixture.fixed_ids, Date.now());
  const actual = new Map<string, Actual>();
  for (const name of forwarded) actual.set(name, { forwarded: true });
  for (const { c, status, body, rows } of raw) {
    actual.set(c.name, { status, body: normalize.body(body), ...(rows ? { rows: normalize.rows(rows, requestsMembers(c)) } : {}) });
  }
  return { d1, queue, actual };
}

describe.each([
  ["trigger-go.json（何度でも送れる設定）", "trigger-go.json"],
  ["trigger-once-go.json（1スプリント1人1回）", "trigger-once-go.json"],
])("トリガーが Go と同じ応答を返す（%s）", (_, file) => {
  const fixture = load(file);
  let result: Run;
  beforeAll(async () => {
    result = await run(fixture);
  });

  it.each(fixture.cases.map((c) => [c.name, c] as const))("%s", (name, c) => {
    if (FORWARDED_TO_GO.has(name)) {
      expect(c.expect.status).toBe(400);
      expect(result.actual.get(name)).toEqual({ forwarded: true });
    } else {
      expect(result.actual.get(name)).toEqual(c.expect);
    }
  });
});

describe("トリガーの通知（Go は直接送るが、Worker は Queue に積む）", () => {
  const fixture = load("trigger-go.json");
  let result: Run;
  beforeAll(async () => {
    result = await run(fixture);
  });

  it("作った通知ごとに BeGit Time! の通知を積み、外部通知のチャネルがあれば配信の仕事も積む", () => {
    const notifications = result.d1.sqlite.prepare(
      `SELECT n.id, n.sprint_id, s.group_id FROM notifications n JOIN sprints s ON s.id = n.sprint_id
       WHERE n.id NOT IN (${fixture.fixed_ids.notifications.join(",")}) ORDER BY n.id`,
    ).all() as { id: number; sprint_id: number; group_id: number }[];
    const jobs = result.d1.sqlite.prepare(
      `SELECT j.id, c.group_id FROM notification_delivery_jobs j JOIN notification_channels c ON c.id = j.channel_id ORDER BY j.channel_id`,
    ).all() as { id: number; group_id: number }[];

    const expected = notifications.flatMap((n) => [
      { type: "begit_time", group_id: n.group_id, notification_id: n.id, sprint_id: n.sprint_id },
      ...jobs.filter((job) => job.group_id === n.group_id).map((job) => ({ job_id: job.id })),
    ]);
    expect(notifications.length).toBe(fixture.cases.filter((c) => c.expect.status === 201).length);
    expect(jobs.map((job) => job.group_id)).toEqual([9307, 9307, 9343]);
    expect(result.queue.sent).toEqual(expected);
  });

  it("D1 への往復は認証と batch() の2回（外部通知のチャネルがあれば仕事の作成で1回増える）", async () => {
    const { d1 } = await run({ ...fixture, cases: fixture.cases.filter((c) => c.name === "owner がトリガー") });
    expect(d1.roundTrips).toBe(2);
    const withChannels = await run({ ...fixture, cases: fixture.cases.filter((c) => c.name === "外部通知のチャネルがある") });
    expect(withChannels.d1.roundTrips).toBe(3);
  });
});

describe("通知に失敗しても、トリガーは成功にする（Go と同じくベストエフォート）", () => {
  const fixture = load("trigger-go.json");
  // 外部通知のチャネルが2つあるグループ
  const withChannels = fixture.cases.find((c) => c.name === "外部通知のチャネルがある")!;
  let d1: FakeD1;
  let queue: FakeQueue;
  let env: ApiEnv;

  beforeEach(async () => {
    d1 = await setupFixture(fixture as unknown as GoFixture);
    queue = new FakeQueue();
    env = { DB: d1.asD1(), DB_ENCRYPTION_KEY: KEY, WORKER_ROUTES: "notifications.send", NOTIFICATION_QUEUE: queue.asQueue() };
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  const trigger = () => handleWorkerRoute(toRequest(withChannels), env);
  const notificationCount = () => (d1.sqlite.prepare("SELECT COUNT(*) AS n FROM notifications WHERE sprint_id = 9307").get() as { n: number }).n;

  it("BeGit Time! の通知を積めなくても 201 で、配信の仕事は積む", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    vi.spyOn(queue, "send").mockRejectedValue(new Error("queue down"));
    expect((await trigger())!.status).toBe(201);
    expect(notificationCount()).toBe(1);
    expect(queue.sent.map((m) => Object.keys(m))).toEqual([["job_id"], ["job_id"]]);
  });

  it("配信の仕事を積めなくても 201 で、BeGit Time! の通知は積む", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    vi.spyOn(queue, "sendBatch").mockRejectedValue(new Error("queue down"));
    expect((await trigger())!.status).toBe(201);
    expect(queue.sent).toEqual([expect.objectContaining({ type: "begit_time", group_id: 9307 })]);
  });

  it("配信の仕事を作れなくても 201 で、BeGit Time! の通知は積む", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const batch = d1.batch.bind(d1);
    let calls = 0;
    // 1回目（通知の作成）は通し、2回目（配信の仕事の作成）だけ失敗させる
    vi.spyOn(d1, "batch").mockImplementation((statements) => (++calls === 2 ? Promise.reject(new Error("D1 down")) : batch(statements)));
    expect((await trigger())!.status).toBe(201);
    expect(notificationCount()).toBe(1);
    expect(d1.sqlite.prepare("SELECT COUNT(*) AS n FROM notification_delivery_jobs").get()).toEqual({ n: 0 });
    expect(queue.sent).toEqual([expect.objectContaining({ type: "begit_time", group_id: 9307 })]);
  });

  it("途中でチャネルが消えても、残りのチャネルの仕事は作る", async () => {
    const batch = d1.batch.bind(d1);
    let calls = 0;
    // 通知の作成の後、配信の仕事を作る前にチャネル 9301 を消す
    vi.spyOn(d1, "batch").mockImplementation((statements) => {
      if (++calls === 2) d1.sqlite.exec("DELETE FROM notification_channel_subscriptions WHERE channel_id = 9301; DELETE FROM notification_channels WHERE id = 9301");
      return batch(statements);
    });
    expect((await trigger())!.status).toBe(201);
    expect(d1.sqlite.prepare("SELECT channel_id FROM notification_delivery_jobs").all()).toEqual([{ channel_id: 9304 }]);
    expect(queue.sent.filter((m) => "job_id" in m)).toHaveLength(1);
  });

  it("Queue がない環境でも 201", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    env.NOTIFICATION_QUEUE = undefined;
    expect((await trigger())!.status).toBe(201);
    expect(notificationCount()).toBe(1);
  });
});

describe("本文の読み方（Go と同じに読めないものは Go に任せる）", () => {
  beforeEach(() => {
    vi.spyOn(console, "log").mockImplementation(() => {});
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  const post = (body: BodyInit | null, headers: Record<string, string> = {}) =>
    new Request("https://begit.example/groups/1/notifications", { method: "POST", body, headers, duplex: "half" } as RequestInit);

  it("本文なし・Content-Length: 0・JSON の null は既定値", async () => {
    const empty = { gameMode: "", memberIDs: [], message: "" };
    expect(await claimTrigger(post(null))).toEqual(empty);
    expect(await claimTrigger(post("", { "Content-Length": "0" }))).toEqual(empty);
    expect(await claimTrigger(post(" null ", { "Content-Length": "6" }))).toEqual(empty);
  });

  it("game_mode・member_ids・message を読む（キーの大文字小文字は Go と同じく区別しない）", async () => {
    const body = '{"Game_Mode":"spotlight","member_ids":[3,1,3],"MESSAGE":"見て"}';
    expect(await claimTrigger(post(body))).toEqual({ gameMode: "spotlight", memberIDs: [3, 1, 3], message: "見て" });
  });

  it.each([
    ["後ろに続くゴミ（Go は無視する）", '{"game_mode":"spotlight"} x'],
    ["重複したキー", '{"game_mode":"everyone","GAME_MODE":"spotlight"}'],
    ["member_ids の要素に null（Go は 0 にする）", '{"member_ids":[null]}'],
    ["member_ids に入れ子の配列", '{"member_ids":[[1]]}'],
    ["先頭の BOM", '﻿{"game_mode":"spotlight"}'],
  ])("%s は転送する", async (_, body) => {
    expect(await claimTrigger(post(body))).toBeUndefined();
  });

  it.each([
    ["不正な UTF-8（Go は1バイトごとに U+FFFD にする）", new Uint8Array([...new TextEncoder().encode('{"message":"'), 0xe2, 0x82, 0x41, 0xf0, 0x9f, ...new TextEncoder().encode('"}')])],
    ["対になっていないサロゲート（message）", new TextEncoder().encode('{"message":"\\ud800"}')],
    ["対になっていないサロゲート（game_mode）", new TextEncoder().encode('{"game_mode":"\\udc00"}')],
  ])("%s は転送する", async (_, bytes) => {
    expect(await claimTrigger(post(bytes))).toBeUndefined();
  });

  it("Content-Length のない（chunked の）正しい本文は読む", async () => {
    const stream = new ReadableStream({ start(c) { c.enqueue(new TextEncoder().encode('{"game_mode":"spotlight","member_ids":[2]}')); c.close(); } });
    expect(await claimTrigger(post(stream))).toEqual({ gameMode: "spotlight", memberIDs: [2], message: "" });
  });

  it("64KB を超える本文は読まずに転送する", async () => {
    const large = JSON.stringify({ message: "x".repeat(64 * 1024) });
    expect(await claimTrigger(post(large, { "Content-Length": String(large.length) }))).toBeUndefined();
    // Content-Length がなくても、読みながら上限で止める
    const stream = new ReadableStream({ start(c) { c.enqueue(new TextEncoder().encode(large)); c.close(); } });
    expect(await claimTrigger(post(stream))).toBeUndefined();
  });

  it("Content-Length のない空の本文は転送する（Go は読もうとして 400 にする）", async () => {
    const stream = new ReadableStream({ start(c) { c.close(); } });
    expect(await claimTrigger(post(stream))).toBeUndefined();
  });

  it("転送するときに元の本文が残っている", async () => {
    const request = post('{"game_mode":"spotlight"} x');
    expect(await claimTrigger(request)).toBeUndefined();
    expect(await request.text()).toBe('{"game_mode":"spotlight"} x');
  });
});
