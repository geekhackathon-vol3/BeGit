#!/usr/bin/env python3
# BeGit Time! のトリガー（POST /groups/:id/notifications）の Go の応答を記録し、test/fixtures/trigger-go.json・trigger-once-go.json を作る（#138）。
# 記録したものは test/trigger-go.test.ts が Worker の応答と比べる。値の置き換えの規則（Normalizer）は、そのテストの TriggerNormalizer と同じにしておく。
#
# 使い方（DEV_MODE=true で、WORKER_ROUTES に notifications.send を入れていない環境に対して）
#   export BASE_URL=https://<計測用の Worker>  WRANGLER_ENV=<wrangler.toml の env 名>  D1_DATABASE=<D1 の名前>  CLOUDFLARE_ACCOUNT_ID=<アカウント ID>
#   python3 -I test/record/trigger_go.py users                                   # /auth/dev でユーザーを作り、id を揃える
#   python3 -I test/record/trigger_go.py multi test/fixtures/trigger-go.json      # BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT=true の環境で
#   python3 -I test/record/trigger_go.py once  test/fixtures/trigger-once-go.json # false の環境で
# 同じ cases を Worker（WORKER_ROUTES に notifications.send を入れた環境）に送って別のファイルに書けば、Go の記録と比べられる。
#   python3 -I test/record/trigger_go.py cleanup                                 # 記録の後に、試験用のデータ（tr/g93NN のグループ・tr_* のユーザーとそれにぶら下がる行）を消す
# #164（ゲームモード）の本文（game_mode・member_ids・message）のケースは、1ケース1グループ（9320〜9351）。201 になると進行中の通知ができ、同じグループの次のケースが 409 になるため
# wrangler.toml の WRANGLER_ENV が DEV_MODE="true" で、その D1 が D1_DATABASE、Worker の名前が BASE_URL のホストと一致する環境でだけ動く。
# dev・本番（WRANGLER_ENV が dev / production、D1 が begit-db / begit-db-dev）では、一致しても動かない。
# 入れようとする id の行がすでにあれば、何も書かずに止まる。
import datetime
import json
import os
import re
import subprocess
import sys
import tomllib
import urllib.error
import urllib.parse
import urllib.request

BASE_URL = os.environ.get("BASE_URL", "")
WRANGLER_ENV = os.environ.get("WRANGLER_ENV", "")
D1_DATABASE = os.environ.get("D1_DATABASE", "")
BACKEND = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# 試験用の ID は 9301〜9399（グループ・ユーザー・スプリント・チャネル）、通知は 930001〜。setup の前に空いているかを確かめる（check_free）
USERS = [
    (9301, "tr_owner"),
    (9302, "tr_poster"),
    (9303, "tr_viewer"),
    (9304, "tr_outsider"),
    (9305, "tr_legacy"),
]

# #164 の本文のケースのグループ（1ケース1グループ）と、そのうち完成済みにするもの
BODY_GROUPS = range(9320, 9352)
BODY_COMPLETED = {9340, 9341}

# BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT=true で記録するもの
SETUP_MULTI = [
    "INSERT INTO groups (id, repo_full_name, name, owner_user_id) VALUES "
    "(9301, 'tr/g9301', 'tr main', 9301), (9302, 'tr/g9302', 'tr expired sprint', 9301), (9303, 'tr/g9303', 'tr no sprint', 9301), "
    "(9305, 'tr/g9305', 'tr stopped', 9301), (9306, 'tr/g9306', 'tr old active', 9301), (9307, 'tr/g9307', 'A&B <チーム> \"q\"', 9301), "
    "(9308, 'tr/g9308', 'tr legacy', 9301), (9309, 'tr/g9309', 'tr body', 9301), (9310, 'tr/g9310', 'tr other', 9304)",
    "INSERT INTO groups (id, repo_full_name, name, owner_user_id, completed_at) VALUES (9304, 'tr/g9304', 'tr completed', 9301, datetime('now', '-1 days'))",
    "INSERT INTO group_members (group_id, user_id, role) VALUES "
    "(9301, 9301, 'owner'), (9301, 9302, 'poster'), (9301, 9303, 'viewer'), "
    "(9302, 9301, 'owner'), (9303, 9301, 'owner'), (9304, 9301, 'owner'), (9305, 9301, 'owner'), (9306, 9301, 'owner'), "
    "(9307, 9302, 'owner'), (9307, 9301, 'poster'), (9308, 9301, 'owner'), (9308, 9305, 'member'), (9309, 9301, 'owner'), (9310, 9304, 'owner')",
    "INSERT INTO sprints (id, group_id, index_num, started_at, ends_at) VALUES "
    "(9301, 9301, 1, datetime('now', '-3 days'), '2099-01-01 00:00:00'), "
    "(9302, 9302, 0, '2026-01-01 00:00:00', '2026-01-08 00:00:00'), "
    "(9304, 9304, 0, datetime('now', '-3 days'), '2099-01-01 00:00:00'), "
    "(9305, 9305, 0, datetime('now', '-3 days'), '2099-01-01 00:00:00'), "
    "(9306, 9306, 0, datetime('now', '-3 days'), '2099-01-01 00:00:00'), "
    "(9307, 9307, 0, datetime('now', '-3 days'), '2099-01-01 00:00:00'), "
    "(9308, 9308, 0, datetime('now', '-3 days'), '2099-01-01 00:00:00'), "
    "(9309, 9309, 0, datetime('now', '-3 days'), '2099-01-01 00:00:00')",
    "INSERT INTO notifications (id, sprint_id, sent_by, sent_at, stopped_at, ended_at, source) VALUES "
    "(930001, 9305, 9301, datetime('now', '-30 minutes'), datetime('now', '-20 minutes'), NULL, 'manual'), "
    "(930002, 9305, 9301, datetime('now', '-15 minutes'), NULL, datetime('now', '-10 minutes'), 'manual'), "
    "(930003, 9306, 9301, datetime('now', '-2 hours'), NULL, NULL, 'manual')",
    "INSERT INTO notification_channels (id, group_id, platform, display_name, encrypted_webhook_url, enabled, created_by) VALUES "
    "(9301, 9307, 'discord', 'on', 'not-encrypted', 1, 9302), (9302, 9307, 'slack', 'disabled', 'not-encrypted', 0, 9302), "
    "(9303, 9307, 'slack', 'reaction only', 'not-encrypted', 1, 9302), (9304, 9307, 'discord', 'two events', 'not-encrypted', 1, 9302), "
    "(9305, 9301, 'slack', 'other group', 'not-encrypted', 1, 9301)",
    "INSERT INTO notification_channel_subscriptions (channel_id, event_type) VALUES "
    "(9301, 'begit_time'), (9302, 'begit_time'), (9303, 'reaction'), (9304, 'reaction'), (9304, 'begit_time'), (9305, 'reaction')",
    # #164 の本文のケース。メンバーはどのグループも owner 9301・poster 9302・viewer 9303・旧 role member 9305（9304 はメンバーでない）
    "INSERT INTO groups (id, repo_full_name, name, owner_user_id, completed_at) VALUES "
    + ", ".join(f"({g}, 'tr/g{g}', 'tr body {g}', 9301, {'datetime(' + chr(39) + 'now' + chr(39) + ', ' + chr(39) + '-1 days' + chr(39) + ')' if g in BODY_COMPLETED else 'NULL'})" for g in BODY_GROUPS),
    "INSERT INTO group_members (group_id, user_id, role) VALUES "
    + ", ".join(f"({g}, 9301, 'owner'), ({g}, 9302, 'poster'), ({g}, 9303, 'viewer'), ({g}, 9305, 'member')" for g in BODY_GROUPS),
    # 進行中の通知があるグループ（不正な対象＋進行中）
    "INSERT INTO sprints (id, group_id, index_num, started_at, ends_at) VALUES (9310, 9342, 0, datetime('now', '-3 days'), '2099-01-01 00:00:00')",
    "INSERT INTO notifications (id, sprint_id, sent_by, sent_at, source) VALUES (930004, 9310, 9302, datetime('now', '-10 minutes'), 'manual')",
    # 外部通知のチャネルがあるグループ（spotlight）
    "INSERT INTO notification_channels (id, group_id, platform, display_name, encrypted_webhook_url, enabled, created_by) VALUES "
    "(9306, 9343, 'discord', 'spotlight', 'not-encrypted', 1, 9301)",
    "INSERT INTO notification_channel_subscriptions (channel_id, event_type) VALUES (9306, 'begit_time')",
]

# BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT=false で記録するもの（1スプリント1人1回）。
# multi の後に流すので、multi で Go が作ったスプリント（9311〜。201 のケースの数だけ）・通知と id がぶつからないよう、9381〜・930101〜 にしてある
SETUP_ONCE = [
    "INSERT INTO groups (id, repo_full_name, name, owner_user_id) VALUES "
    "(9311, 'tr/g9311', 'tr once', 9301), (9312, 'tr/g9312', 'tr scheduled', 9301), (9313, 'tr/g9313', 'tr previous sprint', 9301)",
    "INSERT INTO group_members (group_id, user_id, role) VALUES "
    "(9311, 9301, 'owner'), (9311, 9302, 'poster'), (9312, 9301, 'owner'), (9313, 9301, 'owner')",
    "INSERT INTO sprints (id, group_id, index_num, started_at, ends_at) VALUES "
    "(9381, 9311, 0, datetime('now', '-3 days'), '2099-01-01 00:00:00'), "
    "(9382, 9312, 0, datetime('now', '-3 days'), '2099-01-01 00:00:00'), "
    "(9383, 9313, 0, '2026-01-01 00:00:00', '2026-01-08 00:00:00'), "
    "(9384, 9313, 1, datetime('now', '-3 days'), '2099-01-01 00:00:00')",
    "INSERT INTO notifications (id, sprint_id, sent_by, sent_at, stopped_at, ended_at, source) VALUES "
    "(930101, 9381, 9301, datetime('now', '-30 minutes'), datetime('now', '-20 minutes'), NULL, 'manual'), "
    "(930102, 9382, 9301, datetime('now', '-30 minutes'), datetime('now', '-20 minutes'), NULL, 'scheduled'), "
    "(930103, 9383, 9301, '2026-01-02 00:00:00', NULL, NULL, 'manual')",
]

NOTIF_ROWS = ("SELECT n.id, n.sprint_id, n.sent_by, n.message, n.sent_at, n.stopped_at, n.ended_at, n.source, n.game_mode, n.audience_user_ids, "
              "(SELECT group_concat(user_id) FROM (SELECT user_id FROM notification_members WHERE notification_id = n.id ORDER BY user_id)) AS members "
              "FROM notifications n JOIN sprints s ON s.id = n.sprint_id WHERE s.group_id = {g} ORDER BY n.id")
SPRINT_ROWS = "SELECT id, group_id, index_num, started_at, ends_at FROM sprints WHERE group_id = {g} ORDER BY id"
JOB_ROWS = ("SELECT j.id, j.event_key, j.channel_id, j.event_type, j.payload_json, j.status, j.attempt_count "
            "FROM notification_delivery_jobs j JOIN notification_channels c ON c.id = j.channel_id WHERE c.group_id = {g} ORDER BY j.channel_id")


def case(name, group, token, *, rows=("notifications",), body=None, headers=None, path=None):
    sql = {"notifications": NOTIF_ROWS, "sprints": SPRINT_ROWS, "jobs": JOB_ROWS}
    return {
        "name": name,
        "method": "POST",
        "path": path or f"/groups/{group}/notifications",
        "token": token,
        "body": body,
        "headers": headers or {},
        "rows": {r: sql[r].format(g=group) for r in rows} if rows else None,
    }


def body_case(name, group, body, *, token, rows=None):
    # 本文のケースはどれも、通知とスプリントの行を見る（400・422 で何も作らないこと、201 で何を作ったか）
    return case(name, group, token, rows=rows or ("notifications", "sprints"), body=body, headers={"Content-Type": "application/json"})


