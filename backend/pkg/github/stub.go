package github

import "context"

// stubClient は DEV_MODE 時に使用する Client のスタブ実装。
// 実際の GitHub API を呼ばず、固定のダミーデータを返す。
// これにより、dev 環境ではフロントが GitHub OAuth / 実トークンなしで
// 認証・グループ作成・投稿作成まで全エンドポイントを試せる。
type stubClient struct{}

// NewStubClient は dev 用のスタブ GitHub クライアントを作成する。
func NewStubClient() Client {
	return &stubClient{}
}

// ExchangeCode は固定のダミー access_token を返す。
func (c *stubClient) ExchangeCode(ctx context.Context, clientID, clientSecret, code string) (string, error) {
	return "dev-stub-access-token", nil
}

// GetUser は固定のダミーユーザーを返す。
func (c *stubClient) GetUser(ctx context.Context, accessToken string) (*User, error) {
	return &User{
		ID:        -1000,
		Login:     "dev-stub-user",
		AvatarURL: "https://avatars.githubusercontent.com/u/0?v=4",
		Name:      "Dev Stub User",
	}, nil
}

// GetRepoInfo はリクエストされた repoFullName をそのまま返す（avatar はダミー）。
func (c *stubClient) GetRepoInfo(ctx context.Context, repoFullName, accessToken string) (*RepoInfo, error) {
	return &RepoInfo{
		FullName:  repoFullName,
		AvatarURL: "https://avatars.githubusercontent.com/u/0?v=4",
	}, nil
}

// GetCollaborators は dev シードユーザー（alice / bob）を返す。
// これによりグループ作成時のコラボレーター自動参加が dev でも再現される。
func (c *stubClient) GetCollaborators(ctx context.Context, repoFullName, accessToken string) ([]User, error) {
	return []User{
		{ID: -1001, Login: "alice", AvatarURL: "https://avatars.githubusercontent.com/u/0?v=4", Name: "Alice (dev)"},
		{ID: -1002, Login: "bob", AvatarURL: "https://avatars.githubusercontent.com/u/0?v=4", Name: "Bob (dev)"},
	}, nil
}

// RegisterWebhook は何もせず成功を返す。
func (c *stubClient) RegisterWebhook(ctx context.Context, repoFullName, accessToken, webhookURL, secret string) error {
	return nil
}

// GetRecentCommits は固定のコミットサマリーを返す。
// dev トークンでも POST /posts が成功するようにするためのダミーデータ。
func (c *stubClient) GetRecentCommits(ctx context.Context, repoFullName, login, accessToken string) (*CommitSummary, error) {
	return &CommitSummary{
		CommitCount:         3,
		Additions:           120,
		Deletions:           30,
		LatestCommitMessage: "feat: dev stub commit",
		RepoFullName:        repoFullName,
	}, nil
}

// ListUserRepos は固定のダミーリポジトリ一覧を返す。
func (c *stubClient) ListUserRepos(ctx context.Context, accessToken string) ([]Repo, error) {
	return []Repo{
		{FullName: "dev-stub-user/sample-repo", Name: "sample-repo", Private: false, OwnerLogin: "dev-stub-user", AvatarURL: "https://avatars.githubusercontent.com/u/0?v=4", CanPush: true, CanAdmin: true},
		{FullName: "dev-stub-user/private-repo", Name: "private-repo", Private: true, OwnerLogin: "dev-stub-user", AvatarURL: "https://avatars.githubusercontent.com/u/0?v=4", CanPush: true, CanAdmin: false},
	}, nil
}

// RevokeToken は何もせず成功を返す。
func (c *stubClient) RevokeToken(ctx context.Context, clientID, clientSecret, accessToken string) error {
	return nil
}

// ListCommits は固定のダミーコミット一覧を返す。
func (c *stubClient) ListCommits(ctx context.Context, repoFullName, accessToken string, opts CommitListOptions) ([]Commit, error) {
	return []Commit{
		{SHA: "abc1234", Message: "feat: dev stub commit", AuthorName: "Dev Stub User", AuthorLogin: "dev-stub-user", Date: "2026-06-01T10:00:00Z", Additions: 120, Deletions: 30},
		{SHA: "def5678", Message: "fix: dev stub fix", AuthorName: "Dev Stub User", AuthorLogin: "dev-stub-user", Date: "2026-06-01T09:00:00Z", Additions: 10, Deletions: 5},
	}, nil
}
