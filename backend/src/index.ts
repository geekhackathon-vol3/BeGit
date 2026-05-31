import { Container, getContainer } from "@cloudflare/containers";

interface Env {
  BEGIT_API: DurableObjectNamespace<BeGitAPI>;
  DB: D1Database;
  PHOTOS: R2Bucket;
  GITHUB_CLIENT_SECRET: string;
  GITHUB_WEBHOOK_SECRET: string;
  FIREBASE_SERVICE_ACCOUNT_JSON: string;
  DB_ENCRYPTION_KEY: string;
}

export class BeGitAPI extends Container {
  defaultPort = 8080;
  sleepAfter = "10m";
}

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    return getContainer(env.BEGIT_API, "begit-api-singleton").fetch(request);
  },
};
