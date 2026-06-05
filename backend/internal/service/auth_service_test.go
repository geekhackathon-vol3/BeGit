package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	githubpkg "github.com/irj0927/begit/pkg/github"
)

// mockGitHubClient はテスト用の GitHub クライアントモック
type mockGitHubClient struct {
	exchangeCodeFunc     func(ctx context.Context, clientID, clientSecret, code string) (string, error)
	getUserFunc          func(ctx context.Context, accessToken string) (*githubpkg.User, error)
	getRepoInfoFunc      func(ctx context.Context, repoFullName, accessToken string) (*githubpkg.RepoInfo, error)
	getCollaboratorsFunc func(ctx context.Context, repoFullName, accessToken string) ([]githubpkg.User, error)
	registerWebhookFunc  func(ctx context.Context, repoFullName, accessToken, webhookURL, secret string) error
	getRecentCommitsFunc func(ctx context.Context, repoFullName, login, accessToken string) (*githubpkg.CommitSummary, error)
	listUserReposFunc    func(ctx context.Context, accessToken string) ([]githubpkg.Repo, error)
	listCommitsFunc      func(ctx context.Context, repoFullName, accessToken string, opts githubpkg.CommitListOptions) ([]githubpkg.Commit, error)
	revokeTokenFunc      func(ctx context.Context, clientID, clientSecret, accessToken string) error
}

func (m *mockGitHubClient) ExchangeCode(ctx context.Context, clientID, clientSecret, code string) (string, error) {
	if m.exchangeCodeFunc != nil {
		return m.exchangeCodeFunc(ctx, clientID, clientSecret, code)
	}
	return "mock_access_token", nil
}

func (m *mockGitHubClient) GetUser(ctx context.Context, accessToken string) (*githubpkg.User, error) {
	if m.getUserFunc != nil {
		return m.getUserFunc(ctx, accessToken)
	}
	return &githubpkg.User{ID: 1, Login: "testuser", AvatarURL: "https://example.com/avatar.png", Name: "Test User"}, nil
}

func (m *mockGitHubClient) GetRepoInfo(ctx context.Context, repoFullName, accessToken string) (*githubpkg.RepoInfo, error) {
	if m.getRepoInfoFunc != nil {
		return m.getRepoInfoFunc(ctx, repoFullName, accessToken)
	}
	return &githubpkg.RepoInfo{FullName: repoFullName, AvatarURL: "https://example.com/avatar.png"}, nil
}

func (m *mockGitHubClient) GetCollaborators(ctx context.Context, repoFullName, accessToken string) ([]githubpkg.User, error) {
	if m.getCollaboratorsFunc != nil {
		return m.getCollaboratorsFunc(ctx, repoFullName, accessToken)
	}
	return []githubpkg.User{}, nil
}

func (m *mockGitHubClient) RegisterWebhook(ctx context.Context, repoFullName, accessToken, webhookURL, secret string) error {
	if m.registerWebhookFunc != nil {
		return m.registerWebhookFunc(ctx, repoFullName, accessToken, webhookURL, secret)
	}
	return nil
}

func (m *mockGitHubClient) GetRecentCommits(ctx context.Context, repoFullName, login, accessToken string) (*githubpkg.CommitSummary, error) {
	if m.getRecentCommitsFunc != nil {
		return m.getRecentCommitsFunc(ctx, repoFullName, login, accessToken)
	}
	return &githubpkg.CommitSummary{CommitCount: 3, Additions: 100, Deletions: 50, LatestCommitMessage: "Test commit"}, nil
}

func (m *mockGitHubClient) ListUserRepos(ctx context.Context, accessToken string) ([]githubpkg.Repo, error) {
	if m.listUserReposFunc != nil {
		return m.listUserReposFunc(ctx, accessToken)
	}
	return []githubpkg.Repo{}, nil
}

func (m *mockGitHubClient) ListCommits(ctx context.Context, repoFullName, accessToken string, opts githubpkg.CommitListOptions) ([]githubpkg.Commit, error) {
	if m.listCommitsFunc != nil {
		return m.listCommitsFunc(ctx, repoFullName, accessToken, opts)
	}
	return []githubpkg.Commit{}, nil
}