CASES_MULTI = [
    case("認証なし", 9301, None, rows=None),
    case("知らないトークン", 9301, "dev_nobody_9399", rows=None),
    case("グループ ID が数字でない", None, "dev_tr_owner", rows=None, path="/groups/abc/notifications"),
    case("存在しないグループ", 9399, "dev_tr_owner", rows=None),
    case("viewer", 9301, "dev_tr_viewer"),
    case("メンバーでない", 9301, "dev_tr_outsider"),
    case("owner がトリガー", 9301, "dev_tr_owner"),
    case("進行中に poster がもう一度", 9301, "dev_tr_poster"),
    case("進行中に owner がもう一度", 9301, "dev_tr_owner"),
    case("期限切れのスプリントしかない", 9302, "dev_tr_owner", rows=("notifications", "sprints")),
    case("スプリントがない", 9303, "dev_tr_owner", rows=("notifications", "sprints")),
    case("完成済みのグループ", 9304, "dev_tr_owner", rows=("notifications", "sprints")),
    case("止めた・中断した通知の後", 9305, "dev_tr_owner"),
    case("2時間前の通知が残っている", 9306, "dev_tr_owner"),
    case("外部通知のチャネルがある", 9307, "dev_tr_owner", rows=("notifications", "jobs")),
    case("旧 role の member", 9308, "dev_tr_legacy"),
    case("本文が JSON でない（Content-Type も JSON）", 9309, "dev_tr_owner", body="not json", headers={"Content-Type": "application/json"},
         rows=("notifications", "sprints")),
] + [
    # #164 の本文（game_mode・member_ids・message）。グループにスプリントはない（201 なら Go が作る）
    body_case(name, g, body, token=token, rows=rows)
    for name, g, body, token, rows in [
        ("空の本文（Content-Length: 0）", 9320, "", "dev_tr_owner", None),
        ("本文が {}", 9321, "{}", "dev_tr_owner", None),
        ("本文が null", 9322, "null", "dev_tr_owner", None),
        ("member_ids が null", 9323, '{"member_ids":null}', "dev_tr_owner", None),
        ("member_ids が空", 9324, '{"member_ids":[]}', "dev_tr_owner", None),
        ("本文が壊れた JSON", 9325, '{"game_mode":', "dev_tr_owner", None),
        ("本文が配列", 9326, "[]", "dev_tr_owner", None),
        ("game_mode が数字", 9327, '{"game_mode":1}', "dev_tr_owner", None),
        ("member_ids が文字列", 9328, '{"member_ids":"9302"}', "dev_tr_owner", None),
        ("member_ids に小数", 9329, '{"member_ids":[9302.5]}', "dev_tr_owner", None),
        ("message が数字", 9330, '{"message":1}', "dev_tr_owner", None),
        ("iOS と同じ本文（everyone）", 9331, '{"game_mode":"everyone","member_ids":[9301,9302],"message":"やるぞ"}', "dev_tr_owner", None),
        ("spotlight・自分を含まない対象", 9332, '{"game_mode":"spotlight","member_ids":[9302],"message":"見て"}', "dev_tr_owner", None),
        ("game_mode が知らない値", 9333, '{"game_mode":"solo"}', "dev_tr_owner", None),
        ("game_mode が大文字", 9334, '{"game_mode":"SPOTLIGHT"}', "dev_tr_owner", None),
        ("member_ids が重複", 9335, '{"member_ids":[9302,9302,9301]}', "dev_tr_owner", None),
        ("member_ids に viewer", 9336, '{"member_ids":[9302,9303]}', "dev_tr_owner", None),
        ("member_ids にメンバーでない人", 9337, '{"member_ids":[9304]}', "dev_tr_owner", None),
        ("member_ids に旧 role の member", 9338, '{"member_ids":[9305]}', "dev_tr_owner", None),
        ("message が空文字", 9339, '{"message":""}', "dev_tr_owner", None),
        ("不正な game_mode・完成済み", 9340, '{"game_mode":"solo"}', "dev_tr_owner", None),
        ("不正な member_ids・完成済み", 9341, '{"member_ids":[9304]}', "dev_tr_owner", None),
        ("不正な member_ids・進行中", 9342, '{"member_ids":[9304]}', "dev_tr_owner", None),
        ("外部通知のチャネルがある・spotlight", 9343, '{"game_mode":"spotlight","member_ids":[9302]}', "dev_tr_owner", ("notifications", "jobs")),
        ("member_ids に指数の形の数", 9344, '{"member_ids":[9.302e3]}', "dev_tr_owner", None),
        ("member_ids に .0 つきの数", 9345, '{"member_ids":[9302.0]}', "dev_tr_owner", None),
        ("member_ids に int64 を超える数", 9346, '{"member_ids":[99999999999999999999]}', "dev_tr_owner", None),
        ("member_ids に負の数", 9347, '{"member_ids":[-1]}', "dev_tr_owner", None),
        ("game_mode と message が null", 9348, '{"game_mode":null,"message":null}', "dev_tr_owner", None),
        ("知らない項目は無視", 9349, '{"game_mode":"spotlight","extra":1}', "dev_tr_owner", None),
        ("poster が自分だけを対象", 9350, '{"game_mode":"spotlight","member_ids":[9302]}', "dev_tr_poster", None),
        # Go はキーの大文字小文字を区別しない（Worker も gojson.ts で同じに読む）
        ("キーの大文字小文字が違う", 9351, '{"GAME_MODE":"spotlight"}', "dev_tr_owner", None),
    ]
]

