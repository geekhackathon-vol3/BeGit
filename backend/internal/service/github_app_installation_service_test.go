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

type mockAppInstallationClient struct {
	getFunc func(ctx context.Context, appID, privateKeyPEM string, installationID int64) (*githubpkg.AppInstallation, error)
}

func (m *mockAppInstallationClient) GetAppInstallation(ctx context.Context, appID, privateKeyPEM string, installationID int64) (*githubpkg.AppInstallation, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, appID, privateKeyPEM, installationID)
	}
	return &githubpkg.AppInstallation{
		ID:                  installationID,
		AccountID:           123,
		AccountLogin:        "geekhackathon-vol3",
		AccountType:         "Organization",
		RepositorySelection: "selected",
	}, nil
}

type mockAppInstallationRepository struct {
	upsertFunc func(ctx context.Context, input *repository.GitHubAppInstallationUpsertInput) (*model.GitHubAppInstallation, error)
}

func (m *mockAppInstallationRepository) Upsert(ctx context.Context, input *repository.GitHubAppInstallationUpsertInput) (*model.GitHubAppInstallation, error) {
	if m.upsertFunc != nil {
		return m.upsertFunc(ctx, input)
	}
	return &model.GitHubAppInstallation{
		InstallationID:      input.InstallationID,
		AccountID:           input.AccountID,
		AccountLogin:        input.AccountLogin,
		AccountType:         input.AccountType,
		RepositorySelection: input.RepositorySelection,
		InstalledByUserID:   input.InstalledByUserID,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}, nil
}

func (m *mockAppInstallationRepository) GetByInstallationID(context.Context, int64) (*model.GitHubAppInstallation, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAppInstallationRepository) ListByInstalledUserID(context.Context, int64) ([]model.GitHubAppInstallation, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAppInstallationRepository) DeleteByInstallationID(context.Context, int64) error {
	return errors.New("not implemented")
}

func TestGitHubAppInstallationService_CompleteSetup(t *testing.T) {
	var saved *repository.GitHubAppInstallationUpsertInput
	installedByUserID := int64(42)
	service := NewGitHubAppInstallationService(
		GitHubAppInstallationServiceConfig{AppID: "4886659", PrivateKeyPEM: "private-key"},
		&mockAppInstallationClient{},
		&mockAppInstallationRepository{
			upsertFunc: func(ctx context.Context, input *repository.GitHubAppInstallationUpsertInput) (*model.GitHubAppInstallation, error) {
				saved = input
				return &model.GitHubAppInstallation{InstallationID: input.InstallationID}, nil
			},
		},
	)

	installation, err := service.CompleteSetup(context.Background(), 160356433, "install", &installedByUserID)
	if err != nil {
		t.Fatalf("CompleteSetup() failed: %v", err)
	}
	if installation.InstallationID != 160356433 {
		t.Fatalf("expected installation ID 160356433, got %d", installation.InstallationID)
	}
	if saved == nil || saved.InstalledByUserID == nil || *saved.InstalledByUserID != installedByUserID {
		t.Fatalf("expected installer user ID to be persisted")
	}
}

func TestGitHubAppInstallationService_RejectsInvalidSetupAction(t *testing.T) {
	service := NewGitHubAppInstallationService(
		GitHubAppInstallationServiceConfig{AppID: "4886659", PrivateKeyPEM: "private-key"},
		&mockAppInstallationClient{},
		&mockAppInstallationRepository{},
	)

	_, err := service.CompleteSetup(context.Background(), 160356433, "delete", nil)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}