func (m *mockGitHubClient) RevokeToken(ctx context.Context, clientID, clientSecret, accessToken string) error {
	if m.revokeTokenFunc != nil {
		return m.revokeTokenFunc(ctx, clientID, clientSecret, accessToken)
	}
	return nil
}

// mockUserRepository はテスト用のユーザーリポジトリモック
type mockUserRepository struct {
	getByEncryptedTokenFunc func(ctx context.Context, encryptedToken string) (*model.User, error)
	upsertUserFunc          func(ctx context.Context, user *model.User) (*model.User, error)
	getByGitHubLoginFunc    func(ctx context.Context, login string) (*model.User, error)
	getByIDFunc             func(ctx context.Context, id int64) (*model.User, error)
}

func (m *mockUserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, repository.ErrNotFound
}

func (m *mockUserRepository) GetByEncryptedToken(ctx context.Context, encryptedToken string) (*model.User, error) {
	if m.getByEncryptedTokenFunc != nil {
		return m.getByEncryptedTokenFunc(ctx, encryptedToken)
	}
	return nil, errors.New("not found")
}

func (m *mockUserRepository) UpsertUser(ctx context.Context, user *model.User) (*model.User, error) {
	if m.upsertUserFunc != nil {
		return m.upsertUserFunc(ctx, user)
	}
	user.ID = 1
	user.CreatedAt = time.Now()
	return user, nil
}

func (m *mockUserRepository) GetByGitHubLogin(ctx context.Context, login string) (*model.User, error) {
	if m.getByGitHubLoginFunc != nil {
		return m.getByGitHubLoginFunc(ctx, login)
	}
	return nil, errors.New("not found")
}

// mockEncryptor はテスト用の暗号化モック
type mockEncryptor struct {
	encryptFunc func(plaintext string) (string, error)
	decryptFunc func(ciphertext string) (string, error)
}

func (m *mockEncryptor) Encrypt(plaintext string) (string, error) {
	if m.encryptFunc != nil {
		return m.encryptFunc(plaintext)
	}
	return "encrypted:" + plaintext, nil
}

func (m *mockEncryptor) Decrypt(ciphertext string) (string, error) {
	if m.decryptFunc != nil {
		return m.decryptFunc(ciphertext)
	}
	return "decrypted", nil
}

// TestAuthService_ExchangeCode は有効コードで AuthResult{User, Token} を返すことを確認する
func TestAuthService_ExchangeCode(t *testing.T) {
	githubClient := &mockGitHubClient{}
	userRepo := &mockUserRepository{}
	crypto := &mockEncryptor{}

	svc := NewAuthService(AuthServiceConfig{
		GitHubClientID:     "client_id",
		GitHubClientSecret: "client_secret",
	}, githubClient, userRepo, crypto)

	result, err := svc.ExchangeCode(context.Background(), "valid_code")
	if err != nil {
		t.Fatalf("ExchangeCode() failed: %v", err)
	}
	if result.User.GitHubLogin != "testuser" {
		t.Errorf("expected login=testuser, got %s", result.User.GitHubLogin)
	}
	if result.Token != "mock_access_token" {
		t.Errorf("expected token=mock_access_token, got %s", result.Token)
	}
}

// TestAuthService_ExchangeCode_InvalidCode は無効コードで ErrUnauthorized を返すことを確認する
func TestAuthService_ExchangeCode_InvalidCode(t *testing.T) {
	githubClient := &mockGitHubClient{
		exchangeCodeFunc: func(ctx context.Context, clientID, clientSecret, code string) (string, error) {
			return "", githubpkg.ErrUnauthorized
		},
	}
	userRepo := &mockUserRepository{}
	crypto := &mockEncryptor{}

	svc := NewAuthService(AuthServiceConfig{
		GitHubClientID:     "client_id",
		GitHubClientSecret: "client_secret",
	}, githubClient, userRepo, crypto)

	_, err := svc.ExchangeCode(context.Background(), "invalid_code")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}
