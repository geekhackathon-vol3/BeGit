import type { Caller } from "./auth";
import type { MeteredDB } from "./db";

// ApiEnv は Worker で処理する API が使う環境（index.ts の Env の一部）。
export interface ApiEnv {
  DB: D1Database;
  DB_ENCRYPTION_KEY: string;
  // Worker で処理する API の名前をカンマ区切りで並べる（例 "posts.confirm"）。空なら全てコンテナへ転送する。
  WORKER_ROUTES?: string;
  D1_METRICS?: string;
  // 写真（r2.ts）。R2 の認証情報がなければスタブで動く（Go と同じ）
  PHOTOS?: R2Bucket;
  R2_ACCESS_KEY_ID?: string;
  R2_SECRET_ACCESS_KEY?: string;
  CF_ACCOUNT_ID?: string;
  R2_BUCKET?: string;
}

export interface RouteContext {
  request: Request;
  env: ApiEnv;
  db: MeteredDB;
  // URL のパスパラメータ（デコード前の文字列）。params[0] はグループ ID
  params: string[];
  // router の認証と権限チェックを通った呼び出し元
  caller: Caller;
}

// NotificationQueueMessage は NOTIFICATION_QUEUE に積むメッセージ。受けて送るのは Go の裏方のコンテナ（src/queue.ts）
//   { job_id }             : 外部通知の配信の仕事（Go が積む。今までの形）→ POST /internal/notification-deliveries/:jobId
//   { type: "reaction" }   : リアクションの作者への FCM 通知（Worker が積む）→ POST /internal/push-notifications（#134）
//   { type: "begit_time" } : BeGit Time! のグループ全員への FCM 通知（トリガーを移したときに Worker が積む）→ 同上
export type NotificationQueueMessage =
  | { job_id: number }
  | { type: "reaction"; group_id: number; post_id: number; actor_user_id: number; author_user_id: number }
  | { type: "begit_time"; group_id: number; notification_id: number; sprint_id: number };
