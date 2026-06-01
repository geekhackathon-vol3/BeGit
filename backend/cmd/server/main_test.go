package main

import (
	"os"
	"testing"
)

// TestConfigValidation は必須環境変数の検証をテストする
func TestConfigValidation(t *testing.T) {
	// すべての必須環境変数を設定した場合
	envVars := map[string]string{
		"GITHUB_CLIENT_ID":              "test_client_id",
		"GITHUB_CLIENT_SECRET":          "test_client_secret",
		"GITHUB_WEBHOOK_SECRET":         "test_webhook_secret",
		"FIREBASE_SERVICE_ACCOUNT_JSON": `{"type":"service_account","project_id":"test"}`,
		"DB_ENCRYPTION_KEY":             "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
		"CF_ACCOUNT_ID":                 "test_account_id",
		"D1_DATABASE_ID":                "test_database_id",
		"CF_API_TOKEN":                  "test_api_token",
		"APP_BASE_URL":                  "https://example.com",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
	}
	defer func() {
		for k := range envVars {
			os.Unsetenv(k)
		}
	}()

	cfg, err := loadConfig()
	if err != nil {
		t.Errorf("loadConfig() should succeed when all required env vars are set, got error: %v", err)
	}

	if cfg.GitHubClientID != "test_client_id" {
		t.Errorf("expected GitHubClientID=test_client_id, got %s", cfg.GitHubClientID)
	}
	if cfg.AppBaseURL != "https://example.com" {
		t.Errorf("expected AppBaseURL=https://example.com, got %s", cfg.AppBaseURL)
	}
}

// TestConfigValidation_MissingRequired は必須環境変数が欠けた場合のエラーをテストする
func TestConfigValidation_MissingRequired(t *testing.T) {
	// すべての環境変数をクリアする
	requiredVars := []string{
		"GITHUB_CLIENT_ID",
		"GITHUB_CLIENT_SECRET",
		"GITHUB_WEBHOOK_SECRET",
		"FIREBASE_SERVICE_ACCOUNT_JSON",
		"DB_ENCRYPTION_KEY",
		"CF_ACCOUNT_ID",
		"D1_DATABASE_ID",
		"CF_API_TOKEN",
		"APP_BASE_URL",
	}
	for _, v := range requiredVars {
		os.Unsetenv(v)
	}

	_, err := loadConfig()
	if err == nil {
		t.Error("loadConfig() should fail when required env vars are missing")
	}
}
