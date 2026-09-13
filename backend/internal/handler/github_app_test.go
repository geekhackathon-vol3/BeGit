package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/model"
)

type mockGitHubAppInstallationService struct {
	completeFunc func(ctx context.Context, installationID int64, setupAction string, installedByUserID *int64) (*model.GitHubAppInstallation, error)
}

func (m *mockGitHubAppInstallationService) CompleteSetup(ctx context.Context, installationID int64, setupAction string, installedByUserID *int64) (*model.GitHubAppInstallation, error) {
	return m.completeFunc(ctx, installationID, setupAction, installedByUserID)
}

func TestGitHubAppHandlerSetup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	router := gin.New()
	router.GET("/github/app/setup", NewGitHubAppHandler(&mockGitHubAppInstallationService{
		completeFunc: func(ctx context.Context, installationID int64, setupAction string, installedByUserID *int64) (*model.GitHubAppInstallation, error) {
			called = true
			if installationID != 160356433 || setupAction != "install" || installedByUserID != nil {
				t.Fatalf("unexpected setup arguments: id=%d action=%q user=%v", installationID, setupAction, installedByUserID)
			}
			return &model.GitHubAppInstallation{
				InstallationID: installationID,
				AccountLogin:   "geekhackathon-vol3",
				AccountType:    "Organization",
			}, nil
		},
	}).Setup)

	req := httptest.NewRequest(http.MethodGet, "/github/app/setup?installation_id=160356433&setup_action=install", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if !called {
		t.Fatal("expected installation service to be called")
	}
}

func TestGitHubAppHandlerSetupRejectsInvalidInstallationID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/github/app/setup", NewGitHubAppHandler(&mockGitHubAppInstallationService{
		completeFunc: func(context.Context, int64, string, *int64) (*model.GitHubAppInstallation, error) {
			t.Fatal("installation service must not be called")
			return nil, nil
		},
	}).Setup)

	req := httptest.NewRequest(http.MethodGet, "/github/app/setup?installation_id=invalid&setup_action=install", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", res.Code, res.Body.String())
	}
}

func TestGitHubAppHandlerSetupRedirectsToIOS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/github/app/setup", NewGitHubAppHandler(&mockGitHubAppInstallationService{
		completeFunc: func(context.Context, int64, string, *int64) (*model.GitHubAppInstallation, error) {
			return &model.GitHubAppInstallation{
				InstallationID: 160548256,
				AccountLogin:   "geekhackathon-vol3",
				AccountType:    "Organization",
			}, nil
		},
	}, "begit://github-app-setup").Setup)

	req := httptest.NewRequest(http.MethodGet, "/github/app/setup?installation_id=160548256&setup_action=install", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d: %s", res.Code, res.Body.String())
	}
	location, err := url.Parse(res.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	query := location.Query()
	if location.Scheme != "begit" || location.Host != "github-app-setup" {
		t.Fatalf("unexpected redirect URL: %s", location.String())
	}
	if query.Get("installation_id") != "160548256" || query.Get("setup_action") != "install" {
		t.Fatalf("unexpected redirect query: %s", location.RawQuery)
	}
	if query.Get("account_login") != "geekhackathon-vol3" || query.Get("account_type") != "Organization" {
		t.Fatalf("unexpected account query: %s", location.RawQuery)
	}
}

func TestBuildIOSRedirectURLRejectsNonBegitURL(t *testing.T) {
	if redirectURL, ok := buildIOSRedirectURL("https://example.com/callback", 160548256, "install", "owner", "User"); ok || redirectURL != "" {
		t.Fatalf("expected non-begit redirect URL to be rejected, got %q", redirectURL)
	}
}
