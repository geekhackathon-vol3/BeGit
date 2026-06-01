// Package github は GitHub REST API v3 クライアントを提供する。
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// エラー型定義
var (
	// ErrUnauthorized は GitHub API が 401 を返した場合に返す
	ErrUnauthorized = errors.New("unauthorized")
	// ErrExternalAPI は GitHub API で予期しないエラーが発生した場合に返す
	ErrExternalAPI = errors.New("external api error")
)

// User は GitHub ユーザー情報
type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	Name      string `json:"name"`
}

// RepoInfo は GitHub リポジトリ情報
type RepoInfo struct {
	FullName  string `json:"full_name"`
	AvatarURL string // owner.avatar_url
}

// CommitSummary はコミットサマリー情報
type CommitSummary struct {
	CommitCount         int
	Additions           int
	Deletions           int
	LatestCommitMessage string
	RepoFullName        string
}

// Commit は GitHub のコミット情報（コミット一覧用）
type Commit struct {
	SHA         string `json:"sha"`
	Message     string `json:"message"`
	AuthorName  string `json:"author_name"`
	AuthorLogin string `json:"author_login"`
	Date        string `json:"date"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
}

// CommitListOptions はコミット一覧取得のクエリオプション
type CommitListOptions struct {
	Author  string // author でフィルタ（login or email）
	Since   string // ISO8601 これ以降
	Until   string // ISO8601 これ以前
	PerPage int    // 取得件数（1件ごとに詳細を取得して差分統計を含める）
}

// Repo は GitHub リポジトリ情報（リポジトリ一覧用）
type Repo struct {
	FullName   string `json:"full_name"`
	Name       string `json:"name"`
	Private    bool   `json:"private"`
	OwnerLogin string `json:"owner_login"`
	AvatarURL  string `json:"avatar_url"`
	CanPush    bool   `json:"can_push"`
	CanAdmin   bool   `json:"can_admin"`
}

// Client は GitHub REST API v3 インターフェース
type Client interface {
	ExchangeCode(ctx context.Context, clientID, clientSecret, code string) (accessToken string, err error)
	GetUser(ctx context.Context, accessToken string) (*User, error)
	GetRepoInfo(ctx context.Context, repoFullName, accessToken string) (*RepoInfo, error)
	GetCollaborators(ctx context.Context, repoFullName, accessToken string) ([]User, error)
	RegisterWebhook(ctx context.Context, repoFullName, accessToken, webhookURL, secret string) error
	GetRecentCommits(ctx context.Context, repoFullName, login, accessToken string) (*CommitSummary, error)
	ListUserRepos(ctx context.Context, accessToken string) ([]Repo, error)
	ListCommits(ctx context.Context, repoFullName, accessToken string, opts CommitListOptions) ([]Commit, error)
}

// githubClient は Client インターフェースの実装
type githubClient struct {
	httpClient    *http.Client
	oauthEndpoint string // デフォルト: https://github.com
	apiEndpoint   string // デフォルト: https://api.github.com
}

// NewClient は GitHub API クライアントを作成する
func NewClient() Client {
	return &githubClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		oauthEndpoint: "https://github.com",
		apiEndpoint:   "https://api.github.com",
	}
}

// doAPIRequest は GitHub API への GET リクエストを実行する
func (c *githubClient) doAPIRequest(ctx context.Context, method, path, accessToken string, body interface{}) (*http.Response, error) {
	url := c.apiEndpoint + path

	var bodyReader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("github: failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("github: failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: request failed: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, ErrUnauthorized
	}

	// 2xx 以外のステータスコードをエラーとして扱う
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// ボディから一部読み取ってエラーメッセージに含める
		bodySnippet := make([]byte, 200)
		n, _ := io.ReadFull(resp.Body, bodySnippet)
		resp.Body.Close()
		return nil, fmt.Errorf("%w: status %d, body: %s", ErrExternalAPI, resp.StatusCode, string(bodySnippet[:n]))
	}

	return resp, nil
}

// ExchangeCode は GitHub OAuth code を access_token に交換する
func (c *githubClient) ExchangeCode(ctx context.Context, clientID, clientSecret, code string) (string, error) {
	url := c.oauthEndpoint + "/login/oauth/access_token"

	payload := map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"code":          code,
	}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("github: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("github: oauth request failed: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("github: failed to decode oauth response: %w", err)
	}

	if errCode := result["error"]; errCode != "" {
		return "", fmt.Errorf("%w: %s", ErrUnauthorized, result["error_description"])
	}

	token := result["access_token"]
	if token == "" {
		return "", fmt.Errorf("%w: no access_token in response", ErrUnauthorized)
	}

	return token, nil
}

// GetUser は GitHub ユーザー情報を取得する
func (c *githubClient) GetUser(ctx context.Context, accessToken string) (*User, error) {
	resp, err := c.doAPIRequest(ctx, http.MethodGet, "/user", accessToken, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("github: failed to decode user: %w", err)
	}

	return &user, nil
}

// GetRepoInfo はリポジトリ情報（owner の avatar_url を含む）を取得する
func (c *githubClient) GetRepoInfo(ctx context.Context, repoFullName, accessToken string) (*RepoInfo, error) {
	resp, err := c.doAPIRequest(ctx, http.MethodGet, "/repos/"+repoFullName, accessToken, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: failed to decode repo info: %w", err)
	}

	info := &RepoInfo{
		FullName: repoFullName,
	}
	if fullName, ok := raw["full_name"].(string); ok {
		info.FullName = fullName
	}
	if owner, ok := raw["owner"].(map[string]interface{}); ok {
		if avatarURL, ok := owner["avatar_url"].(string); ok {
			info.AvatarURL = avatarURL
		}
	}

	return info, nil
}

// GetCollaborators はリポジトリのコラボレーター一覧を取得する
func (c *githubClient) GetCollaborators(ctx context.Context, repoFullName, accessToken string) ([]User, error) {
	resp, err := c.doAPIRequest(ctx, http.MethodGet, "/repos/"+repoFullName+"/collaborators", accessToken, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("github: failed to decode collaborators: %w", err)
	}

	return users, nil
}

// RegisterWebhook は GitHub リポジトリに Webhook を登録する
// push と pull_request_review イベントを受信するよう設定する
func (c *githubClient) RegisterWebhook(ctx context.Context, repoFullName, accessToken, webhookURL, secret string) error {
	payload := map[string]interface{}{
		"name":   "web",
		"active": true,
		"events": []string{"push", "pull_request_review"},
		"config": map[string]string{
			"url":          webhookURL,
			"content_type": "json",
			"secret":       secret,
		},
	}

	resp, err := c.doAPIRequest(ctx, http.MethodPost, "/repos/"+repoFullName+"/hooks", accessToken, payload)
	if err != nil {
		return fmt.Errorf("%w: failed to register webhook: %v", ErrExternalAPI, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: webhook registration returned status %d", ErrExternalAPI, resp.StatusCode)
	}

	return nil
}

// GetRecentCommits はリポジトリの最近のコミット情報を取得する
// 実装: GET /repos/{owner}/{repo}/commits?author={login}&per_page=5 で一覧を取得し、
// 最新コミットの SHA で GET /repos/{owner}/{repo}/commits/{sha} を呼んで additions/deletions を取得
func (c *githubClient) GetRecentCommits(ctx context.Context, repoFullName, login, accessToken string) (*CommitSummary, error) {
	// Step 1: コミット一覧を取得
	path := fmt.Sprintf("/repos/%s/commits?author=%s&per_page=5", repoFullName, login)
	resp, err := c.doAPIRequest(ctx, http.MethodGet, path, accessToken, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to list commits: %v", ErrExternalAPI, err)
	}
	defer resp.Body.Close()

	var commits []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return nil, fmt.Errorf("github: failed to decode commits list: %w", err)
	}

	if len(commits) == 0 {
		return &CommitSummary{RepoFullName: repoFullName}, nil
	}

	// Step 2: 最新コミットの詳細を取得
	latestSHA, _ := commits[0]["sha"].(string)
	detailPath := fmt.Sprintf("/repos/%s/commits/%s", repoFullName, latestSHA)
	detailResp, err := c.doAPIRequest(ctx, http.MethodGet, detailPath, accessToken, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get commit detail: %v", ErrExternalAPI, err)
	}
	defer detailResp.Body.Close()

	var commitDetail map[string]interface{}
	if err := json.NewDecoder(detailResp.Body).Decode(&commitDetail); err != nil {
		return nil, fmt.Errorf("github: failed to decode commit detail: %w", err)
	}

	summary := &CommitSummary{
		CommitCount:  len(commits),
		RepoFullName: repoFullName,
	}

	// コミットメッセージを取得
	if commit, ok := commitDetail["commit"].(map[string]interface{}); ok {
		if msg, ok := commit["message"].(string); ok {
			// 最初の行のみ取得
			if idx := strings.Index(msg, "\n"); idx >= 0 {
				summary.LatestCommitMessage = msg[:idx]
			} else {
				summary.LatestCommitMessage = msg
			}
		}
	}

	// additions/deletions を取得
	if stats, ok := commitDetail["stats"].(map[string]interface{}); ok {
		if additions, ok := stats["additions"].(float64); ok {
			summary.Additions = int(additions)
		}
		if deletions, ok := stats["deletions"].(float64); ok {
			summary.Deletions = int(deletions)
		}
	}

	return summary, nil
}

// ListUserRepos は認証ユーザーがアクセスできるリポジトリ一覧を取得する。
// GET /user/repos?affiliation=owner,collaborator&sort=updated&per_page=100 のプロキシ。
// グループ作成では Webhook 登録のため push / admin 権限が要るので、その権限のあるものに絞る。
func (c *githubClient) ListUserRepos(ctx context.Context, accessToken string) ([]Repo, error) {
	resp, err := c.doAPIRequest(ctx, http.MethodGet,
		"/user/repos?affiliation=owner,collaborator&sort=updated&per_page=100",
		accessToken, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to list user repos: %v", ErrExternalAPI, err)
	}
	defer resp.Body.Close()

	var raw []struct {
		FullName string `json:"full_name"`
		Name     string `json:"name"`
		Private  bool   `json:"private"`
		Owner    struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		} `json:"owner"`
		Permissions struct {
			Admin bool `json:"admin"`
			Push  bool `json:"push"`
		} `json:"permissions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: failed to decode user repos: %w", err)
	}

	repos := make([]Repo, 0, len(raw))
	for _, r := range raw {
		// push / admin 権限がないリポジトリは Webhook 登録できないため除外する
		if !r.Permissions.Push && !r.Permissions.Admin {
			continue
		}
		repos = append(repos, Repo{
			FullName:   r.FullName,
			Name:       r.Name,
			Private:    r.Private,
			OwnerLogin: r.Owner.Login,
			AvatarURL:  r.Owner.AvatarURL,
			CanPush:    r.Permissions.Push,
			CanAdmin:   r.Permissions.Admin,
		})
	}
	return repos, nil
}