CASES_ONCE = [
    case("同じスプリントで2回目（1回目は止めた）", 9311, "dev_tr_owner"),
    case("同じスプリントでも別の人", 9311, "dev_tr_poster"),
    case("定期の通知は数えない", 9312, "dev_tr_owner"),
    case("前のスプリントで送っていても", 9313, "dev_tr_owner"),
]


# 既定の Python-urllib は Cloudflare に 1010 で断られるため
_opener = urllib.request.build_opener()
_opener.addheaders = [("User-Agent", "begit-record/1.0")]
urllib.request.install_opener(_opener)


def d1(sql):
    out = subprocess.run(
        ["npx", "wrangler", "d1", "execute", D1_DATABASE, "--env", WRANGLER_ENV, "--remote", "--json", "--command", sql],
        cwd=BACKEND, capture_output=True, text=True, check=True,
    ).stdout
    return [block.get("results", []) for block in json.loads(out[out.index("["):])]


def send(c):
    # "" は Content-Length: 0 で送る（None は本文なし）
    data = c["body"].encode() if c["body"] is not None else None
    req = urllib.request.Request(BASE_URL + c["path"], data=data, method=c["method"])
    for k, v in c["headers"].items():
        req.add_header(k, v)
    if c["token"] is not None:
        req.add_header("Authorization", "Bearer " + c["token"])
    try:
        with urllib.request.urlopen(req, timeout=60) as res:
            return res.status, json.loads(res.read() or b"null")
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read() or b"null")


TS = re.compile(r"^(\d{4}-\d\d-\d\d)[ T](\d\d:\d\d:\d\d)(Z?)$")


class Normalizer:
    """新しい行の id を出てきた順に <kind:N> にし、今に近い時刻を <now±Mm> にする（test/trigger-go.test.ts と同じ規則）"""

    def __init__(self, now):
        self.now = now
        self.ids = {}

    def new_id(self, kind, value, fixed):
        if value is None or value in fixed:
            return value
        table = self.ids.setdefault(kind, {})
        if value not in table:
            table[value] = f"<{kind}:{len(table) + 1}>"
        return table[value]

    def time(self, value):
        if not isinstance(value, str):
            return value
        m = TS.match(value)
        if not m:
            return value
        at = datetime.datetime.fromisoformat(f"{m.group(1)}T{m.group(2)}+00:00")
        minutes = (at - self.now).total_seconds() / 60
        if abs(minutes) > 8 * 24 * 60:
            return value
        rounded = int(round(minutes / 5) * 5)
        label = "now" if rounded == 0 else f"now{rounded:+d}m"
        return f"<{label}{' Z' if m.group(3) else ''}>"

    def payload(self, text, fixed):
        def key(m):
            return "begit_time:" + str(self.new_id("notification", int(m.group(1)), fixed))
        text = re.sub(r"begit_time:(\d+)", key, text)
        return re.sub(r'"occurred_at":"([^"]+)"', lambda m: '"occurred_at":"' + self.time(m.group(1)) + '"', text)


def request_member_ids(c):
    """ケースの本文で member_ids を指定していれば、その配列（Go はその順に重複を除き、発行者を末尾に足す）"""
    try:
        parsed = json.loads(c["body"] or "null")
    except ValueError:
        return None
    ids = parsed.get("member_ids") if isinstance(parsed, dict) else None
    return ids if isinstance(ids, list) and ids else None


