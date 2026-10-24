import { MeteredDB } from "./db";
import { errorResponse } from "./errors";
import { confirmPost } from "./posts/confirm";
import type { ApiEnv, RouteContext } from "./types";

interface Route {
  // WORKER_ROUTES に書く名前
  name: string;
  method: string;
  pattern: RegExp;
  handler: (ctx: RouteContext) => Promise<Response>;
}

// Worker で処理できる API（#132）。WORKER_ROUTES に名前を挙げたものだけが有効になる。
// 1つの API は Worker か Go のどちらか一方だけが持つ。
const ROUTES: Route[] = [
  {
    name: "posts.confirm",
    method: "POST",
    pattern: /^\/groups\/([^/]+)\/posts\/([^/]+)\/confirm$/,
    handler: confirmPost,
  },
];

// handleWorkerRoute は WORKER_ROUTES で有効にした API なら Worker で処理し、それ以外は null を返す。
// null のときは、呼び出し元がこれまでどおりコンテナへ転送する。
// 一致しない形のパス（末尾のスラッシュなど）もコンテナへ回り、Go がこれまでどおり処理する。
// パーセントエンコードを含むパスも、Go とデコードの仕方を揃えずに済むよう、すべてコンテナへ回す。
export async function handleWorkerRoute(request: Request, env: ApiEnv): Promise<Response | null> {
  const enabled = enabledRouteNames(env.WORKER_ROUTES);
  if (enabled.size === 0) return null;

  const { pathname } = new URL(request.url);
  if (pathname.includes("%")) return null;
  for (const route of ROUTES) {
    if (!enabled.has(route.name) || route.method !== request.method) continue;
    const match = route.pattern.exec(pathname);
    if (!match) continue;

    const db = new MeteredDB(env.DB);
    let response: Response;
    try {
      response = await route.handler({ request, env, db, params: match.slice(1) });
    } catch (error) {
      // 予期しない失敗も、Go と同じ JSON の 500 で返す
      console.error(`[${route.name}] failed`, error);
      response = errorResponse(500, "internal server error");
    }
    return env.D1_METRICS === "true" ? db.applyMetricsHeaders(response) : response;
  }
  return null;
}

export function enabledRouteNames(value: string | undefined): Set<string> {
  return new Set((value ?? "").split(",").map((name) => name.trim()).filter(Boolean));
}
