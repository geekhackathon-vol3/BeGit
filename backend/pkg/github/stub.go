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

// GetAppInstallation は開発環境用の固定Installation情報を返す。
func (c *stubClient) GetAppInstallation(ctx context.Context, appID, privateKeyPEM string, installationID int64) (*AppInstallation, error) {
	return &AppInstallation{
		ID:                  installationID,
		AccountID:           -2000,
		AccountLogin:        "dev-stub-account",
		AccountType:         "Organization",
		RepositorySelection: "selected",
	}, nil
}

// CreateInstallationAccessToken は開発環境用の固定Installation Tokenを返す。
func (c *stubClient) CreateInstallationAccessToken(ctx context.Context, appID, privateKeyPEM string, installationID int64) (string, error) {
	return "dev-stub-installation-access-token", nil
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

// GetCommit は開発環境用の固定コミットを返す。
func (c *stubClient) GetCommit(ctx context.Context, repoFullName, sha, accessToken string) (*Commit, error) {
	return &Commit{
		SHA: sha, Message: "feat: dev stub commit", AuthorName: "Dev Stub User",
		AuthorLogin: "dev-stub-user", Date: "2026-06-01T10:00:00Z", Additions: 120, Deletions: 30,
	}, nil
}

// GetLatestPullRequest は開発環境用の固定 PR を返す。
func (c *stubClient) GetLatestPullRequest(ctx context.Context, repoFullName, login, accessToken string) (*PullRequestSummary, error) {
	return &PullRequestSummary{
		Number:       42,
		Title:        "feat: dev stub pull request",
		RepoFullName: repoFullName,
	}, nil
}

// ListPullRequests は開発環境用の固定 PR 一覧を返す。
func (c *stubClient) ListPullRequests(ctx context.Context, repoFullName, accessToken string, opts PullRequestListOptions) ([]PullRequest, error) {
	return []PullRequest{
		{Number: 42, Title: "feat: dev stub pull request", AuthorLogin: "dev-stub-user", State: "open", UpdatedAt: "2026-06-01T10:00:00Z"},
		{Number: 41, Title: "fix: dev stub fix", AuthorLogin: "dev-stub-user", State: "closed", Merged: true, UpdatedAt: "2026-05-31T10:00:00Z"},
	}, nil
}

// GetPullRequest は開発環境用の固定 PR を返す。
func (c *stubClient) GetPullRequest(ctx context.Context, repoFullName string, number int, accessToken string) (*PullRequest, error) {
	return &PullRequest{
		Number: number, Title: "feat: dev stub pull request", AuthorLogin: "dev-stub-user",
		State: "open", UpdatedAt: "2026-06-01T10:00:00Z",
	}, nil
}

// ListUserRepos は固定のダミーリポジトリ一覧を返す。
func (c *stubClient) ListUserRepos(ctx context.Context, accessToken string) ([]Repo, error) {
	return []Repo{
		{ID: 1001, FullName: "dev-stub-user/sample-repo", Name: "sample-repo", Private: false, OwnerLogin: "dev-stub-user", AvatarURL: "https://avatars.githubusercontent.com/u/0?v=4", CanPush: true, CanAdmin: true},
		{ID: 1002, FullName: "dev-stub-user/private-repo", Name: "private-repo", Private: true, OwnerLogin: "dev-stub-user", AvatarURL: "https://avatars.githubusercontent.com/u/0?v=4", CanPush: true, CanAdmin: false},
	}, nil
}

// ListInstallationRepos は開発環境用のInstallationリポジトリ一覧を返す。
func (c *stubClient) ListInstallationRepos(ctx context.Context, accessToken string) ([]Repo, error) {
	return []Repo{
		{ID: 2001, FullName: "dev-stub-account/BeGit", Name: "BeGit", Private: true, OwnerLogin: "dev-stub-account", AvatarURL: "https://avatars.githubusercontent.com/u/0?v=4", CanPush: true, CanAdmin: true},
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
