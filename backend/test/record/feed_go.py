#!/usr/bin/env python3
# フィード取得（GET /groups/:id/posts）の Go の応答を記録し、test/fixtures/feed-go.json の cases を作り直す。
# 記録したものは test/feed-go.test.ts が Worker の応答と比べる。seed は fixture の seed をそのまま使う（seed は手で直してよい。expect は手で直さない）。
#
# 使い方（DEV_MODE=true で、計測用のユーザー user001〜user005（id 2〜6）がいる環境に対して）
#   export BASE_URL=https://<計測用の Worker>  WRANGLER_ENV=<wrangler.toml の env 名>  D1_DATABASE=<D1 の名前>  CLOUDFLARE_ACCOUNT_ID=<アカウント ID>
#   python3 -I test/record/feed_go.py seed                            # fixture の seed を D1 に入れる
#   python3 -I test/record/feed_go.py record test/fixtures/feed-go.json # WORKER_ROUTES に posts.list を入れていない環境で。cases と expect を書き直す
#   python3 -I test/record/feed_go.py compare test/fixtures/feed-go.json # WORKER_ROUTES に posts.list を入れた環境で。同じ cases を送って expect と比べる
#   python3 -I test/record/feed_go.py cleanup                         # seed の行を消し、自動の続き番号を戻す
# wrangler.toml の WRANGLER_ENV が DEV_MODE="true" で、その D1 が D1_DATABASE、Worker の名前が BASE_URL のホストと一致する環境でだけ動く。
# dev・本番（WRANGLER_ENV が dev / production、D1 が begit-db / begit-db-dev）では、一致しても動かない。
import base64
import datetime
import json
import os
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

# 外部キーの向きに合わせた順（test/go-fixture.ts の TABLES と同じ）
TABLES = ["users", "groups", "group_members", "sprints", "notifications", "posts", "photos", "reactions"]
# seed の id の範囲（plan.md の PR② の範囲）。seed・cleanup はこの範囲だけを触る
ID_RANGES = {
    "users": (9101, 9199), "groups": (9101, 9199), "sprints": (9101, 9199), "notifications": (9101000, 9101999),
    "posts": (910000, 919999), "photos": (9101000, 9101999), "reactions": (9101000, 9101999),
}
CALLER_IDS = (2, 3, 4, 5, 6)


def cursor(value):
    """Go と同じく、JSON を base64url（パディングなし）にする"""
    text = value if isinstance(value, str) else json.dumps(value, ensure_ascii=False, separators=(",", ":"))
    return base64.urlsafe_b64encode(text.encode()).decode().rstrip("=")


def std_cursor(value):
    """base64 の標準の形（+ / とパディングあり）。Go の RawURLEncoding では読めない"""
    return base64.b64encode(json.dumps(value, separators=(",", ":")).encode()).decode()


def case(name, path, token="dev_user001", *, cursor_from=None):
    # cursor_from があれば、path の {cursor} をそのケースの応答の next_cursor に置き換える
    c = {"name": name, "method": "GET", "path": path, "token": token, "headers": {}}
    if cursor_from:
        c["cursor_from"] = cursor_from
    return c