def normalize_audience(text, keep_order):
    # member_ids を指定しないと、Go は対象にできる人を map で回した順に入れるので、順が毎回変わる。そのときだけ並べ替える
    if keep_order or not isinstance(text, str):
        return text
    return json.dumps(sorted(json.loads(text)), separators=(",", ":"))


def normalize_case(n, fixed, body, rows, keep_audience_order=False):
    if isinstance(body, dict) and "id" in body:
        body = dict(body)
        body["id"] = n.new_id("notification", body["id"], fixed["notifications"])
        body["sprint_id"] = n.new_id("sprint", body["sprint_id"], fixed["sprints"])
        body["sent_at"] = n.time(body["sent_at"])
    out_rows = None
    if rows is not None:
        out_rows = {}
        for name, items in rows.items():
            normalized = []
            for row in items:
                row = dict(row)
                if name == "notifications":
                    if row["id"] not in fixed["notifications"]:
                        row["audience_user_ids"] = normalize_audience(row.get("audience_user_ids"), keep_audience_order)
                    row["id"] = n.new_id("notification", row["id"], fixed["notifications"])
                    row["sprint_id"] = n.new_id("sprint", row["sprint_id"], fixed["sprints"])
                elif name == "sprints":
                    row["id"] = n.new_id("sprint", row["id"], fixed["sprints"])
                elif name == "jobs":
                    row["id"] = n.new_id("job", row["id"], set())
                    row["event_key"] = re.sub(r"\d+$", lambda m: str(n.new_id("notification", int(m.group(0)), fixed["notifications"])), row["event_key"])
                    row["payload_json"] = n.payload(row["payload_json"], fixed["notifications"])
                for k in ("sent_at", "stopped_at", "ended_at", "started_at", "ends_at"):
                    if k in row:
                        row[k] = n.time(row[k])
                normalized.append(row)
            out_rows[name] = normalized
    return body, out_rows


def record(kind, out_path):
    setup = SETUP_MULTI if kind == "multi" else SETUP_ONCE
    cases = CASES_MULTI if kind == "multi" else CASES_ONCE
    check_free(kind)
    for sql in setup:
        d1(sql)
    # setup で入れた行（このケースのグループのもの）だけを固定の id にする
    groups = ",".join(sorted({c["path"].split("/")[2] for c in cases if c["path"].split("/")[2].isdigit()}))
    fixed = {
        "notifications": {r["id"] for r in d1(f"SELECT n.id FROM notifications n JOIN sprints s ON s.id = n.sprint_id WHERE s.group_id IN ({groups})")[0]},
        "sprints": {r["id"] for r in d1(f"SELECT id FROM sprints WHERE group_id IN ({groups})")[0]},
    }
    results = []
    for c in cases:
        status, body = send(c)
        rows = None
        if c["rows"]:
            names = list(c["rows"])
            blocks = d1("; ".join(c["rows"][name] for name in names))
            rows = dict(zip(names, blocks))
        now = datetime.datetime.now(datetime.timezone.utc)
        results.append((c, status, body, rows, now))
        print(f"{status} {c['name']}", file=sys.stderr)

    # 時刻は最後のケースの後の now を基準にする（ケース全体で数分以内）
    n = Normalizer(results[-1][4])
    out_cases = []
    for c, status, body, rows, _ in results:
        nbody, nrows = normalize_case(n, fixed, body, rows, keep_audience_order=request_member_ids(c) is not None)
        expect = {"status": status, "body": nbody}
        if nrows is not None:
            expect["rows"] = nrows
        out_cases.append({**c, "expect": expect})

    users = d1(f"SELECT id, github_login AS login, github_id, avatar_url FROM users WHERE id IN ({USER_IDS}) ORDER BY id")[0]
    fixture = {
        "about": "BeGit Time! のトリガー（POST /groups/:id/notifications）の Go の実際の応答。Worker が同じ応答を返すかを test/trigger-go.test.ts で確かめる。"
                 "expect は、users と setup を検証環境に入れ、cases を上から順に Go に送って記録したもので、手で書き換えない。rows は各ケースの後に読んだ行。"
                 "新しくできた通知・スプリント・配信の仕事の id は出てきた順に <notification:N> などに、今に近い時刻は5分単位で <now-30m> などに置き換えてある"
                 "（T と Z の形の時刻は末尾に ' Z'）。",
        "env": {"BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT": "true" if kind == "multi" else "false"},
        "users": users,
        "setup": setup,
        "fixed_ids": {k: sorted(v) for k, v in fixed.items()},
        "cases": out_cases,
        "recorded": {
            "at": datetime.date.today().isoformat(),
            "go": os.environ.get("GO_VERSION", ""),
            "how": "test/record/trigger_go.py で、users を /auth/dev で作って id を揃え、setup を流し、cases を上から順に送って、応答と rows を記録した",
        },
    }
    with open(out_path, "w") as f:
        json.dump(fixture, f, ensure_ascii=False, indent=1)
        f.write("\n")


