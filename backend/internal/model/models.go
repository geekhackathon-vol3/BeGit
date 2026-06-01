package model

import "time"

// User は GitHub アカウントの 1:1 マッピング
type User struct {
	ID                   int64
	GitHubID             int64
	GitHubLogin          string
	GitHubName           string
	AvatarURL            string
	EncryptedAccessToken string
	CreatedAt            time.Time
}

// Group は GitHub リポジトリに紐づくチーム単位
type Group struct {
	ID                 int64
	RepoFullName       string
	Name               string
	AvatarURL          string
	OwnerUserID        int64
	SprintDurationDays int
	CreatedAt          time.Time
}

// GroupMember は Group と User の多対多（role: owner | member）
type GroupMember struct {
	GroupID    int64
	UserID     int64
	Login      string
	AvatarURL  string
	Role       string
	AutoJoined bool
}

// Sprint はグループのアクティブな期間
type Sprint struct {
	ID        int64
	GroupID   int64
	IndexNum  int
	StartedAt time.Time
	EndsAt    time.Time
}

// Notification は BeGit Time 通知。1スプリント1ユーザー1回制約 UNIQUE(sprint_id, sent_by)
type Notification struct {
	ID       int64
	SprintID int64
	SentBy   int64
	Message  string
	SentAt   time.Time
}

// Post は投稿。notification_id でどの通知に応答したかを記録
type Post struct {
	ID                  int64
	NotificationID      *int64
	UserID              int64
	GroupID             int64
	PostType            string
	Body                *string
	RepoFullName        *string
	BranchName          *string
	CommitCount         int
	Additions           int
	Deletions           int
	LatestCommitMessage *string
	Status              *string
	CreatedAt           time.Time
}

// PostFeed はフィード表示用の投稿（ぼかし制御フラグ付き）
type PostFeed struct {
	Post
	Login     string
	AvatarURL string
	Blurred   bool
}

// Reaction は投稿へのリアクション。UNIQUE(post_id, user_id, reaction_type)。
// Login / AvatarURL は users テーブルとの JOIN で付与する一覧表示用フィールド。
type Reaction struct {
	ID           int64
	PostID       int64
	UserID       int64
	ReactionType string
	Login        string
	AvatarURL    string
}
