package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestConfig は buildHandler が起動できる最小の設定を返す
func newTestConfig(d1Metrics bool) *Config {
	return &Config{
		GitHubClientID:      "test_client_id",
		GitHubClientSecret:  "test_client_secret",
		GitHubWebhookSecret: "test_webhook_secret",
		DBEncryptionKey:     "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
		CFAccountID:         "test_account_id",
		D1DatabaseID:        "test_database_id",
		CFAPIToken:          "test_api_token",
		AppBaseURL:          "https://example.com",
		D1Metrics:           d1Metrics,
	}
}

func TestConfigFromHeaders_D1Metrics(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Internal-D1-Metrics", "true")

	cfg := &Config{}
	configFromHeaders(req, cfg)

	if !cfg.D1Metrics {
		t.Fatal("expected D1Metrics=true from header")
	}
}

// 計測の有効・無効の切り替えもハンドラー再構築の対象になる（再デプロイなしで vars の変更を反映する）
func TestServeHTTP_RebuildsWhenD1MetricsChanges(t *testing.T) {
	srv, builds, _ := countingServer(nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Internal-DB-Encryption-Key", "key")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Internal-DB-Encryption-Key", "key")
	req.Header.Set("X-Internal-D1-Metrics", "true")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	if *builds != 2 {
		t.Fatalf("expected 2 builds, got %d", *builds)
	}
	if !srv.cfg.D1Metrics {
		t.Fatal("rebuilt config should enable D1Metrics")
	}
}

func TestBuildHandler_D1MetricsEnabled(t *testing.T) {
	srv := &server{cfg: newTestConfig(true)}
	handler, err := srv.buildHandler()
	if err != nil {
		t.Fatalf("buildHandler() failed: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if got := rr.Header().Get("X-D1-Queries"); got != "0" {
		t.Errorf("X-D1-Queries = %q, want 0", got)
	}

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/internal/d1-metrics", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("GET /internal/d1-metrics status = %d, want 200", rr.Code)
	}
}

// 計測を有効にしない環境（本番・dev）では、ヘッダーも総数エンドポイントも出さない
func TestBuildHandler_D1MetricsDisabled(t *testing.T) {
	srv := &server{cfg: newTestConfig(false)}
	handler, err := srv.buildHandler()
	if err != nil {
		t.Fatalf("buildHandler() failed: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if got := rr.Header().Get("X-D1-Queries"); got != "" {
		t.Errorf("X-D1-Queries = %q, want empty", got)
	}

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/internal/d1-metrics", nil))
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /internal/d1-metrics status = %d, want 404", rr.Code)
	}
}
