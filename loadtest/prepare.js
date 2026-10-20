// 計測の準備：ユーザー（user001〜user102）とグループ（3人 × 34）を作る。2段階に分けて実行する。
//
//   1. PHASE=groups（既定）: ログインと、各グループの1人目（owner）によるグループ作成。
//      groups.json と、2・3人目を poster として登録する members.sql を書き出す
//   2. members.sql を D1 に流す（wrangler d1 execute。README 参照）
//   3. PHASE=seed: 1人1件のシード投稿（1周目からリアクション先があるように）
//
// 2・3人目を API（POST /groups）で参加させないのは、スタブの GitHub が全員を同じユーザーとして返し、
// 全員が「GitHub 上の実オーナー」と判定されて owner の付け替え（ClaimOwnership）に入ってしまうため。
// メンバー登録は計測するシナリオに含まれないので、SQL で入れても計測には影響しない。
//
// groups 段階と members.sql は何度実行しても同じ状態になる。seed は実行するたびに投稿が増えるため、
// 流し直すのは途中で失敗したときだけにする。D1 REST の上限（5分で1,200回）を使い切らないよう、
// リクエストの間隔をあけて直列に実行する。
//   k6 run -e BASE_URL=... loadtest/prepare.js
//   k6 run -e BASE_URL=... -e PHASE=seed loadtest/prepare.js
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

const PHASE = __ENV.PHASE || 'groups';
const PREPARE_SLEEP = parseFloat(__ENV.PREPARE_SLEEP || '1.5');
const MEMBERS_SQL_FILE = __ENV.MEMBERS_SQL_FILE || './members.sql';

if (PHASE !== 'groups' && PHASE !== 'seed') {
  throw new Error(`PHASE must be "groups" or "seed": ${PHASE}`);
}

// seed では groups 段階の結果を使う（open は init でしか呼べない）
const prepared = PHASE === 'seed' ? JSON.parse(open(GROUPS_FILE)) : null;

export const options = {
  setupTimeout: '60m',
  scenarios: {
    prepare: { executor: 'shared-iterations', vus: 1, iterations: 1 },
  },
};

function mustSucceed(res, label) {
  const ok = check(res, { [`${label} 2xx`]: (r) => r.status >= 200 && r.status < 300 });
  if (!ok) {
    // Go 側はエラーの中身を返さないため、D1 の内訳も出して原因の見当をつけられるようにする
    const d1 = `X-D1-Queries=${res.headers['X-D1-Queries']} X-D1-Status=${res.headers['X-D1-Status']}`;
    throw new Error(`${label} failed: ${res.status} ${res.body} (${d1})`);
  }
  return res;
}

// レスポンスから ID を取り出す。groups.json と members.sql にそのまま入るため、整数でなければ止める
function mustGetId(res, path, label) {
  const id = res.json(path);
  if (!Number.isInteger(id)) {
    throw new Error(`${label}: response has no integer ${path}: ${res.body}`);
  }
  return id;
}

function prepareGroups() {
  const users = [];
  for (let i = 0; i < MAX_USERS; i++) {
    const login = loginOf(i);
    const res = withRateLimitRetry(
      () => api('POST', '/auth/dev', { body: { login }, endpoint: 'prepare_login' }),
      `login ${login}`,
    );
    users.push({ login, id: mustGetId(mustSucceed(res, `login ${login}`), 'user.id', `login ${login}`) });
    sleep(PREPARE_SLEEP);
  }

  const groups = [];
  for (let g = 0; g < MAX_USERS / GROUP_SIZE; g++) {
    const repo = `loadtest/group-${String(g + 1).padStart(2, '0')}`;
    const members = [0, 1, 2].map((m) => g * GROUP_SIZE + m);
    const owner = users[members[0]].login;
    const res = withRateLimitRetry(
      () => api('POST', '/groups', {
        token: tokenOf(owner),
        body: { repo_full_name: repo, name: repo },
        endpoint: 'prepare_group',
      }),
      `group ${repo} ${owner}`,
    );
    const label = `group ${repo} ${owner}`;
    groups.push({ id: mustGetId(mustSucceed(res, label), 'id', label), repo, members });
    sleep(PREPARE_SLEEP);
  }

  return { users, groups };
}

function seedPosts() {
  for (const group of prepared.groups) {
    for (const idx of group.members) {
      const login = prepared.users[idx].login;
      const res = withRateLimitRetry(
        () => api('POST', `/groups/${group.id}/posts`, {
          token: tokenOf(login),
          body: { post_type: 'memo', content_source: 'manual', body: 'seed', repo_full_name: group.repo },
          endpoint: 'prepare_seed_post',
        }),
        `seed post ${login}`,
      );
      if (res.status === 403) {
        throw new Error(`seed post ${login} failed: 403 forbidden — members.sql を D1 に適用しましたか？（README の準備 2）`);
      }
      mustSucceed(res, `seed post ${login}`);
      sleep(PREPARE_SLEEP);
    }
  }
  return prepared;
}

export function setup() {
  return PHASE === 'groups' ? prepareGroups() : seedPosts();
}

export default function () {}

// 2・3人目を poster として登録する SQL（INSERT OR IGNORE なので何度流してもよい）
function membersSQL({ users, groups }) {
  const rows = [];
  for (const group of groups) {
    for (const idx of group.members.slice(1)) {
      rows.push(`INSERT OR IGNORE INTO group_members (group_id, user_id, role) VALUES (${group.id}, ${users[idx].id}, 'poster');`);
    }
  }
  return `${rows.join('\n')}\n`;
}

export function handleSummary(data) {
  const result = data.setup_data;
  // setup が例外で止まっても handleSummary は呼ばれる。ここで落ちると本来のエラーが埋もれるため、何も書き出さずに終える
  if (!result) {
    console.error('準備が途中で失敗しました。上に出ているエラーを確認してください（groups.json / members.sql は書き出していません）');
    return {};
  }
  if (PHASE === 'seed') {
    console.log(`seeded posts for ${result.users.length} users`);
    return {};
  }
  console.log(`prepared ${result.users.length} users / ${result.groups.length} groups -> ${GROUPS_FILE}, ${MEMBERS_SQL_FILE}`);
  return {
    [GROUPS_FILE]: JSON.stringify(result, null, 2),
    [MEMBERS_SQL_FILE]: membersSQL(result),
  };
}
