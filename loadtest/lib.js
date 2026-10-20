// prepare.js / scenario.js の共通処理（#126 Before 計測。After でも同じものを使う）
import http from 'k6/http';
import { sleep } from 'k6';
import encoding from 'k6/encoding';
import { Counter, Trend } from 'k6/metrics';

export const BASE_URL = (__ENV.BASE_URL || '').replace(/\/$/, '');
if (!BASE_URL) {
  throw new Error('BASE_URL is required (e.g. BASE_URL=https://begit-loadtest.<subdomain>.workers.dev)');
}

export const GROUP_SIZE = 3;
export const MAX_USERS = 102; // 3人 × 34グループ
export const GROUPS_FILE = __ENV.GROUPS_FILE || './groups.json';

export function loginOf(i) {
  return `user${String(i + 1).padStart(3, '0')}`;
}

// /auth/dev が発行する固定トークン
export function tokenOf(login) {
  return `dev_${login}`;
}

// 1x1 の PNG（R2 はスタブなので中身は何でもよい。画像として判定される最小のもの）
export const PHOTO = encoding.b64decode(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==',
);

// サーバー側の計測値（X-D1-* ヘッダー）。D1_METRICS が無効な環境ではヘッダーが無く、記録されない
export const d1Queries = new Trend('d1_queries');
export const d1TimeMs = new Trend('d1_time_ms', true);
// レスポンスに載った D1 呼び出しのうち 429 だった回数（Go 側では 500 に変換されて k6 からは見えないため）
export const d1RateLimited = new Counter('d1_rate_limited');

export function recordD1(res, endpoint) {
  const tags = { endpoint };
  const queries = res.headers['X-D1-Queries'];
  if (queries === undefined) return;
  d1Queries.add(Number(queries), tags);
  d1TimeMs.add(Number(res.headers['X-D1-Time-Ms'] || 0), tags);
  const limited = (res.headers['X-D1-Status'] || '').split(',').filter((s) => s === '429').length;
  if (limited > 0) d1RateLimited.add(limited, tags);
}

// レスポンスに D1 の 429 が含まれていたか
export function hitRateLimit(res) {
  return (res.headers['X-D1-Status'] || '').split(',').includes('429');
}

// API を呼び、エンドポイント名でタグを付けて D1 の計測値も記録する。
// body が文字列以外のオブジェクトで multipart を使う場合は json=false を渡す
export function api(method, path, { token, body, endpoint, json = true } = {}) {
  const headers = {};
  if (token) headers.Authorization = `Bearer ${token}`;
  let payload = body;
  if (body !== undefined && json) {
    headers['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }
  const tags = { endpoint, name: endpoint };
  const res = http.request(method, `${BASE_URL}${path}`, payload === undefined ? null : payload, {
    headers,
    tags,
  });
  recordD1(res, endpoint);
  return res;
}

// サーバー全体の D1 呼び出しの総数（GET /internal/d1-metrics）。無効な環境では null
export function d1Totals() {
  const res = http.get(`${BASE_URL}/internal/d1-metrics`, { tags: { endpoint: 'd1_metrics', name: 'd1_metrics' } });
  if (res.status !== 200) return null;
  return res.json();
}

// 準備作業用：D1 の 429 に当たったらブロックが解けるまで待って再試行する
export function withRateLimitRetry(fn, label) {
  for (let attempt = 1; attempt <= 5; attempt++) {
    const res = fn();
    if (!hitRateLimit(res)) return res;
    console.warn(`[${label}] D1 rate limited (attempt ${attempt}), waiting 65s`);
    sleep(65);
  }
  throw new Error(`[${label}] still rate limited after retries`);
}
