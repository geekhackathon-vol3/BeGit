# BeGit データベース ER 図

**バージョン:** 1.0.0  
**DB:** Cloudflare D1（SQLite 互換）  
**スキーマ:** [backend/migrations/0001_initial.sql](../backend/migrations/0001_initial.sql)  
**関連:** [spec.md セクション6](../spec.md#6-データモデル) / [.kiro/steering/database.md](../.kiro/steering/database.md)

## 閲覧方法

| 方法 | ファイル | 手順 |
|------|---------|------|
| **ブラウザ（推奨）** | [database-er.html](database-er.html) | ファイルをダブルクリック、または `open docs/database-er.html` |
| **Mermaid ソース** | [database-er.mermaid](database-er.mermaid) | Cursor / VS Code の Mermaid プレビュー、または [Mermaid Live Editor](https://mermaid.live) に貼り付け |
| **GitHub** | この Markdown | GitHub 上では Mermaid が自動レンダリングされる |

---

## 全体 ER 図

```mermaid
erDiagram
    users {
        TEXT id PK
        INTEGER github_id UK
        TEXT github_login UK
        TEXT username
        TEXT access_token_encrypted
    }

    fcm_tokens {
        TEXT id PK
        TEXT user_id FK
        TEXT registration_token UK
    }

    groups {
        TEXT id PK
        TEXT name
        TEXT repo_full_name
        INTEGER sprint_duration_days
        TEXT created_by FK
    }

    group_members {
        TEXT group_id PK_FK
        TEXT user_id PK_FK
        TEXT role
        TEXT left_at
    }

    sprints {
        TEXT id PK
        TEXT group_id FK
        INTEGER index_num
        TEXT started_at
        TEXT ends_at
    }

    be_time_notifications {
        TEXT id PK
        TEXT group_id FK
        TEXT sprint_id FK
        TEXT sent_by FK
        TEXT message
        TEXT sent_at
    }

    posts {
        TEXT id PK
        TEXT notification_id FK
        TEXT user_id FK
        TEXT group_id FK
        TEXT status
        TEXT privacy_level
    }

    tags {
        TEXT id PK
        TEXT name UK
    }

    post_tags {
        TEXT post_id PK_FK
        TEXT tag_id PK_FK
    }

    photos {
        TEXT id PK
        TEXT post_id FK
        TEXT r2_key
        TEXT type
    }

    reactions {
        TEXT id PK
        TEXT post_id FK
        TEXT user_id FK
        TEXT type
    }

    comments {
        TEXT id PK
        TEXT post_id FK
        TEXT user_id FK
        TEXT body
    }

    github_webhook_deliveries {
        TEXT delivery_id PK
        TEXT event_type
    }

    users ||--o{ fcm_tokens : has
    users ||--o{ groups : creates
    users ||--o{ group_members : joins
    groups ||--o{ group_members : has
    groups ||--o{ sprints : has
    groups ||--o{ be_time_notifications : has
    groups ||--o{ posts : contains
    sprints ||--o{ be_time_notifications : contains
    users ||--o{ be_time_notifications : sends
    be_time_notifications ||--o{ posts : triggers
    users ||--o{ posts : writes
    posts ||--o{ photos : has
    posts ||--o{ reactions : receives
    users ||--o{ reactions : gives
    posts ||--o{ comments : has
    users ||--o{ comments : writes
    posts ||--o{ post_tags : tagged
    tags ||--o{ post_tags : used_in
```

> `github_webhook_deliveries` は他テーブルと FK を持たない独立テーブル（Webhook 冪等性用）。

---

## ドメイン別構造

### 認証・デバイス

```mermaid
flowchart LR
    User[users] -->|1:N| FCM[fcm_tokens]
    User -->|github_login| GitHub[GitHub Collaborators]
```

- `users.github_login` で GitHub コラボレーター自動参加をマッチング
- `users.access_token_encrypted` はサーバー側で暗号化保存

### グループ・スプリント（中核）

```mermaid
flowchart TB
    User[users] -->|creates| Group[groups]
    User -->|N:M via group_members| Group
    Group -->|1:N| Sprint[sprints]
    Sprint -->|1:N| Notif[be_time_notifications]
    Notif -->|1:N| Post[posts]
    Group -->|1:N| Post
    User -->|writes| Post
    User -->|sends| Notif
```

- MVP では **Group : GitHub Repo = 1:1**（`groups.repo_full_name` に直接保持）
- `group_repositories` 中間テーブルは使用しない

### 投稿・ソーシャル

```mermaid
flowchart TB
    Post[posts] -->|1:N| Photo[photos]
    Post -->|1:N| Reaction[reactions]
    Post -->|1:N| Comment[comments]
    Post -->|N:M via post_tags| Tag[tags]
    User[users] --> Reaction
    User --> Comment
```

---

## データの流れ（時系列）

```mermaid
sequenceDiagram
    participant App
    participant API
    participant D1

    App->>API: POST /groups
    API->>D1: INSERT groups, group_members, sprints

    App->>API: POST /groups/:id/notifications
    API->>D1: SELECT sprint WHERE group_id AND ends_at > now
    API->>D1: INSERT be_time_notifications
    Note over D1: UNIQUE(sprint_id, sent_by)

    App->>API: POST /posts
    API->>D1: SELECT sent_at FROM be_time_notifications
    API->>API: status = on_time or late
    API->>D1: INSERT posts
    Note over D1: UNIQUE(notification_id, user_id)

    Note over API,D1: Sprint 終了 Cron
    API->>D1: INSERT posts status=missed for members without post
```

| 段階 | テーブル | 内容 |
|------|---------|------|
| 1 | `groups`, `group_members`, `sprints` | グループ作成 + 初回スプリント開始 |
| 2 | `be_time_notifications` | メンバーが BeGit Time 通知を発行 |
| 3 | `posts` | 各メンバーが開発状況を投稿 |
| 4 | `posts` (batch) | 未投稿者に `missed` を upsert |

---

## ビジネス制約

| 制約 | テーブル | 意味 |
|------|---------|------|
| `UNIQUE(sprint_id, sent_by)` | `be_time_notifications` | 1 スプリント・1 人・1 回だけ通知発行 |
| `UNIQUE(notification_id, user_id)` | `posts` | 1 通知に対し 1 ユーザー 1 投稿 |
| `UNIQUE(post_id, user_id, type)` | `reactions` | リアクション種別ごとに 1 つ |
| `PK(group_id, user_id)` | `group_members` | グループ所属の一意性 |
| `left_at IS NULL` | `group_members` | 在籍中メンバーの判定 |

### Post.status 算出

| status | 条件 |
|--------|------|
| `on_time` | `created_at` ≤ `sent_at` + 1 時間 |
| `late` | `created_at` > `sent_at` + 1 時間 |
| `missed` | 投稿なし — スプリント終了バッチで upsert |

---

## テーブル一覧（13 テーブル）

| # | テーブル | 役割 |
|---|---------|------|
| 1 | `users` | GitHub 連携ユーザー |
| 2 | `fcm_tokens` | Push 通知デバイストークン |
| 3 | `groups` | チーム（1 repo 紐付け） |
| 4 | `group_members` | グループ所属（User ↔ Group N:M） |
| 5 | `sprints` | スプリント期間 |
| 6 | `be_time_notifications` | BeGit Time 通知発行（FCM Push とは別概念） |
| 7 | `posts` | 開発状況投稿 |
| 8 | `tags` | 技術タグマスタ |
| 9 | `post_tags` | 投稿 ↔ タグ（N:M 中間） |
| 10 | `photos` | 添付写真（R2 オブジェクトキー） |
| 11 | `reactions` | リアクション |
| 12 | `comments` | コメント |
| 13 | `github_webhook_deliveries` | Webhook 冪等性 |

---

## リレーションシップ一覧

```
User 1──N FCMToken
User 1──N Group (created_by)
User N──N Group (via group_members)
Group 1──N Sprint
Sprint 1──N BeTimeNotification
BeTimeNotification 1──N Post
User 1──N Post
Group 1──N Post
Post 1──N Photo / Reaction / Comment
Post N──N Tag (via post_tags)
```

---

## 将来拡張（MVP 外）

| 機能 | 追加予定 |
|------|---------|
| フォロワー限定公開 | `follows` テーブル + `privacy_level = followers` |
| 複数リポジトリ紐付け | `group_repositories` 中間テーブル |
