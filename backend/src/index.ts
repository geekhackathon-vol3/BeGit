import { Container, getContainer } from "@cloudflare/containers";

interface Env {
  BEGIT_API: DurableObjectNamespace<BeGitAPI>;
  DB: D1Database;
  PHOTOS: R2Bucket;
  NOTIFICATION_QUEUE: Queue<{ job_id: number }>;
  // Workers Secrets
  GITHUB_CLIENT_ID: string;
  GITHUB_CLIENT_SECRET: string;
  GITHUB_WEBHOOK_SECRET: string;
  // GitHub App credentials. The private key is Base64-encoded before it is
  // forwarded because PEM values contain newlines and cannot be sent in a
  // HTTP header as-is.
  GITHUB_APP_ID: string;
  GITHUB_APP_PRIVATE_KEY: string;
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
  // GitHub App setup completion callback for the iOS custom URL scheme.
  GITHUB_APP_IOS_REDIRECT_URI: string;
  // dev 環境でのみ "true"（[env.dev.vars]）。dev 認証バイパスを有効化する。
  DEV_MODE: string;
  // "true" で BeGit Time! の「1スプリント1人1回」を解除する（[vars] / [env.dev.vars]）。未設定は制限あり。
  BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT: string;
  // 内部 Cron 起動シークレット。dev は [env.dev.vars] の var、本番は secret 運用。
  // scheduled() が X-Cron-Secret ヘッダーで Go コンテナへ転送し、cron_handler が定数時間比較する。
  CRON_SECRET: string;
  NOTIFICATION_QUEUE_SECRET: string;
}

// internalHeaders は Workers Secrets / vars を Go コンテナへ転送する X-Internal-* ヘッダーを構築する。
// fetch / scheduled の双方で再利用する。
function internalHeaders(env: Env): Record<string, string> {
  const encodeHeaderValue = (value: string): string => {
    const bytes = new TextEncoder().encode(value);
    let binary = '';
    for (const byte of bytes) {
      binary += String.fromCharCode(byte);
    }
    return btoa(binary);
  };

  const privateKey = env.GITHUB_APP_PRIVATE_KEY ?? '';
  const firebaseServiceAccountJSON = env.FIREBASE_SERVICE_ACCOUNT_JSON ?? '';

  return {
    'X-Internal-DB-Encryption-Key': env.DB_ENCRYPTION_KEY ?? '',
    'X-Internal-Github-Client-Id': env.GITHUB_CLIENT_ID ?? '',
    'X-Internal-Github-Client-Secret': env.GITHUB_CLIENT_SECRET ?? '',
    'X-Internal-Github-Webhook-Secret': env.GITHUB_WEBHOOK_SECRET ?? '',
    'X-Internal-Github-App-Id': env.GITHUB_APP_ID ?? '',
    'X-Internal-Github-App-Private-Key-B64': privateKey
      ? encodeHeaderValue(privateKey)
      : '',
    // Firebase service account JSON can contain newlines in private_key.
    // Encode it before putting it in an HTTP header.
    'X-Internal-Firebase-Service-Account-B64': firebaseServiceAccountJSON
      ? encodeHeaderValue(firebaseServiceAccountJSON)
      : '',
    'X-Internal-CF-Account-Id': env.CF_ACCOUNT_ID ?? '',
    'X-Internal-D1-Database-Id': env.D1_DATABASE_ID ?? '',
    'X-Internal-CF-Api-Token': env.CF_API_TOKEN ?? '',
    'X-Internal-R2-Access-Key-Id': env.R2_ACCESS_KEY_ID ?? '',
    'X-Internal-R2-Secret-Access-Key': env.R2_SECRET_ACCESS_KEY ?? '',
    'X-Internal-R2-Bucket': env.R2_BUCKET ?? '',
    'X-Internal-App-Base-URL': env.APP_BASE_URL ?? '',
    'X-Internal-Github-App-Ios-Redirect-Uri': env.GITHUB_APP_IOS_REDIRECT_URI ?? '',
    'X-Internal-Cron-Secret': env.CRON_SECRET ?? '',
    'X-Internal-Notification-Queue-Secret': env.NOTIFICATION_QUEUE_SECRET ?? '',
    'X-Internal-Dev-Mode': env.DEV_MODE ?? '',
    'X-Internal-Begit-Time-Allow-Multiple-Per-Sprint': env.BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT ?? '',
  };
}

export class BeGitAPI extends Container {
  defaultPort = 8080;
  sleepAfter = "10m";
}

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    const requestURL = new URL(request.url);
    if (requestURL.pathname === "/internal/notification-queue" && request.method === "POST") {
      const suppliedSecret = request.headers.get("X-Notification-Queue-Secret") ?? "";
      if (!env.NOTIFICATION_QUEUE_SECRET || suppliedSecret !== env.NOTIFICATION_QUEUE_SECRET) {
        return Response.json({ error: "forbidden" }, { status: 403 });
      }
      const rawBody = await request.text();
      if (new TextEncoder().encode(rawBody).byteLength > 1024) {
        return Response.json({ error: "request too large" }, { status: 413 });
      }
      let body: { job_id?: unknown };
      try {
        body = JSON.parse(rawBody) as { job_id?: unknown };
      } catch {
        return Response.json({ error: "invalid request body" }, { status: 400 });
      }
      if (!Number.isSafeInteger(body.job_id) || Number(body.job_id) <= 0) {
        return Response.json({ error: "invalid job_id" }, { status: 422 });
      }
      await env.NOTIFICATION_QUEUE.send({ job_id: Number(body.job_id) });
      return new Response(null, { status: 202 });
    }

    // Workers Secrets と [vars] を X-Internal-* ヘッダーとして Container に転送する
    const modifiedRequest = new Request(request, {
      headers: {
        ...Object.fromEntries(request.headers.entries()),
        ...internalHeaders(env),
      }
    });
    return getContainer(env.BEGIT_API, "begit-api-singleton").fetch(modifiedRequest);
  },

  async queue(batch: MessageBatch<{ job_id: number }>, env: Env): Promise<void> {
    const container = getContainer(env.BEGIT_API, "begit-api-singleton");
    for (const message of batch.messages) {
      try {
        const response = await container.fetch(new Request(
          `${env.APP_BASE_URL}/internal/notification-deliveries/${message.body.job_id}`,
          {
            method: "POST",
            headers: {
              ...internalHeaders(env),
              "X-Notification-Queue-Secret": env.NOTIFICATION_QUEUE_SECRET ?? "",
            },
          },
        ));
        if (response.ok || (response.status >= 400 && response.status < 500)) {
          message.ack();
        } else {
          message.retry();
        }
      } catch {
        message.retry();
      }
    }
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
      getContainer(env.BEGIT_API, "begit-api-singleton").fetch(req).then(async (response) => {
        if (!response.ok) {
          // 失敗理由（コンテナの応答本文）をログに残す。コンテナの標準出力は Workers Logs に出ないため。
          const body = (await response.text().catch(() => "")).slice(0, 500);
          throw new Error(`Cron fetch failed: ${response.status} ${response.statusText} (url: ${url}) body: ${body}`);
        }
      })
    );
  },
};
