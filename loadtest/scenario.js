// BeGit Time! の同時投稿シナリオ（#126 Before 計測。After でも同じものを使う）
//
// 各グループのオーナーがトリガーを発火したあと（setup で全グループ同時に発火する最悪ケース）、
// 全員がほぼ同時に フィード取得 → 投稿作成 → 写真アップロード → 投稿確定 → リアクション を実行する。
//
//   k6 run -e BASE_URL=... -e USERS=15 loadtest/scenario.js
//
// 環境変数
//   BASE_URL        計測先（必須）
//   USERS           同時人数（3の倍数、最大102）。既定 3
//   ROUNDS          1人あたりの周回数（少人数の段階でサンプルを増やす）。既定 1
//   ROUND_INTERVAL  周回の間隔（秒）。既定 5
//   GROUPS_FILE     prepare.js が書き出したファイル。既定 ./groups.json
import { check, sleep } from 'k6';
import http from 'k6/http';
import exec from 'k6/execution';
import { Counter } from 'k6/metrics';
import { api, BASE_URL, d1Totals, GROUP_SIZE, GROUPS_FILE, MAX_USERS, PHOTO, recordD1, tokenOf } from './lib.js';

const USERS = parseInt(__ENV.USERS || '3', 10);
const ROUNDS = parseInt(__ENV.ROUNDS || '1', 10);
const ROUND_INTERVAL = parseFloat(__ENV.ROUND_INTERVAL || '5');

if (USERS % GROUP_SIZE !== 0 || USERS < GROUP_SIZE || USERS > MAX_USERS) {
  throw new Error(`USERS must be a multiple of ${GROUP_SIZE} between ${GROUP_SIZE} and ${MAX_USERS}: ${USERS}`);
}

const prepared = JSON.parse(open(GROUPS_FILE));
const GROUPS = prepared.groups.slice(0, USERS / GROUP_SIZE);

// 合格ラインの判定対象（#125：全エンドポイントの p95 < 1秒、エラー率 < 1%）
const ENDPOINTS = ['trigger', 'feed', 'create_post', 'upload_photo', 'confirm_post', 'reaction'];
const REACTION_TYPES = ['heart', 'thumbsup', 'celebrate', 'fire', 'rocket'];

// 計測の前後で取った /internal/d1-metrics の差分（レスポンス後の非同期の呼び出しも含む上限の枠の消費量）
const d1TotalCalls = new Counter('d1_total_calls');
const d1TotalRateLimited = new Counter('d1_total_rate_limited');

const thresholds = {};
for (const e of ENDPOINTS) {
  thresholds[`http_req_duration{endpoint:${e}}`] = ['p(95)<1000'];
  thresholds[`http_req_failed{endpoint:${e}}`] = ['rate<0.01'];
  // サマリーにエンドポイント別の内訳を出すため（判定には使わない）
  thresholds[`d1_queries{endpoint:${e}}`] = ['avg>=0'];
  thresholds[`d1_time_ms{endpoint:${e}}`] = ['avg>=0'];
}

export const options = {
  setupTimeout: '5m',
  teardownTimeout: '5m',
  summaryTrendStats: ['avg', 'p(50)', 'p(95)', 'p(99)', 'max', 'count'],
  thresholds,
  scenarios: {
    begit_time: {
      executor: 'per-vu-iterations',
      vus: USERS,
      iterations: ROUNDS,
      maxDuration: '15m',
    },
  },
};

const is2xx = (r) => r.status >= 200 && r.status < 300;

function ownerOf(group) {
  return prepared.users[group.members[0]];
}

