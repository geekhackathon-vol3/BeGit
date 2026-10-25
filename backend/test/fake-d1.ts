// テスト用の D1。node:sqlite に migrations/ をすべて当て、D1 バインディングと同じ呼び方で使えるようにする。
// 使う範囲（prepare / bind / first / all / run / batch）だけを実装している。
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";

const MIGRATIONS_DIR = join(import.meta.dirname, "..", "migrations");

type Value = string | number | null;

class FakeStatement {
  constructor(private owner: FakeD1, readonly sql: string, readonly values: Value[] = []) {}

  bind(...values: Value[]): FakeStatement {
    return new FakeStatement(this.owner, this.sql, values);
  }

  async first<T>(): Promise<T | null> {
    this.owner.roundTrips++;
    return (this.owner.sqlite.prepare(this.sql).get(...this.values) as T | undefined) ?? null;
  }

  async all<T>() {
    this.owner.roundTrips++;
    return this.execute<T>();
  }

  async run() {
    this.owner.roundTrips++;
    const result = this.owner.sqlite.prepare(this.sql).run(...this.values);
    return { success: true, results: [], meta: { changes: Number(result.changes), last_row_id: Number(result.lastInsertRowid) } };
  }

  execute<T>() {
    const results = this.owner.sqlite.prepare(this.sql).all(...this.values) as T[];
    return { success: true, results, meta: {} };
  }
}

export class FakeD1 {
  readonly sqlite = new DatabaseSync(":memory:");
  // 往復の回数（batch() は1回）。1回の往復にまとまっているかを確かめるため
  roundTrips = 0;

  constructor() {
    for (const file of readdirSync(MIGRATIONS_DIR).filter((f) => f.endsWith(".sql")).sort()) {
      this.sqlite.exec(readFileSync(join(MIGRATIONS_DIR, file), "utf8"));
    }
  }

  prepare(sql: string): FakeStatement {
    return new FakeStatement(this, sql);
  }

  // D1 の batch() と同じく、まとめて1つのトランザクションで実行する
  async batch(statements: FakeStatement[]) {
    this.roundTrips++;
    this.sqlite.exec("BEGIN");
    try {
      const results = statements.map((statement) => statement.execute());
      this.sqlite.exec("COMMIT");
      return results;
    } catch (error) {
      this.sqlite.exec("ROLLBACK");
      throw error;
    }
  }

  asD1(): D1Database {
    return this as unknown as D1Database;
  }
}
