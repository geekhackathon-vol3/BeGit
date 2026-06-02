import { Container, getContainer } from "@cloudflare/containers";

interface Env {
  BEGIT_API: DurableObjectNamespace<BeGitAPI>;
  DB: D1Database;
  PHOTOS: R2Bucket;
  // Workers Secrets
  GITHUB_CLIENT_ID: string;
  GITHUB_CLIENT_SECRET: string;
  GITHUB_WEBHOOK_SECRET: string;
  FIREBASE_SERVICE_ACCOUNT_JSON: string;
  DB_ENCRYPTION_KEY: string;
  CF_API_TOKEN: string;
  // R2 S3 互換 API 認証情報（Secrets）。R2 ダッシュボードで発行する Access Key。
  R2_ACCESS_KEY_ID: string;
  R2_SECRET_ACCESS_KEY: string;
  // [vars] in wrangler.toml
  CF_ACCOUNT_ID: string;
  D1_DATABASE_ID: string;
  R2_BUCKET: string;
  APP_BASE_URL: string;
  // dev 環境でのみ "true"（[env.dev.vars]）。dev 認証バイパスを有効化する。
  DEV_MODE: string;
  // 内部 Cron 起動シークレット。dev は [env.dev.vars] の var、本番は secret 運用。
  // scheduled() が X-Cron-Secret ヘッダーで Go コンテナへ転送し、cron_handler が定数時間比較する。
  CRON_SECRET: string;
}

// internalHeaders は Workers Secrets / vars を Go コンテナへ転送する X-Internal-* ヘッダーを構築する。
// fetch / scheduled の双方で再利用する。
function internalHeaders(env: Env): Record<string, string> {
  return {
    'X-Internal-DB-Encryption-Key': env.DB_ENCRYPTION_KEY ?? '',
    'X-Internal-Github-Client-Id': env.GITHUB_CLIENT_ID ?? '',
    'X-Internal-Github-Client-Secret': env.GITHUB_CLIENT_SECRET ?? '',
    'X-Internal-Github-Webhook-Secret': env.GITHUB_WEBHOOK_SECRET ?? '',
    'X-Internal-Firebase-Service-Account': env.FIREBASE_SERVICE_ACCOUNT_JSON ?? '',
    'X-Internal-CF-Account-Id': env.CF_ACCOUNT_ID ?? '',
    'X-Internal-D1-Database-Id': env.D1_DATABASE_ID ?? '',
    'X-Internal-CF-Api-Token': env.CF_API_TOKEN ?? '',
    'X-Internal-R2-Access-Key-Id': env.R2_ACCESS_KEY_ID ?? '',
    'X-Internal-R2-Secret-Access-Key': env.R2_SECRET_ACCESS_KEY ?? '',
    'X-Internal-R2-Bucket': env.R2_BUCKET ?? '',
    'X-Internal-App-Base-URL': env.APP_BASE_URL ?? '',
    'X-Internal-Dev-Mode': env.DEV_MODE ?? '',
  };
}

export class BeGitAPI extends Container {
  defaultPort = 8080;
  sleepAfter = "10m";
}

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    // Workers Secrets と [vars] を X-Internal-* ヘッダーとして Container に転送する
    const modifiedRequest = new Request(request, {
      headers: {
        ...Object.fromEntries(request.headers.entries()),
        ...internalHeaders(env),
      }
    });
    return getContainer(env.BEGIT_API, "begit-api-singleton").fetch(modifiedRequest);
  },

  // scheduled は [triggers] crons から起動される。
  //   "* * * * *"（毎分） → kind=minutely（③ challenge_end）
  //   "0 0 * * *"（日次）  → kind=daily   （④ sprint_reminder / ⑤ sprint_end / ⑥ sprint_start）
  // X-Internal-* に加えて X-Cron-Secret を付け、Container の POST /internal/cron?kind= へ転送する。
  // controller.cron（= ScheduledController.cron）の分秒指定で minutely/daily を振り分ける。
  async scheduled(controller: ScheduledController, env: Env, ctx: ExecutionContext): Promise<void> {
    // 日次トリガー "0 0 * * *" は分=0・時=0。それ以外（毎分トリガー）は minutely。
    const kind = controller.cron === "0 0 * * *" ? "daily" : "minutely";

    const url = `${env.APP_BASE_URL}/internal/cron?kind=${kind}`;
    const req = new Request(url, {
      method: "POST",
      headers: {
        ...internalHeaders(env),
        "X-Cron-Secret": env.CRON_SECRET ?? "",
      },
    });

    ctx.waitUntil(
      getContainer(env.BEGIT_API, "begit-api-singleton").fetch(req).then(() => undefined)
    );
  },
};