AT = "2026-10-20T10:00:00Z"
CASES = [
    case("owner が取る", "/groups/9101/posts"),
    case("poster が取る", "/groups/9101/posts", "dev_user002"),
    case("旧 role の member が取る", "/groups/9101/posts", "dev_user003"),
    case("viewer も取れる", "/groups/9101/posts", "dev_user005"),
    case("投稿がないグループ", "/groups/9103/posts"),
    case("投稿が100件を超えるグループ", "/groups/9102/posts"),
    case("メンバーでない", "/groups/9101/posts", "dev_user004"),
    case("存在しないグループ", "/groups/9199/posts"),
    case("グループ ID が整数でない", "/groups/abc/posts"),
    case("認証なし", "/groups/9101/posts", None),
    case("知らないトークン", "/groups/9101/posts", "dev_nobody"),
    # ページ送り（グループ 9101 は新しい順に 915006, 915003, 915002, 915001, 915007, 915008, 915009, 915011, 時刻が読めない 915010）
    case("limit=1（小数秒の投稿のカーソル）", "/groups/9101/posts?limit=1"),
    case("limit=1 の次のページ", "/groups/9101/posts?limit=1&cursor={cursor}", cursor_from="limit=1（小数秒の投稿のカーソル）"),
    case("limit=2", "/groups/9101/posts?limit=2"),
    case("limit=2 の2ページ目（同じ時刻の途中から）", "/groups/9101/posts?limit=2&cursor={cursor}", cursor_from="limit=2"),
    case("limit=2 の3ページ目", "/groups/9101/posts?limit=2&cursor={cursor}", cursor_from="limit=2 の2ページ目（同じ時刻の途中から）"),
    case("limit=2 の4ページ目（時刻が読めない投稿は出ない）", "/groups/9101/posts?limit=2&cursor={cursor}", cursor_from="limit=2 の3ページ目"),
    case("limit=8（時刻が読めない投稿だけ残る）", "/groups/9101/posts?limit=8"),
    case("limit=8 の次のページ", "/groups/9101/posts?limit=8&cursor={cursor}", cursor_from="limit=8（時刻が読めない投稿だけ残る）"),
    case("limit=9（ちょうど全部）", "/groups/9101/posts?limit=9"),
    case("viewer が limit=2 の2ページ目", "/groups/9101/posts?limit=2&cursor={cursor}", "dev_user005", cursor_from="limit=2"),
    # 100件を超えるグループ（新しい順に 915012, 915200, 915199, …, 915100）
    case("limit=51 は 50 件", "/groups/9102/posts?limit=51"),
    case("50 件ずつの2ページ目", "/groups/9102/posts?limit=50&cursor={cursor}", cursor_from="limit=51 は 50 件"),
    case("50 件ずつの3ページ目（最後）", "/groups/9102/posts?limit=50&cursor={cursor}", cursor_from="50 件ずつの2ページ目"),
    case("limit が空なら 20 件", "/groups/9102/posts?limit="),
    case("limit=+3", "/groups/9101/posts?limit=%2B3"),
    case("limit=007", "/groups/9101/posts?limit=007"),
    case("limit が2つ（最初を使う）", "/groups/9101/posts?limit=1&limit=abc"),
    case("limit が int64 に収まる大きな数", "/groups/9102/posts?limit=9223372036854775807"),
    case("limit=0", "/groups/9101/posts?limit=0"),
    case("limit=-1", "/groups/9101/posts?limit=-1"),
    case("limit=abc", "/groups/9101/posts?limit=abc"),
    case("limit=1.5", "/groups/9101/posts?limit=1.5"),
    case("limit の前に空白", "/groups/9101/posts?limit=%203"),
    case("limit が int64 を超える", "/groups/9101/posts?limit=9223372036854775808"),
    case("limit と cursor の両方が不正", "/groups/9101/posts?limit=0&cursor=!!!"),
    case("メンバーでなければ limit が不正でも 403", "/groups/9101/posts?limit=0", "dev_user004"),
    # カーソル
    case("cursor が空なら最初のページ", "/groups/9101/posts?limit=2&cursor="),
    case("cursor が base64 でない", "/groups/9101/posts?cursor=!!!"),
    case("cursor にパディング", "/groups/9101/posts?cursor=" + urllib.parse.quote(cursor({"created_at": AT, "id": 915003}) + "==")),
    case("cursor が base64 の標準の形", "/groups/9101/posts?cursor=" + urllib.parse.quote(std_cursor({"created_at": AT, "id": 915003, "x": "??>>"}))),
    case("cursor の長さが 4n+1", "/groups/9101/posts?cursor=abcde"),
    case("cursor が JSON でない", "/groups/9101/posts?cursor=" + cursor("not json")),
    case("cursor が配列", "/groups/9101/posts?cursor=" + cursor([])),
    case("cursor が null", "/groups/9101/posts?cursor=" + cursor("null")),
    case("cursor に created_at がない", "/groups/9101/posts?cursor=" + cursor({"id": 915003})),
    case("cursor の created_at が SQLite の形", "/groups/9101/posts?cursor=" + cursor({"created_at": "2026-10-20 10:00:00", "id": 915003})),
    case("cursor の created_at が数", "/groups/9101/posts?cursor=" + cursor({"created_at": 1, "id": 915003})),
    case("cursor の id が文字列", "/groups/9101/posts?cursor=" + cursor({"created_at": AT, "id": "915003"})),
    case("cursor の id が小数", "/groups/9101/posts?cursor=" + cursor({"created_at": AT, "id": 915003.5})),
    case("cursor の id がない（0 として扱う）", "/groups/9101/posts?cursor=" + cursor({"created_at": AT})),
    case("cursor の id が null", "/groups/9101/posts?cursor=" + cursor({"created_at": AT, "id": None})),
    case("cursor のタイムゾーンが +09:00", "/groups/9101/posts?cursor=" + cursor({"created_at": "2026-10-20T19:00:00+09:00", "id": 915003})),
    case("cursor の時刻に小数（秒までで比べる）", "/groups/9101/posts?cursor=" + cursor({"created_at": "2026-10-20T10:00:00.999Z", "id": 915002})),
    case("cursor のキーが大文字", "/groups/9101/posts?cursor=" + cursor({"CREATED_AT": AT, "Id": 915002})),
    case("cursor に知らないキー", "/groups/9101/posts?cursor=" + cursor({"created_at": AT, "id": 915003, "extra": [1, {"a": None}]})),
    case("cursor がゼロ時刻なら最初のページ", "/groups/9101/posts?limit=2&cursor=" + cursor({"created_at": "0001-01-01T00:00:00Z", "id": 915003})),
    case("cursor が一番古い投稿より前", "/groups/9101/posts?cursor=" + cursor({"created_at": "2000-01-01T00:00:00Z", "id": 1})),
    case("投稿がないグループに cursor", "/groups/9103/posts?cursor=" + cursor({"created_at": AT, "id": 915003})),
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
    req = urllib.request.Request(BASE_URL + c["path"], method=c["method"])
    for k, v in c["headers"].items():
        req.add_header(k, v)
    if c["token"] is not None:
        req.add_header("Authorization", "Bearer " + c["token"])
    try:
        with urllib.request.urlopen(req, timeout=60) as res:
            return res.status, json.loads(res.read() or b"null")
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read() or b"null")


