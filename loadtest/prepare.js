// 計測の準備：ユーザー（user001〜user102）とグループ（3人 × 34）を作り、groups.json に書き出す。
// 何度実行しても同じ状態になる（ログインは UPSERT、2回目以降のグループ作成は既存グループへの参加になる）。
//
// D1 REST の上限（5分で1,200回）を準備作業で使い切らないよう、リクエストの間隔をあけて直列に実行する。
//   k6 run -e BASE_URL=... loadtest/prepare.js
import { check, sleep } from 'k6';
import {
  api,
  GROUP_SIZE,
  GROUPS_FILE,
  loginOf,
  MAX_USERS,
  tokenOf,
  withRateLimitRetry,
} from './lib.js';

const PREPARE_SLEEP = parseFloat(__ENV.PREPARE_SLEEP || '1.5');

export const options = {
  setupTimeout: '60m',
  scenarios: {
    prepare: { executor: 'shared-iterations', vus: 1, iterations: 1 },
  },
};

function mustSucceed(res, label) {
  const ok = check(res, { [`${label} 2xx`]: (r) => r.status >= 200 && r.status < 300 });
  if (!ok) throw new Error(`${label} failed: ${res.status} ${res.body}`);
  return res;
}

export function setup() {
  const users = [];
  for (let i = 0; i < MAX_USERS; i++) {
    const login = loginOf(i);
    const res = withRateLimitRetry(
      () => api('POST', '/auth/dev', { body: { login }, endpoint: 'prepare_login' }),
      `login ${login}`,
    );
    users.push({ login, id: mustSucceed(res, `login ${login}`).json('user.id') });
    sleep(PREPARE_SLEEP);
  }

  const groups = [];
  for (let g = 0; g < MAX_USERS / GROUP_SIZE; g++) {
    const repo = `loadtest/group-${String(g + 1).padStart(2, '0')}`;
    let groupId = null;
    // 1人目が owner として作成し、2人目以降は同じ repo_full_name で既存グループに参加する
    for (let m = 0; m < GROUP_SIZE; m++) {
      const login = users[g * GROUP_SIZE + m].login;
      const res = withRateLimitRetry(
        () => api('POST', '/groups', {
          token: tokenOf(login),
          body: { repo_full_name: repo, name: repo },
          endpoint: 'prepare_group',
        }),
        `group ${repo} ${login}`,
      );
      groupId = mustSucceed(res, `group ${repo} ${login}`).json('id');
      sleep(PREPARE_SLEEP);
    }
    groups.push({ id: groupId, repo, members: [0, 1, 2].map((m) => g * GROUP_SIZE + m) });
  }

  // 1周目からフィードにメンバーの投稿があり、リアクション先を選べるように1人1件ずつ投稿しておく
  for (const group of groups) {
    for (const idx of group.members) {
      const login = users[idx].login;
      const res = withRateLimitRetry(
        () => api('POST', `/groups/${group.id}/posts`, {
          token: tokenOf(login),
          body: { post_type: 'memo', content_source: 'manual', body: 'seed', repo_full_name: group.repo },
          endpoint: 'prepare_seed_post',
        }),
        `seed post ${login}`,
      );
      mustSucceed(res, `seed post ${login}`);
      sleep(PREPARE_SLEEP);
    }
  }

  return { users, groups };
}

export default function () {}

export function handleSummary(data) {
  const prepared = data.setup_data;
  console.log(`prepared ${prepared.users.length} users / ${prepared.groups.length} groups -> ${GROUPS_FILE}`);
  return { [GROUPS_FILE]: JSON.stringify(prepared, null, 2) };
}