export function setup() {
  const before = d1Totals();

  // 全グループのオーナーが同時にトリガーする
  const triggers = http.batch(GROUPS.map((g) => ({
    method: 'POST',
    url: `${BASE_URL}/groups/${g.id}/notifications`,
    params: {
      headers: { Authorization: `Bearer ${tokenOf(ownerOf(g).login)}` },
      tags: { endpoint: 'trigger', name: 'trigger' },
      // 409（進行中の BeGit Time! が残っている）はサーバーの不調ではないため、エラー率に含めない
      responseCallback: http.expectedStatuses({ min: 200, max: 299 }, 409),
    },
  })));

  const notifications = GROUPS.map((g, i) => {
    const res = triggers[i];
    recordD1(res, 'trigger');
    check(res, { 'trigger 2xx or 409': (r) => is2xx(r) || r.status === 409 });
    if (is2xx(res)) return res.json('id');
    // 前回の計測が途中で止まり、進行中の BeGit Time! が残っている場合はそれを使う
    const active = api('GET', `/groups/${g.id}/notifications/active`, {
      token: tokenOf(ownerOf(g).login),
      endpoint: 'setup_active',
    });
    if (active.status === 200) return active.json('notification_id');
    throw new Error(`trigger failed for group ${g.id}: ${res.status} ${res.body}`);
  });

  return { before, notifications };
}

export default function (data) {
  const idx = exec.vu.idInTest - 1;
  const user = prepared.users[idx];
  const groupIndex = Math.floor(idx / GROUP_SIZE);
  const group = GROUPS[groupIndex];
  const notificationId = data.notifications[groupIndex];
  const token = tokenOf(user.login);

  if (exec.vu.iterationInScenario > 0) sleep(ROUND_INTERVAL);

  // 1. フィード取得
  const feed = api('GET', `/groups/${group.id}/posts`, { token, endpoint: 'feed' });
  check(feed, { 'feed 2xx': is2xx });
  const posts = is2xx(feed) ? feed.json() : [];
  const teammatePost = (Array.isArray(posts) ? posts : []).find((p) => p.user_id !== user.id);

  // 2. 投稿作成（GitHub はスタブ）
  const created = api('POST', `/groups/${group.id}/posts`, {
    token,
    body: {
      notification_id: notificationId,
      post_type: 'commit',
      content_source: 'github',
      github_login: user.login,
      repo_full_name: group.repo,
    },
    endpoint: 'create_post',
  });
  check(created, { 'create_post 2xx': is2xx });
  if (!is2xx(created)) return;
  const postId = created.json('id');

  // 3. 写真アップロード（R2 はスタブ）
  const photo = api('POST', `/groups/${group.id}/posts/${postId}/photos`, {
    token,
    body: { main: http.file(PHOTO, 'main.png', 'image/png') },
    endpoint: 'upload_photo',
    json: false,
  });
  check(photo, { 'upload_photo 2xx': is2xx });

  // 4. 投稿確定
  const confirmed = api('POST', `/groups/${group.id}/posts/${postId}/confirm`, {
    token,
    body: {},
    endpoint: 'confirm_post',
  });
  check(confirmed, { 'confirm_post 2xx': is2xx });

  // 5. リアクション（メンバーの投稿へ。見つからなければ自分の投稿へ）
  const targetId = teammatePost ? teammatePost.id : postId;
  const reaction = api('POST', `/groups/${group.id}/posts/${targetId}/reactions`, {
    token,
    body: { reaction_type: REACTION_TYPES[idx % REACTION_TYPES.length] },
    endpoint: 'reaction',
  });
  check(reaction, { 'reaction 2xx': is2xx });
}

export function teardown(data) {
  // レスポンス後に非同期で走る D1 呼び出しが終わるのを待ってから総数を取る
  sleep(3);
  const after = d1Totals();
  if (data.before && after) {
    d1TotalCalls.add(after.calls - data.before.calls);
    d1TotalRateLimited.add(after.rate_limited - data.before.rate_limited);
  }

  // 次の回でトリガーできるよう、BeGit Time! を止める（チャレンジ中の1時間は次を発行できないため）
  GROUPS.forEach((g, i) => {
    api('POST', `/groups/${g.id}/notifications/${data.notifications[i]}/stop`, {
      token: tokenOf(ownerOf(g).login),
      endpoint: 'teardown_stop',
    });
  });
}