def run_cases(cases):
    """cases を上から順に送る。cursor_from は、前のケースの応答の next_cursor で path を埋める"""
    responses, out = {}, []
    for c in cases:
        c = dict(c)
        source = c.pop("cursor_from", None)
        if source:
            next_cursor = responses[source][1].get("next_cursor")
            if not next_cursor:
                sys.exit(f"{c['name']}: {source} の応答に next_cursor がない")
            c["path"] = c["path"].replace("{cursor}", next_cursor)
        status, body = send(c)
        responses[c["name"]] = (status, body)
        print(f"{status} {c['name']}", file=sys.stderr)
        out.append((c, status, body))
    return out


def sql_value(value):
    if value is None:
        return "NULL"
    if isinstance(value, (int, float)):
        return str(value)
    return "'" + str(value).replace("'", "''") + "'"


def check_free():
    for table, (lo, hi) in ID_RANGES.items():
        rows = d1(f"SELECT id FROM {table} WHERE id BETWEEN {lo} AND {hi} LIMIT 5")[0]
        if rows:
            sys.exit(f"{table} の {lo}〜{hi} に行がある（{[r['id'] for r in rows]}）。cleanup してから流す")
    callers = d1(f"SELECT id, github_login FROM users WHERE id IN ({','.join(map(str, CALLER_IDS))}) ORDER BY id")[0]
    if [r["github_login"] for r in callers] != [f"user{n:03d}" for n in range(1, 6)]:
        sys.exit(f"id 2〜6 が user001〜user005 でない: {callers}")


def seed(fixture):
    check_free()
    for table in TABLES:
        rows = fixture["seed"].get(table, [])
        if not rows:
            continue
        lo, hi = ID_RANGES.get(table, (None, None))
        for row in rows:
            if lo is not None and "id" in row and not lo <= row["id"] <= hi:
                sys.exit(f"seed の {table} の id {row['id']} が {lo}〜{hi} の外")
        columns = list(dict.fromkeys(c for row in rows for c in row))
        values = ", ".join("(" + ", ".join(sql_value(row.get(c)) for c in columns) + ")" for row in rows)
        d1(f"INSERT INTO {table} ({', '.join(columns)}) VALUES {values}")


def callers():
    rows = d1(f"SELECT id, github_login AS login, avatar_url FROM users WHERE id IN ({','.join(map(str, CALLER_IDS))}) ORDER BY id")[0]
    return [{"id": r["id"], "login": r["login"], "avatar_url": r["avatar_url"]} for r in rows]


def dump(fixture, path):
    """元の fixture と同じく、配列の要素を1行ずつにする"""
    line = lambda v: json.dumps(v, ensure_ascii=False)
    rows = lambda items, indent: (",\n").join(indent + line(item) for item in items)
    seed_text = ",\n".join(f'  "{t}": [\n{rows(fixture["seed"][t], "   ")}\n  ]' for t in fixture["seed"])
    text = (
        "{\n"
        f' "about": {line(fixture["about"])},\n'
        f' "callers": [\n{rows(fixture["callers"], "  ")}\n ],\n'
        f' "cases": [\n{rows(fixture["cases"], "  ")}\n ],\n'
        f' "recorded": {line(fixture["recorded"])},\n'
        f' "seed": {{\n{seed_text}\n }}\n'
        "}\n"
    )
    json.loads(text)
    with open(path, "w") as f:
        f.write(text)