USER_IDS = ",".join(str(uid) for uid, _ in USERS)
USER_LOGINS = ",".join(f"'{login}'" for _, login in USERS)

# setup が id を指定して入れる行。すでにあれば、ほかの行を上書き・削除しかねないので止める
SETUP_IDS = {
    "users": [("users", "id", 9301, 9305)],
    "multi": [("groups", "id", 9301, 9310), ("groups", "id", 9320, 9351), ("groups", "id", 9399, 9399), ("sprints", "id", 9301, 9310),
              ("notifications", "id", 930001, 930004), ("notification_channels", "id", 9301, 9306)],
    "once": [("groups", "id", 9311, 9313), ("sprints", "id", 9381, 9384), ("notifications", "id", 930101, 930103)],
}


def check_free(kind):
    for table, column, lo, hi in SETUP_IDS[kind]:
        # users だけは、前に作った試験用のユーザー（/auth/dev は負の github_id で作る）なら作り直してよい
        mine = f" AND NOT (github_login IN ({USER_LOGINS}) AND github_id < 0)" if table == "users" else ""
        rows = d1(f"SELECT {column} FROM {table} WHERE {column} BETWEEN {lo} AND {hi}{mine} LIMIT 5")[0]
        if rows:
            sys.exit(f"{table} の {lo}〜{hi} に行がある（{[r[column] for r in rows]}）。cleanup するか、空いている id に変えてから流す")


def create_users():
    check_free("users")
    for uid, login in USERS:
        req = urllib.request.Request(BASE_URL + "/auth/dev", data=json.dumps({"login": login}).encode(), method="POST")
        req.add_header("Content-Type", "application/json")
        with urllib.request.urlopen(req, timeout=60) as res:
            assert res.status == 200, res.status
        d1(f"UPDATE users SET id = {uid} WHERE github_login = '{login}' AND github_id < 0 AND id <> {uid}")
    print(d1(f"SELECT id, github_login, github_id FROM users WHERE id IN ({USER_IDS})")[0], file=sys.stderr)


# 試験用の行は、id の範囲ではなく印で選ぶ（setup で id を指定して入れたため、自動の続き番号がこの範囲に進み、後から本物の行が入りうる）。
# グループは repo_full_name が tr/g93NN、ユーザーは USERS の login で github_id が負（/auth/dev で作ったもの）
GROUPS = "SELECT id FROM groups WHERE repo_full_name GLOB 'tr/g93[0-9][0-9]'"
SPRINTS = f"SELECT id FROM sprints WHERE group_id IN ({GROUPS})"
NOTIFICATIONS = f"SELECT id FROM notifications WHERE sprint_id IN ({SPRINTS})"
POSTS = f"SELECT id FROM posts WHERE group_id IN ({GROUPS})"
CHANNELS = f"SELECT id FROM notification_channels WHERE group_id IN ({GROUPS})"
TEST_USERS = f"SELECT id FROM users WHERE github_login IN ({USER_LOGINS}) AND github_id < 0"

