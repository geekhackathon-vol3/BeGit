package repository

import (
	"context"
	"testing"
	"time"
)

func TestGitHubAppInstallationRepository_Upsert(t *testing.T) {
	installedBy := int64(42)
	var capturedSQL string
	var capturedParams []interface{}
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			capturedSQL = sql
			capturedParams = params
			return 1, nil
		},
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			return []map[string]interface{}{{
				"id":                   float64(1),
				"installation_id":      float64(160356433),
				"account_id":           float64(123456),
				"account_login":        "geekhackathon-vol3",
				"account_type":         "Organization",
				"repository_selection": "selected",
				"installed_by_user_id": float64(42),
				"created_at":           "2026-09-10 12:00:00",
				"updated_at":           "2026-09-10 12:00:00",
			}}, nil
		},
	}

	repo := NewGitHubAppInstallationRepository(mock)
	installation, err := repo.Upsert(context.Background(), &GitHubAppInstallationUpsertInput{
		InstallationID:      160356433,
		AccountID:           123456,
		AccountLogin:        "geekhackathon-vol3",
		AccountType:         "Organization",
		RepositorySelection: "selected",
		InstalledByUserID:   &installedBy,
	})
	if err != nil {
		t.Fatalf("Upsert() failed: %v", err)
	}
	if installation.InstallationID != 160356433 {
		t.Fatalf("expected InstallationID=160356433, got %d", installation.InstallationID)
	}
	if installation.AccountLogin != "geekhackathon-vol3" {
		t.Fatalf("expected account login, got %q", installation.AccountLogin)
	}
	if capturedSQL == "" || len(capturedParams) != 6 {
		t.Fatalf("expected upsert SQL and six params, got sql=%q params=%#v", capturedSQL, capturedParams)
	}
	if !installation.CreatedAt.Equal(time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected CreatedAt: %v", installation.CreatedAt)
	}
}

func TestGitHubAppInstallationRepository_GetByInstallationID_NotFound(t *testing.T) {
	mock := &mockD1Client{}
	repo := NewGitHubAppInstallationRepository(mock)
	_, err := repo.GetByInstallationID(context.Background(), 999)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
