// 外部リクエストから Go コンテナの内部用の入口を守る（#131）。テストできるよう index.ts から分けている。

// isInternalPath は Go コンテナの内部専用エンドポイント（/internal/*）かを判定する。
// Go(net/http) はパーセントエンコードを復号してからルーティングするため、
// 復号・連続スラッシュの正規化後のパスで判定し、/%69nternal/cron のような迂回を防ぐ。
export function isInternalPath(pathname: string): boolean {
  let decoded = pathname;
  try {
    decoded = decodeURIComponent(pathname);
  } catch {
    // 不正なエンコードは安全側に倒して内部扱いにする
    return true;
  }
  const normalized = decoded.replace(/\/{2,}/g, "/").toLowerCase();
  return normalized === "/internal" || normalized.startsWith("/internal/");
}

// stripInternalHeaders は、Worker だけが付与すべき内部ヘッダー
// （X-Internal-* と内部エンドポイント用シークレット）を取り除いたヘッダーを返す。
export function stripInternalHeaders(source: Headers): Headers {
  const headers = new Headers(source);
  for (const name of [...headers.keys()]) {
    if (
      name.startsWith("x-internal-") ||
      name === "x-cron-secret" ||
      name === "x-notification-queue-secret"
    ) {
      headers.delete(name);
    }
  }
  return headers;
}
