import { readCloneUpTo } from "../body";
import { errorResponse } from "../errors";
import { decodeGoObject } from "../gojson";
import { formatTimestamp } from "../posts/post";
import type { NotificationQueueMessage, RouteContext } from "../types";

// Go の notificationService.SendNotification と同じ既定の文言（本文の message が空のとき）
const DEFAULT_MESSAGE = "今、なに作ってる？";
// Go の SendNotification は GetOrCreateCurrentSprint(ctx, groupID, 7) で作る
const SPRINT_DAYS = 7;
// Go の model.GameMode（#164）
const GAME_MODES = new Set(["everyone", "spotlight"]);
const DEFAULT_GAME_MODE = "everyone";

// 開催中のスプリント（Go の sprintRepository.GetCurrentSprint）
const CURRENT_SPRINT =
  "SELECT id FROM sprints WHERE group_id = ? AND started_at <= datetime('now') AND ends_at > datetime('now') LIMIT 1";
// 通知の対象にできる人（Go の SendNotification の eligible。旧 role の member は poster と読む = model.NormalizeGroupRole）。
// 0016 の notification_capture_members は owner / poster だけを入れるので、旧 role の member は検査を通っても
// notification_members には入らない（Go と同じ）
const ELIGIBLE_MEMBERS = "SELECT user_id FROM group_members WHERE group_id = ? AND role IN ('owner', 'poster', 'member')";

// 本文は game_mode・member_ids・message の3つだけなので、これより大きいものは読まずに Go に任せる
const MAX_BODY_BYTES = 64 * 1024;

// Go の SendNotificationRequestJSON
const SEND_FIELDS = { game_mode: "string", member_ids: "[]int64", message: "string" } as const;

export interface TriggerInput {
  gameMode: string;
  memberIDs: number[];
  message: string;
}

interface GroupRow {
  completed_at: string | null;
  name: string;
  login: string | null;
}

interface NotificationRow {
  id: number;
  sprint_id: number;
  sent_at: string;
  game_mode: string;
}

// claimTrigger は、Go の NotificationHandler.Send と同じに本文を読めるなら、その入力を返す。
// Go は Content-Length が 0 なら本文を読まず、それ以外は ShouldBindJSON で読む（壊れた JSON・型の誤りは 400）。
// 400 になる本文と、Go と読み方がずれうる本文（後ろに続くゴミ、重複したキー、大きな本文など。gojson.ts）は
// undefined を返してコンテナへ転送し、Go に任せる。転送に元の本文を使えるよう、読むのは request.clone()。認証より前に呼ばれる
export async function claimTrigger(request: Request): Promise<TriggerInput | undefined> {
  const empty = { gameMode: "", memberIDs: [], message: "" };
  const contentLength = request.headers.get("Content-Length");
  if (!request.body || contentLength === "0") return empty;
  if (Number(contentLength ?? "0") > MAX_BODY_BYTES) return forward("too large");
  const bytes = await readCloneUpTo(request, MAX_BODY_BYTES);
  if (bytes === null) return forward("too large");
  if (bytes.byteLength === 0) return forward("empty body without Content-Length");
  // 不正な UTF-8 は転送する（Go は不正な1バイトごとに U+FFFD にするが、TextDecoder はまとめて1つにするので、message がずれる）。
  // 先頭の BOM を残す（Go は BOM 付きの JSON をエラーにするので、JSON.parse でも失敗させて転送する）
  let text: string;
  try {
    text = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(bytes);
  } catch {
    return forward("invalid UTF-8");
  }
  // 本文が JSON の null なら、Go は何も読まずに既定値で進める
  if (isJSONNull(text)) return empty;
  const fields = decodeGoObject(text, SEND_FIELDS);
  if (!fields) return forward("body Go may read differently");
  const gameMode = (fields.game_mode as string | undefined) ?? "";
  const message = (fields.message as string | undefined) ?? "";
  // "\ud800" のような対になっていないサロゲートは、Go は U+FFFD にするが JavaScript はそのまま持つので転送する
  if (LONE_SURROGATE.test(gameMode) || LONE_SURROGATE.test(message)) return forward("lone surrogate");
  return { gameMode, memberIDs: (fields.member_ids as number[] | null | undefined) ?? [], message };
}

