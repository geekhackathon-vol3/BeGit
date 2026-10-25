import type { Caller } from "./auth";
import type { MeteredDB } from "./db";

// ApiEnv は Worker で処理する API が使う環境（index.ts の Env の一部）。
export interface ApiEnv {
  DB: D1Database;
  DB_ENCRYPTION_KEY: string;
  // Worker で処理する API の名前をカンマ区切りで並べる（例 "posts.confirm"）。空なら全てコンテナへ転送する。
  WORKER_ROUTES?: string;
  D1_METRICS?: string;
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
