package main

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
)

func TestConfigFromHeaders_DecodesGitHubAppCredentials(t *testing.T) {
	privateKey := "-----BEGIN PRIVATE KEY-----\nprivate-key\n-----END PRIVATE KEY-----\n"
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Internal-Github-App-Id", "4886659")
	req.Header.Set(
		"X-Internal-Github-App-Private-Key-B64",
		base64.StdEncoding.EncodeToString([]byte(privateKey)),
	)
	req.Header.Set("X-Internal-Github-App-Ios-Redirect-Uri", "begit://github-app-setup")

	cfg := &Config{}
	configFromHeaders(req, cfg)

	if cfg.GitHubAppID != "4886659" {
		t.Fatalf("expected GitHubAppID=4886659, got %q", cfg.GitHubAppID)
	}
	if cfg.GitHubAppPrivateKey != privateKey {
		t.Fatalf("decoded GitHub App private key does not match original")
	}
	if cfg.GitHubAppIOSRedirectURI != "begit://github-app-setup" {
		t.Fatalf("expected iOS redirect URI to be forwarded, got %q", cfg.GitHubAppIOSRedirectURI)
	}
}

func TestConfigFromHeaders_DecodesFirebaseServiceAccount(t *testing.T) {
	serviceAccount := "{\"type\":\"service_account\",\n\"private_key\":\"line1\\nline2\"}"
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(
		"X-Internal-Firebase-Service-Account-B64",
		base64.StdEncoding.EncodeToString([]byte(serviceAccount)),
	)

	cfg := &Config{}
	configFromHeaders(req, cfg)

	if cfg.FirebaseServiceAccountJSON != serviceAccount {
		t.Fatalf("decoded Firebase service account does not match original: %q", cfg.FirebaseServiceAccountJSON)
	}
}

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

func TestBuildHandler_AllowsMissingR2Credentials(t *testing.T) {
	srv := &server{
		cfg: &Config{
			GitHubClientID:      "test_client_id",
			GitHubClientSecret:  "test_client_secret",
			GitHubWebhookSecret: "test_webhook_secret",
			DBEncryptionKey:     "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
			CFAccountID:         "test_account_id",
			D1DatabaseID:        "test_database_id",
			CFAPIToken:          "test_api_token",
			AppBaseURL:          "https://example.com",
		},
	}

	handler, err := srv.buildHandler()
	if err != nil {
		t.Fatalf("buildHandler() should not require R2 credentials for auth routes: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected healthz status 200, got %d", rr.Code)
	}
}

// countingServer は構築回数と、構築時に見えていた FirebaseServiceAccountJSON を記録するテスト用 server を返す
func countingServer(buildErr *error) (*server, *int, *[]string) {
	builds := 0
	seen := []string{}
	srv := &server{cfg: &Config{}}
	srv.newHandler = func() (http.Handler, error) {
		if buildErr != nil && *buildErr != nil {
			return nil, *buildErr
		}
		builds++
		seen = append(seen, srv.cfg.FirebaseServiceAccountJSON)
		n := builds
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Build", strconv.Itoa(n))
		}), nil
	}
	return srv, &builds, &seen
}

func serveWithFirebase(srv *server, firebaseJSON string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Internal-DB-Encryption-Key", "key")
	if firebaseJSON != "" {
		req.Header.Set("X-Internal-Firebase-Service-Account-B64", base64.StdEncoding.EncodeToString([]byte(firebaseJSON)))
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

// 同じ内部設定のリクエストではハンドラーを再構築しない
func TestServeHTTP_DoesNotRebuildWhenConfigUnchanged(t *testing.T) {
	srv, builds, _ := countingServer(nil)

	serveWithFirebase(srv, "")
	serveWithFirebase(srv, "")
	serveWithFirebase(srv, "")

	if *builds != 1 {
		t.Fatalf("expected 1 build, got %d", *builds)
	}
}

// secret 追加などで内部設定が変わったら、再起動を待たずに新しい設定で再構築する
func TestServeHTTP_RebuildsWhenSecretAdded(t *testing.T) {
	srv, builds, seen := countingServer(nil)

	serveWithFirebase(srv, "")
	rr := serveWithFirebase(srv, `{"project_id":"begit"}`)

	if *builds != 2 {
		t.Fatalf("expected 2 builds, got %d", *builds)
	}
	if got := (*seen)[1]; got != `{"project_id":"begit"}` {
		t.Fatalf("rebuilt handler should see new firebase config, got %q", got)
	}
	if rr.Header().Get("X-Build") != "2" {
		t.Fatalf("request should be served by rebuilt handler, got build %q", rr.Header().Get("X-Build"))
	}
}

// 内部ヘッダーが無いリクエスト（Worker 非経由）では既存ハンドラーをそのまま使う
func TestServeHTTP_KeepsHandlerForRequestsWithoutInternalHeaders(t *testing.T) {
	srv, builds, _ := countingServer(nil)

	serveWithFirebase(srv, `{"project_id":"begit"}`)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if *builds != 1 {
		t.Fatalf("expected 1 build, got %d", *builds)
	}
}

// 再構築に失敗したら既存ハンドラーで応答し続け、同じ設定での再試行を繰り返さない
func TestServeHTTP_KeepsPreviousHandlerWhenRebuildFails(t *testing.T) {
	var buildErr error
	srv, builds, _ := countingServer(&buildErr)

	serveWithFirebase(srv, "")
	buildErr = errors.New("boom")
	rr := serveWithFirebase(srv, `{"project_id":"begit"}`)
	serveWithFirebase(srv, `{"project_id":"begit"}`)

	if rr.Code != http.StatusOK || rr.Header().Get("X-Build") != "1" {
		t.Fatalf("expected previous handler to serve, got code=%d build=%q", rr.Code, rr.Header().Get("X-Build"))
	}
	if *builds != 1 {
		t.Fatalf("expected no successful rebuild, got %d builds", *builds)
	}
	if srv.cfg.FirebaseServiceAccountJSON != "" {
		t.Fatalf("cfg should be restored after failed rebuild")
	}
}

func TestConfigFromHeaders_BeGitTimeAllowMultiplePerSprint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Internal-Begit-Time-Allow-Multiple-Per-Sprint", "true")

	cfg := &Config{}
	configFromHeaders(req, cfg)

	if !cfg.BeGitTimeAllowMultiplePerSprint {
		t.Fatal("expected BeGitTimeAllowMultiplePerSprint=true from header")
	}
}

// この設定の変更もハンドラー再構築の対象になる（dev の vars 変更を再起動なしで反映する）
func TestServeHTTP_RebuildsWhenAllowMultiplePerSprintChanges(t *testing.T) {
	srv, builds, _ := countingServer(nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Internal-DB-Encryption-Key", "key")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Internal-DB-Encryption-Key", "key")
	req.Header.Set("X-Internal-Begit-Time-Allow-Multiple-Per-Sprint", "true")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	if *builds != 2 {
		t.Fatalf("expected 2 builds, got %d", *builds)
	}
	if !srv.cfg.BeGitTimeAllowMultiplePerSprint {
		t.Fatal("rebuilt config should enable BeGitTimeAllowMultiplePerSprint")
	}
}
