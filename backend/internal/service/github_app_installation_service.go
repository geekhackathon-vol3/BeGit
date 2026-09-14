package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	githubpkg "github.com/irj0927/begit/pkg/github"
)

// GitHubAppInstallationServiceConfig はGitHub App認証情報を保持する。
type GitHubAppInstallationServiceConfig struct {
	AppID         string
	PrivateKeyPEM string
}

// GitHubAppInstallationService はGitHub Appインストール完了処理を提供する。
type GitHubAppInstallationService interface {
	CompleteSetup(ctx context.Context, installationID int64, setupAction string, installedByUserID *int64) (*model.GitHubAppInstallation, error)
}

type githubAppInstallationService struct {
	config           GitHubAppInstallationServiceConfig
	githubClient     githubpkg.AppInstallationClient
	installationRepo repository.GitHubAppInstallationRepository
}

func NewGitHubAppInstallationService(
	config GitHubAppInstallationServiceConfig,
	githubClient githubpkg.AppInstallationClient,
	installationRepo repository.GitHubAppInstallationRepository,
) GitHubAppInstallationService {
	return &githubAppInstallationService{
		config:           config,
		githubClient:     githubClient,
		installationRepo: installationRepo,
	}
}

// CompleteSetup はGitHubから受け取ったInstallation IDを検証し、D1へ保存する。
// GitHub APIでApp自身が所有するInstallationか確認してから保存するため、
// 任意のアカウント情報をクエリパラメータだけで登録できない。
func (s *githubAppInstallationService) CompleteSetup(ctx context.Context, installationID int64, setupAction string, installedByUserID *int64) (*model.GitHubAppInstallation, error) {
	if installationID <= 0 {
		return nil, fmt.Errorf("%w: installation_id must be positive", ErrValidation)
	}
	if setupAction != "install" && setupAction != "update" {
		return nil, fmt.Errorf("%w: unsupported setup_action", ErrValidation)
	}
	if s.githubClient == nil || s.installationRepo == nil {
		return nil, fmt.Errorf("%w: GitHub App installation service is not configured", ErrExternalAPI)
	}

	installation, err := s.githubClient.GetAppInstallation(ctx, s.config.AppID, s.config.PrivateKeyPEM, installationID)
	if err != nil {
		// GitHubの応答理由をコンテナログで確認できるようにする。
		// 秘密鍵そのものはエラーへ含めない。
		log.Printf("github_app_setup: get installation failed (installation_id=%d): %v", installationID, err)
		if errors.Is(err, githubpkg.ErrUnauthorized) || errors.Is(err, githubpkg.ErrForbidden) {
			return nil, fmt.Errorf("%w: GitHub App installation is not accessible", ErrForbidden)
		}
		return nil, fmt.Errorf("%w: failed to get GitHub App installation: %v", ErrExternalAPI, err)
	}

	saved, err := s.installationRepo.Upsert(ctx, &repository.GitHubAppInstallationUpsertInput{
		InstallationID:      installation.ID,
		AccountID:           installation.AccountID,
		AccountLogin:        installation.AccountLogin,
		AccountType:         installation.AccountType,
		RepositorySelection: installation.RepositorySelection,
		InstalledByUserID:   installedByUserID,
	})
	if err != nil {
		log.Printf("github_app_setup: save installation failed (installation_id=%d): %v", installationID, err)
		return nil, fmt.Errorf("%w: failed to save GitHub App installation: %v", ErrExternalAPI, err)
	}
	return saved, nil
}
