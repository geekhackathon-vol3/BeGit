// MeteredDB は D1 バインディングへの往復を数える。
// D1_METRICS=true のとき、#126 / #141 の計測で使う X-D1-* ヘッダーで返す。
// 計測では REST の HTTP 呼び出し1回を1クエリと数えたため、こちらも往復1回（batch() は1回）で数える。
export class MeteredDB {
  private queries = 0;
  private timeMs = 0;
  private statuses: string[] = [];

  constructor(readonly db: D1Database) {}

  prepare(sql: string): D1PreparedStatement {
    return this.db.prepare(sql);
  }

  first<T = Record<string, unknown>>(statement: D1PreparedStatement): Promise<T | null> {
    return this.measure(() => statement.first<T>());
  }

  async all<T = Record<string, unknown>>(statement: D1PreparedStatement): Promise<T[]> {
    const result = await this.measure(() => statement.all<T>());
    return result.results;
  }

  batch<T = Record<string, unknown>>(statements: D1PreparedStatement[]): Promise<D1Result<T>[]> {
    return this.measure(() => this.db.batch<T>(statements));
  }

  applyMetricsHeaders(response: Response): Response {
    const headers = new Headers(response.headers);
    headers.set("X-D1-Queries", String(this.queries));
    headers.set("X-D1-Time-Ms", this.timeMs.toFixed(1));
    headers.set("X-D1-Status", this.statuses.join(","));
    return new Response(response.body, { status: response.status, statusText: response.statusText, headers });
  }

  private async measure<T>(fn: () => Promise<T>): Promise<T> {
    const started = Date.now();
    try {
      const result = await fn();
      this.statuses.push("200");
      return result;
    } catch (error) {
      this.statuses.push("500");
      throw error;
    } finally {
      this.queries++;
      this.timeMs += Date.now() - started;
    }
  }
}
