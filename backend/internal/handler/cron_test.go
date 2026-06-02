package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/service"
)

// mockCronService はテスト用の CronService モック
type mockCronService struct {
	runFunc func(ctx context.Context, kind string) error
}

func (m *mockCronService) RunCron(ctx context.Context, kind string) error {
	if m.runFunc != nil {
		return m.runFunc(ctx, kind)
	}
	return nil
}

func newCronRouter(h *CronHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/internal/cron", h.Run)
	return r
}

// TestCron_SecretMatch_DispatchesKind は secret 一致で 200・kind が振り分けられることを確認する
func TestCron_SecretMatch_DispatchesKind(t *testing.T) {
	var gotKind string
	svc := &mockCronService{runFunc: func(ctx context.Context, kind string) error {
		gotKind = kind
		return nil
	}}
	r := newCronRouter(NewCronHandler(svc, "topsecret"))

	req := httptest.NewRequest(http.MethodPost, "/internal/cron?kind=minutely", nil)
	req.Header.Set("X-Cron-Secret", "topsecret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotKind != "minutely" {
		t.Errorf("expected kind=minutely dispatched, got %q", gotKind)
	}
}

// TestCron_SecretMismatch_403 は secret 不一致で 403 を返すことを確認する
func TestCron_SecretMismatch_403(t *testing.T) {
	called := false
	svc := &mockCronService{runFunc: func(ctx context.Context, kind string) error {
		called = true
		return nil
	}}
	r := newCronRouter(NewCronHandler(svc, "topsecret"))

	req := httptest.NewRequest(http.MethodPost, "/internal/cron?kind=minutely", nil)
	req.Header.Set("X-Cron-Secret", "wrong")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if called {
		t.Error("cron service should not run on secret mismatch")
	}
}

// TestCron_InvalidKind_400 は kind 不正で 400 を返すことを確認する
func TestCron_InvalidKind_400(t *testing.T) {
	svc := &mockCronService{runFunc: func(ctx context.Context, kind string) error {
		return service.ErrInvalidCronKind
	}}
	r := newCronRouter(NewCronHandler(svc, "topsecret"))

	req := httptest.NewRequest(http.MethodPost, "/internal/cron?kind=weekly", nil)
	req.Header.Set("X-Cron-Secret", "topsecret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestCron_EmptySecretConfig_403 は cronSecret 未設定なら常に 403 になることを確認する
func TestCron_EmptySecretConfig_403(t *testing.T) {
	r := newCronRouter(NewCronHandler(&mockCronService{}, ""))
	req := httptest.NewRequest(http.MethodPost, "/internal/cron?kind=minutely", nil)
	req.Header.Set("X-Cron-Secret", "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when cron secret not configured, got %d", w.Code)
	}
}
