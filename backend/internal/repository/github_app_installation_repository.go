package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// GitHubAppInstallationUpsertInput はGitHub Appインストール情報の保存入力。
type GitHubAppInstallationUpsertInput struct {
	InstallationID      int64
	AccountID           int64
	AccountLogin        string
	AccountType         string
	RepositorySelection string
	InstalledByUserID   *int64
}

// GitHubAppInstallationRepository は github_app_installations テーブルへのアクセスを提供する。
type GitHubAppInstallationRepository interface {
	Upsert(ctx context.Context, input *GitHubAppInstallationUpsertInput) (*model.GitHubAppInstallation, error)
	GetByInstallationID(ctx context.Context, installationID int64) (*model.GitHubAppInstallation, error)
	ListByInstalledUserID(ctx context.Context, userID int64) ([]model.GitHubAppInstallation, error)
	DeleteByInstallationID(ctx context.Context, installationID int64) error
}

type githubAppInstallationRepository struct {
	db d1.Client
}

// NewGitHubAppInstallationRepository はGitHub App Installation Repositoryを作成する。
func NewGitHubAppInstallationRepository(db d1.Client) GitHubAppInstallationRepository {
	return &githubAppInstallationRepository{db: db}
}

func scanGitHubAppInstallation(row map[string]interface{}) (*model.GitHubAppInstallation, error) {
	installation := &model.GitHubAppInstallation{}
	if v, ok := row["id"].(float64); ok {
		installation.ID = int64(v)
	}
	if v, ok := row["installation_id"].(float64); ok {
		installation.InstallationID = int64(v)
	}
	if v, ok := row["account_id"].(float64); ok {
		installation.AccountID = int64(v)
	}
	if v, ok := row["account_login"].(string); ok {
		installation.AccountLogin = v
	}
	if v, ok := row["account_type"].(string); ok {
		installation.AccountType = v
	}
	if v, ok := row["repository_selection"].(string); ok {
		installation.RepositorySelection = v
	}
	if v, ok := row["installed_by_user_id"].(float64); ok {
		userID := int64(v)
		installation.InstalledByUserID = &userID
	}
	installation.CreatedAt = parseD1Time(row["created_at"])
	installation.UpdatedAt = parseD1Time(row["updated_at"])
	return installation, nil
}

func parseD1Time(value interface{}) time.Time {
	valueString, ok := value.(string)
	if !ok || valueString == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, valueString)
	if err == nil {
		return parsed
	}
	parsed, _ = time.Parse("2006-01-02 15:04:05", valueString)
	return parsed
}

func (r *githubAppInstallationRepository) Upsert(ctx context.Context, input *GitHubAppInstallationUpsertInput) (*model.GitHubAppInstallation, error) {
	if input == nil {
		return nil, errors.New("github_app_installation_repository: input is nil")
	}
	if input.InstallationID <= 0 {
		return nil, errors.New("github_app_installation_repository: installation_id must be positive")
	}
	if input.AccountID <= 0 {
		return nil, errors.New("github_app_installation_repository: account_id must be positive")
	}
	if input.AccountLogin == "" {
		return nil, errors.New("github_app_installation_repository: account_login is required")
	}
	if input.AccountType == "" {
		return nil, errors.New("github_app_installation_repository: account_type is required")
	}
	if input.RepositorySelection == "" {
		input.RepositorySelection = "selected"
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO github_app_installations
			(installation_id, account_id, account_login, account_type, repository_selection, installed_by_user_id)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(installation_id) DO UPDATE SET
			account_id           = excluded.account_id,
			account_login        = excluded.account_login,
			account_type         = excluded.account_type,
			repository_selection = excluded.repository_selection,
			installed_by_user_id = COALESCE(excluded.installed_by_user_id, github_app_installations.installed_by_user_id),
			updated_at           = datetime('now')`,
		[]interface{}{input.InstallationID, input.AccountID, input.AccountLogin, input.AccountType, input.RepositorySelection, input.InstalledByUserID},
	)
	if err != nil {
		return nil, fmt.Errorf("github_app_installation_repository: Upsert failed: %w", err)
	}

	return r.GetByInstallationID(ctx, input.InstallationID)
}

func (r *githubAppInstallationRepository) GetByInstallationID(ctx context.Context, installationID int64) (*model.GitHubAppInstallation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, installation_id, account_id, account_login, account_type,
		        repository_selection, installed_by_user_id, created_at, updated_at
		 FROM github_app_installations
		 WHERE installation_id = ? LIMIT 1`,
		[]interface{}{installationID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("github_app_installation_repository: GetByInstallationID failed: %w", err)
	}
	return scanGitHubAppInstallation(rows[0])
}

func (r *githubAppInstallationRepository) ListByInstalledUserID(ctx context.Context, userID int64) ([]model.GitHubAppInstallation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, installation_id, account_id, account_login, account_type,
		        repository_selection, installed_by_user_id, created_at, updated_at
		 FROM github_app_installations
		 WHERE installed_by_user_id = ?
		 ORDER BY created_at DESC`,
		[]interface{}{userID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []model.GitHubAppInstallation{}, nil
		}
		return nil, fmt.Errorf("github_app_installation_repository: ListByInstalledUserID failed: %w", err)
	}

	installations := make([]model.GitHubAppInstallation, 0, len(rows))
	for _, row := range rows {
		installation, err := scanGitHubAppInstallation(row)
		if err != nil {
			return nil, err
		}
		installations = append(installations, *installation)
	}
	return installations, nil
}

func (r *githubAppInstallationRepository) DeleteByInstallationID(ctx context.Context, installationID int64) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM github_app_installations WHERE installation_id = ?`,
		[]interface{}{installationID},
	)
	if err != nil {
		return fmt.Errorf("github_app_installation_repository: DeleteByInstallationID failed: %w", err)
	}
	return nil
}
