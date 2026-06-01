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
  // [vars] in wrangler.toml
  CF_ACCOUNT_ID: string;
  D1_DATABASE_ID: string;
  APP_BASE_URL: string;
  // dev 環境でのみ "true"（[env.dev.vars]）。dev 認証バイパスを有効化する。
  DEV_MODE: string;
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
        'X-Internal-DB-Encryption-Key': env.DB_ENCRYPTION_KEY ?? '',
        'X-Internal-Github-Client-Id': env.GITHUB_CLIENT_ID ?? '',
        'X-Internal-Github-Client-Secret': env.GITHUB_CLIENT_SECRET ?? '',
        'X-Internal-Github-Webhook-Secret': env.GITHUB_WEBHOOK_SECRET ?? '',
        'X-Internal-Firebase-Service-Account': env.FIREBASE_SERVICE_ACCOUNT_JSON ?? '',
        'X-Internal-CF-Account-Id': env.CF_ACCOUNT_ID ?? '',
        'X-Internal-D1-Database-Id': env.D1_DATABASE_ID ?? '',
        'X-Internal-CF-Api-Token': env.CF_API_TOKEN ?? '',
        'X-Internal-App-Base-URL': env.APP_BASE_URL ?? '',
        'X-Internal-Dev-Mode': env.DEV_MODE ?? '',
      }
    });
    return getContainer(env.BEGIT_API, "begit-api-singleton").fetch(modifiedRequest);
  },
};