// 対になっていないサロゲート（String.prototype.isWellFormed は ES2024 で、tsconfig の lib にないため正規表現で見る）
const LONE_SURROGATE = /[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/;

// forward は本文を判定せずにコンテナへ転送させる（理由をログに残す。どの本文がどれだけ Go に回ったかを後から見られるように）
function forward(reason: string): undefined {
  console.log(`[notifications.send] forwarded to container: ${reason}`);
  return undefined;
}

function isJSONNull(text: string): boolean {
  try {
    return JSON.parse(text) === null;
  } catch {
    return false;
  }
}

// sendNotification は POST /groups/:id/notifications（BeGit Time! のトリガー）。
// Go の NotificationHandler.Send → notificationService.SendNotification と同じルールで通知を作る。エラーの順も Go と同じ。
//   game_mode が everyone / spotlight 以外 → 422（グループを見る前）
//   完成済みのグループ → 409
//   member_ids に対象にできない人（viewer・メンバーでない人）がいる → 422
//   1時間以内に進行中（止めても中断してもいない）の通知がある、または
//   BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT が "true" でなく、このスプリントで送っている（定期の通知は数えない）→ 409
// 開催中のスプリントがなければ作る（7日間）。検査に通らないときは、Go と同じくスプリントも作らない。
// 対象（audience_user_ids）は、member_ids を重複を除いてその順に並べ、発行者がいなければ末尾に足したもの。
// member_ids が空なら対象にできる人全員（Go は map の順で毎回変わる。ここもサブクエリの ORDER BY に頼るだけで、順は保証しない）。
// 同時に押されても2つ作られないよう、Go と同じく条件つきの INSERT 1つで作る。検査と書き込みは1回の batch() にまとめる。
// 定期の BeGit Time!（Cron）は Go の notificationRepository.CreateIfNoActive のまま。進行中の条件を変えるときは両方を変える。
// 通知は Queue に積むだけにする（FCM は Go の裏方のコンテナが、通知の対象メンバーにモードの文言で送る。SendBeGitTime）。
export async function sendNotification({ env, db, caller, input }: RouteContext): Promise<Response> {
  const request = input as TriggerInput;
  const gameMode = request.gameMode || DEFAULT_GAME_MODE;
  if (!GAME_MODES.has(gameMode)) return errorResponse(422, "invalid game mode or notification members");
  const message = request.message || DEFAULT_MESSAGE;
  // 指定された対象（重複を除き、発行者を末尾に足す）。空なら null（対象にできる人全員を SQL で入れる）
  const requested = request.memberIDs.length > 0 ? [...new Set([...request.memberIDs, caller.userID])] : null;
  const requestedJSON = requested ? JSON.stringify(requested) : "[]";

  const group = caller.groupID;
  // 指定された対象が全員、対象にできる人であること
  const audienceValid = `NOT EXISTS (SELECT 1 FROM json_each(?) j WHERE j.value NOT IN (${ELIGIBLE_MEMBERS}))`;
  const oncePerSprint = env.BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT !== "true"
    ? " AND NOT EXISTS (SELECT 1 FROM notifications WHERE sprint_id = s.id AND sent_by = ? AND source = 'manual')"
    : "";

  const [groups, eligible, , created, channels] = await db.batch<unknown>([
    db.prepare(
      `SELECT g.completed_at, g.name, u.github_login AS login
       FROM groups g LEFT JOIN users u ON u.id = ? WHERE g.id = ?`,
    ).bind(caller.userID, group),
    db.prepare(ELIGIBLE_MEMBERS).bind(group),
    // 開催中のスプリントがなければ作る。index_num はそのグループの最大値 + 1（Go と同じ）
    db.prepare(
      `INSERT INTO sprints (group_id, index_num, started_at, ends_at)
       SELECT ?, (SELECT COALESCE(MAX(index_num), -1) + 1 FROM sprints WHERE group_id = ?), datetime('now'), datetime('now', '+${SPRINT_DAYS} days')
       WHERE (SELECT completed_at FROM groups WHERE id = ?) IS NULL AND ${audienceValid} AND NOT EXISTS (${CURRENT_SPRINT})`,
    ).bind(group, group, group, requestedJSON, group, group),
    db.prepare(
      `INSERT INTO notifications (sprint_id, sent_by, message, source, game_mode, audience_user_ids)
       SELECT s.id, ?, ?, 'manual', ?, COALESCE(?, (SELECT json_group_array(user_id) FROM (${ELIGIBLE_MEMBERS} ORDER BY user_id)))
       FROM sprints s JOIN groups g ON g.id = s.group_id
       WHERE s.id = (${CURRENT_SPRINT}) AND g.completed_at IS NULL AND ${audienceValid}
         AND NOT EXISTS (
           SELECT 1 FROM notifications
           WHERE sprint_id = s.id AND stopped_at IS NULL AND ended_at IS NULL AND datetime(sent_at, '+1 hour') > datetime('now')
         )${oncePerSprint}
       RETURNING id, sprint_id, sent_at, game_mode`,
    ).bind(
      caller.userID, message, gameMode, requested ? requestedJSON : null, group,
      group, requestedJSON, group, ...(oncePerSprint ? [caller.userID] : []),
    ),
    // 外部通知を受け取るチャネル（Go の notificationChannelRepository.ListEnabledForEvent）
    db.prepare(
      `SELECT c.id FROM notification_channels c
       WHERE c.group_id = ? AND c.enabled = 1
         AND EXISTS (SELECT 1 FROM notification_channel_subscriptions x WHERE x.channel_id = c.id AND x.event_type = 'begit_time')
       ORDER BY c.id`,
    ).bind(group),
  ]);

  const groupRow = groups.results[0] as GroupRow | undefined;
  const notification = created.results[0] as NotificationRow | undefined;
  if (!notification) {
    // Go の service.ErrProjectCompleted → ErrValidation → ErrConflict の順。
    // 認証の後にグループが消えていたときは 409 になる（Go は GetByID の ErrNotFound を変換せず 500。実際には起きない）
    if (groupRow?.completed_at != null) return errorResponse(409, "conflict: project is completed");
    const eligibleIDs = new Set((eligible.results as { user_id: number }[]).map((row) => row.user_id));
    if (requested?.some((id) => !eligibleIDs.has(id))) return errorResponse(422, "invalid game mode or notification members");
    return errorResponse(409, "conflict: a challenge is in progress or already sent in this sprint");
  }

  const sentAt = formatTimestamp(notification.sent_at);
  const jobIDs = await createDeliveryJobs(
    db,
    (channels.results as { id: number }[]).map((row) => row.id),
    deliveryPayload(notification.id, group, groupRow?.name ?? "", groupRow?.login, sentAt, notification.game_mode),
  );
  await enqueue(env, { type: "begit_time", group_id: group, notification_id: notification.id, sprint_id: notification.sprint_id }, jobIDs);

  // Go の NotificationJSON。作ったばかりの通知に ended_at はない
  return Response.json(
    { id: notification.id, sprint_id: notification.sprint_id, sent_at: sentAt, game_mode: notification.game_mode },
    { status: 201 },
  );
}

// createDeliveryJobs は外部通知の配信の仕事を作り、その id を返す（Go の externalNotificationPublisher.Publish）。
// Go と同じく、失敗してもトリガーは失敗にしない。途中で消えたチャネルは0行になるだけで、ほかのチャネルの仕事は作る。
// Go はチャネルごとに INSERT するが、ここは1回の batch() なので、D1 のエラーでは全チャネルの仕事がまとめて作られない（ベストエフォートなので許容する）
async function createDeliveryJobs(db: RouteContext["db"], channelIDs: number[], payload: { key: string; json: string }): Promise<number[]> {
  if (channelIDs.length === 0) return [];
  try {
    const results = await db.batch<{ id: number }>(channelIDs.map((channelID) =>
      db.prepare(
        `INSERT INTO notification_delivery_jobs (event_key, channel_id, event_type, payload_json)
         SELECT ?, c.id, 'begit_time', ? FROM notification_channels c WHERE c.id = ?
         ON CONFLICT (event_key, channel_id) DO NOTHING RETURNING id`,
      ).bind(payload.key, payload.json, channelID)
    ));
    return results.flatMap((result) => result.results.map((row) => row.id));
  } catch (error) {
    console.error(`[notifications.send] delivery jobs failed event=${payload.key}`, error);
    return [];
  }
}

// deliveryPayload は Go の model.NotificationEvent を json.Marshal したものと同じ文字列を作る（Go の SendNotification の event）。
// 項目の順は構造体の順。fields は Go の map なので、キーをバイト順（UTF-8）に並べる（ゲームモード → 残り時間 → 発行者）。
// 本文は固定の文言で、トリガーの message は使わない（Go と同じ）
function deliveryPayload(
  notificationID: number, groupID: number, groupName: string, login: string | null | undefined, sentAt: string, gameMode: string,
) {
  const key = `begit_time:${notificationID}`;
  // Go はメンバー一覧から発行者を探し、見つかれば "@" + Login（空でも "@"）。発行者は必ずメンバーなので、ユーザーがいれば "@" をつける
  const issuer = login == null ? "チームメンバー" : `@${login}`;
  const json = `{"key":${goString(key)},"type":"begit_time","group_id":${groupID},"group_name":${goString(groupName)},` +
    `"title":"⏰ BeGit Time!","body":${goString("今、なに作ってる？ 1時間だけ、みんなで進捗を草にしよう。")},"accent_color":"#39D353",` +
    `"fields":{"ゲームモード":${goString(gameMode)},"残り時間":"60分","発行者":${goString(issuer)}},"occurred_at":${goString(sentAt)}}`;
  return { key, json };
}

// goString は Go の json.Marshal と同じく、HTML の特殊文字と U+2028 / U+2029 もエスケープした JSON 文字列にする
function goString(value: string): string {
  return JSON.stringify(value).replace(/[<>&\u2028\u2029]/g, (c) => `\\u${c.charCodeAt(0).toString(16).padStart(4, "0")}`);
}

// sendBatch に一度に積めるメッセージの上限（Cloudflare Queues）
const QUEUE_BATCH_LIMIT = 100;

// enqueue は通知を Queue に積む（受けて送るのは Go の裏方のコンテナ。src/queue.ts）。
// Go と同じく、通知の失敗ではトリガーを失敗にしない。
// Go と同じく FCM と外部通知は別々に積み、片方の失敗がもう片方を巻き込まないようにする
async function enqueue(env: RouteContext["env"], push: NotificationQueueMessage, jobIDs: number[]): Promise<void> {
  const queue = env.NOTIFICATION_QUEUE;
  if (!queue) {
    console.warn("[notifications.send] NOTIFICATION_QUEUE is not bound; notification skipped");
    return;
  }
  try {
    await queue.send(push);
  } catch (error) {
    console.error("[notifications.send] queue send failed type=begit_time", error);
  }
  for (let i = 0; i < jobIDs.length; i += QUEUE_BATCH_LIMIT) {
    const chunk = jobIDs.slice(i, i + QUEUE_BATCH_LIMIT);
    try {
      await queue.sendBatch(chunk.map((id) => ({ body: { job_id: id } })));
    } catch (error) {
      console.error(`[notifications.send] queue send failed job_ids=${chunk.join(",")}`, error);
    }
  }
}
