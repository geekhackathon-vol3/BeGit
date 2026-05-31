package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/crypto"
	githubpkg "github.com/irj0927/begit/pkg/github"
)

// AuthResult は認証成功時の結果
type AuthResult struct {
	User  model.User
	Token string // GitHub access_token の平文
}

// AuthServiceConfig は AuthService の設定
type AuthServiceConfig struct {
	GitHubClientID     string
	GitHubClientSecret string
}

// AuthService は GitHub OAuth フローを処理するサービスインターフェース
type AuthService interface {
	ExchangeCode(ctx context.Context, code string) (*AuthResult, error)
}

// authService は AuthService インターフェースの実装
type authService struct {
	config       AuthServiceConfig
	githubClient githubpkg.Client
	userRepo     repository.UserRepository
	crypto       crypto.Encryptor
}

// NewAuthService は AuthService を作成する
func NewAuthService(
	config AuthServiceConfig,
	githubClient githubpkg.Client,
	userRepo repository.UserRepository,
	crypto crypto.Encryptor,
) AuthService {
	return &authService{
		config:       config,
		githubClient: githubClient,
		userRepo:     userRepo,
		crypto:       crypto,
	}
}

// ExchangeCode は GitHub 認可コードを access_token に交換し、ユーザーを DB に UPSERT する
func (s *authService) ExchangeCode(ctx context.Context, code string) (*AuthResult, error) {
	// Step 1: GitHub OAuth code → access_token
	accessToken, err := s.githubClient.ExchangeCode(ctx, s.config.GitHubClientID, s.config.GitHubClientSecret, code)
	if err != nil {
		if errors.Is(err, githubpkg.ErrUnauthorized) {
			return nil, fmt.Errorf("%w: %v", ErrUnauthorized, err)
		}
		return nil, fmt.Errorf("auth_service: code exchange failed: %w", err)
	}

	// Step 2: access_token → GitHub ユーザー情報
	githubUser, err := s.githubClient.GetUser(ctx, accessToken)
	if err != nil {
		if errors.Is(err, githubpkg.ErrUnauthorized) {
			return nil, fmt.Errorf("%w: %v", ErrUnauthorized, err)
		}
		return nil, fmt.Errorf("auth_service: get user failed: %w", err)
	}

	// Step 3: access_token を AES-GCM 暗号化
	encryptedToken, err := s.crypto.Encrypt(accessToken)
	if err != nil {
		return nil, fmt.Errorf("auth_service: token encryption failed: %w", err)
	}

	// Step 4: ユーザーを DB に UPSERT
	user := &model.User{
		GitHubID:             githubUser.ID,
		GitHubLogin:          githubUser.Login,
		GitHubName:           githubUser.Name,
		AvatarURL:            githubUser.AvatarURL,
		EncryptedAccessToken: encryptedToken,
	}

	savedUser, err := s.userRepo.UpsertUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("auth_service: upsert user failed: %w", err)
	}

	return &AuthResult{
		User:  *savedUser,
		Token: accessToken, // 平文の access_token を返す
	}, nil
}