# 外部キーの向きの逆順に消す。残すと Cron が 2099 年までのスプリントや仮の webhook のチャネルを拾い続ける。
# Cron が動く環境では、未投稿（missed）の投稿・ポイント・終了の通知の記録もできるので、それも消す。
# 0016・0017 のトリガーが作る行（通知の結果・同期のカーソル）も消す
CLEANUP = [
    f"DELETE FROM notification_delivery_jobs WHERE channel_id IN ({CHANNELS})",
    f"DELETE FROM notification_channel_subscriptions WHERE channel_id IN ({CHANNELS})",
    f"DELETE FROM notification_channels WHERE group_id IN ({GROUPS})",
    f"DELETE FROM reactions WHERE post_id IN ({POSTS})",
    f"DELETE FROM comments WHERE post_id IN ({POSTS})",
    f"DELETE FROM photos WHERE post_id IN ({POSTS})",
    f"DELETE FROM posts WHERE group_id IN ({GROUPS})",
    f"DELETE FROM point_awards WHERE group_id IN ({GROUPS})",
    f"DELETE FROM final_result_members WHERE group_id IN ({GROUPS})",
    f"DELETE FROM notification_deliveries WHERE (kind = 'challenge_end' AND ref_id IN ({NOTIFICATIONS})) "
    f"OR (kind LIKE 'sprint_%' AND ref_id IN ({SPRINTS}))",
    f"DELETE FROM notification_game_results WHERE notification_id IN ({NOTIFICATIONS})",
    f"DELETE FROM notification_members WHERE notification_id IN ({NOTIFICATIONS})",
    f"DELETE FROM notifications WHERE sprint_id IN ({SPRINTS})",
    f"DELETE FROM sprints WHERE group_id IN ({GROUPS})",
    f"DELETE FROM group_members WHERE group_id IN ({GROUPS})",
    # 0017 のトリガーが、上の行の追加・削除のたびに書く（削除でも書くので、groups の直前に消す）
    f"DELETE FROM repository_changes WHERE group_id IN ({GROUPS})",
    f"DELETE FROM groups WHERE id IN ({GROUPS})",
    f"DELETE FROM users WHERE id IN ({TEST_USERS})",
]
# 自動の続き番号を、残った行の最大値に戻す（setup の id で進んだ分）
RESET_SEQUENCES = [
    f"UPDATE sqlite_sequence SET seq = (SELECT COALESCE(MAX({'sequence' if table == 'repository_changes' else 'id'}), 0) FROM {table}) WHERE name = '{table}'"
    for table in ("users", "groups", "sprints", "notifications", "notification_channels", "notification_delivery_jobs", "repository_changes")
]


def cleanup():
    for sql in CLEANUP + RESET_SEQUENCES:
        d1(sql)


# 試験用の行を入れたり users の id を書き換えたりするので、wrangler.toml で計測用と分かる環境でだけ動かす
PROTECTED_ENVS = {"dev", "production", "prod"}
PROTECTED_DATABASES = {"begit-db", "begit-db-dev"}


def check_environment():
    if WRANGLER_ENV in PROTECTED_ENVS or D1_DATABASE in PROTECTED_DATABASES:
        sys.exit(f"{WRANGLER_ENV} / {D1_DATABASE} では動かさない（計測用の環境だけ）")
    with open(os.path.join(BACKEND, "wrangler.toml"), "rb") as f:
        env = tomllib.load(f).get("env", {}).get(WRANGLER_ENV)
    if env is None:
        sys.exit(f"wrangler.toml に env.{WRANGLER_ENV} がない")
    if env.get("vars", {}).get("DEV_MODE") != "true":
        sys.exit(f"env.{WRANGLER_ENV} は DEV_MODE=\"true\" でない（/auth/dev が使えない）")
    databases = [db.get("database_name") for db in env.get("d1_databases", [])]
    if D1_DATABASE not in databases:
        sys.exit(f"env.{WRANGLER_ENV} の D1 は {databases} で、D1_DATABASE={D1_DATABASE} と違う")
    host = urllib.parse.urlparse(BASE_URL).hostname or ""
    if not host.startswith(f"{env.get('name')}."):
        sys.exit(f"BASE_URL のホスト {host} が env.{WRANGLER_ENV} の Worker（{env.get('name')}）でない")


USAGE = "usage: trigger_go.py users | multi <out.json> | once <out.json> | cleanup"

if __name__ == "__main__":
    if not (BASE_URL and WRANGLER_ENV and D1_DATABASE):
        sys.exit("BASE_URL, WRANGLER_ENV, D1_DATABASE を設定してください")
    check_environment()
    command = sys.argv[1] if len(sys.argv) > 1 else ""
    if command == "users":
        create_users()
    elif command == "cleanup":
        cleanup()
    elif command in ("multi", "once") and len(sys.argv) == 3:
        record(command, sys.argv[2])
    else:
        sys.exit(USAGE)