// ListCommits はリポジトリのコミット一覧を取得する。
// GET /repos/{owner}/{repo}/commits のプロキシ。一覧レスポンスには差分統計が
// 含まれないため、各コミットの詳細 (GET .../commits/{sha}) を取得して
// additions/deletions を埋める。PerPage が取得・詳細取得の件数上限になる。
func (c *githubClient) ListCommits(ctx context.Context, repoFullName, accessToken string, opts CommitListOptions) ([]Commit, error) {
	perPage := opts.PerPage
	if perPage <= 0 {
		perPage = 20
	}
	if perPage > 50 {
		perPage = 50
	}

	q := url.Values{}
	q.Set("per_page", strconv.Itoa(perPage))
	if opts.Author != "" {
		q.Set("author", opts.Author)
	}
	if opts.Since != "" {
		q.Set("since", opts.Since)
	}
	if opts.Until != "" {
		q.Set("until", opts.Until)
	}

	resp, err := c.doAPIRequest(ctx, http.MethodGet,
		"/repos/"+repoFullName+"/commits?"+q.Encode(), accessToken, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to list commits: %v", ErrExternalAPI, err)
	}
	defer resp.Body.Close()

	var raw []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name string `json:"name"`
				Date string `json:"date"`
			} `json:"author"`
		} `json:"commit"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: failed to decode commits list: %w", err)
	}

	commits := make([]Commit, 0, len(raw))
	for _, r := range raw {
		commit := Commit{
			SHA:         r.SHA,
			Message:     r.Commit.Message,
			AuthorName:  r.Commit.Author.Name,
			AuthorLogin: r.Author.Login,
			Date:        r.Commit.Author.Date,
		}
		// 差分統計は詳細エンドポイントから取得する
		if add, del, err := c.commitStats(ctx, repoFullName, r.SHA, accessToken); err == nil {
			commit.Additions = add
			commit.Deletions = del
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

// commitStats は単一コミットの additions/deletions を取得する。
func (c *githubClient) commitStats(ctx context.Context, repoFullName, sha, accessToken string) (additions, deletions int, err error) {
	resp, err := c.doAPIRequest(ctx, http.MethodGet,
		"/repos/"+repoFullName+"/commits/"+sha, accessToken, nil)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var detail struct {
		Stats struct {
			Additions int `json:"additions"`
			Deletions int `json:"deletions"`
		} `json:"stats"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return 0, 0, err
	}
	return detail.Stats.Additions, detail.Stats.Deletions, nil
}