def record(path):
    with open(path) as f:
        fixture = json.load(f)
    results = run_cases(CASES)
    fixture["callers"] = callers()
    fixture["cases"] = [{**c, "expect": {"status": status, "body": body}} for c, status, body in results]
    fixture["recorded"] = {
        "at": datetime.date.today().isoformat(),
        "go": os.environ.get("GO_VERSION", ""),
        "d1_migration": os.environ.get("D1_MIGRATION", ""),
        "r2": "認証情報なし（Go はスタブの R2 を使う）",
        "how": "test/record/feed_go.py で seed を入れ、cases を上から順に送り、応答を記録した。cursor を含む path は、前のケースの next_cursor で埋めたもの",
    }
    dump(fixture, path)


def compare(path):
    with open(path) as f:
        fixture = json.load(f)
    failed = 0
    for c in fixture["cases"]:
        status, body = send(c)
        if {"status": status, "body": body} != c["expect"]:
            failed += 1
            print(f"DIFF {c['name']}: {status} {json.dumps(body, ensure_ascii=False)[:300]}", file=sys.stderr)
    print(f"{len(fixture['cases']) - failed}/{len(fixture['cases'])} 一致", file=sys.stderr)
    sys.exit(1 if failed else 0)


def cleanup():
    for table in reversed(TABLES):
        if table == "group_members":
            # group_members には id がない。seed のグループの行と、seed のユーザーの行を消す
            (glo, ghi), (ulo, uhi) = ID_RANGES["groups"], ID_RANGES["users"]
            d1(f"DELETE FROM group_members WHERE group_id BETWEEN {glo} AND {ghi} OR user_id BETWEEN {ulo} AND {uhi}")
            continue
        lo, hi = ID_RANGES[table]
        if table == "notifications":
            # 0015 のトリガーが、通知を入れたときの対象者（notification_members）と、投稿のポイント（point_awards）を作っている
            glo, ghi = ID_RANGES["groups"]
            d1(f"DELETE FROM point_awards WHERE notification_id BETWEEN {lo} AND {hi} OR group_id BETWEEN {glo} AND {ghi}")
            d1(f"DELETE FROM notification_members WHERE notification_id BETWEEN {lo} AND {hi}")
        if table == "groups":
            # 0017 のトリガーが、seed を入れたとき・消したときに repository_changes（groups への外部キー）に行を足している
            d1(f"DELETE FROM repository_changes WHERE group_id BETWEEN {lo} AND {hi}")
        d1(f"DELETE FROM {table} WHERE id BETWEEN {lo} AND {hi}")
    # seed の id で進んだ自動の続き番号を、残った行の最大値に戻す
    for table in ID_RANGES:
        d1(f"UPDATE sqlite_sequence SET seq = (SELECT COALESCE(MAX(id), 0) FROM {table}) WHERE name = '{table}'")
    d1("UPDATE sqlite_sequence SET seq = (SELECT COALESCE(MAX(sequence), 0) FROM repository_changes) WHERE name = 'repository_changes'")


# 試験用の行を入れるので、wrangler.toml で計測用と分かる環境でだけ動かす
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
        sys.exit(f"env.{WRANGLER_ENV} は DEV_MODE=\"true\" でない")
    databases = [db.get("database_name") for db in env.get("d1_databases", [])]
    if D1_DATABASE not in databases:
        sys.exit(f"env.{WRANGLER_ENV} の D1 は {databases} で、D1_DATABASE={D1_DATABASE} と違う")
    host = urllib.parse.urlparse(BASE_URL).hostname or ""
    if not host.startswith(f"{env.get('name')}."):
        sys.exit(f"BASE_URL のホスト {host} が env.{WRANGLER_ENV} の Worker（{env.get('name')}）でない")


USAGE = "usage: feed_go.py seed | record <fixture.json> | compare <fixture.json> | cleanup"

if __name__ == "__main__":
    if not (BASE_URL and WRANGLER_ENV and D1_DATABASE):
        sys.exit("BASE_URL, WRANGLER_ENV, D1_DATABASE を設定してください")
    check_environment()
    command = sys.argv[1] if len(sys.argv) > 1 else ""
    fixture_path = os.path.join(BACKEND, "test", "fixtures", "feed-go.json")
    if command == "seed":
        with open(fixture_path) as f:
            seed(json.load(f))
    elif command == "cleanup":
        cleanup()
    elif command in ("record", "compare") and len(sys.argv) == 3:
        (record if command == "record" else compare)(sys.argv[2])
    else:
        sys.exit(USAGE)
