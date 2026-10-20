package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/pkg/d1"
)

// newD1StubServer は指定ステータスを順に返す D1 REST API のスタブを立てる
func newD1StubServer(t *testing.T, statuses ...int) *httptest.Server {
	t.Helper()
	i := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := statuses[i%len(statuses)]
		i++
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server
}

// callD1 はリクエストの ctx を引き継いで D1 スタブを n 回呼ぶ（repository 経由の呼び出しを模す）
func callD1(c *gin.Context, url string, n int) {
	client := &http.Client{Transport: d1.NewMetricsTransport(nil)}
	for i := 0; i < n; i++ {
		req, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, url, nil)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
}

// TestD1MetricsWritesHeaders は D1 呼び出しの内訳がレスポンスヘッダーに載ることを確認する
func TestD1MetricsWritesHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := newD1StubServer(t, http.StatusOK, http.StatusTooManyRequests)

	r := gin.New()
	r.Use(D1Metrics())
	r.GET("/x", func(c *gin.Context) {
		callD1(c, server.URL, 2)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	if got := w.Header().Get(headerD1Queries); got != "2" {
		t.Errorf("%s = %q, want 2", headerD1Queries, got)
	}
	if got := w.Header().Get(headerD1Status); got != "200,429" {
		t.Errorf("%s = %q, want 200,429", headerD1Status, got)
	}
	if _, err := strconv.ParseFloat(w.Header().Get(headerD1TimeMs), 64); err != nil {
		t.Errorf("%s = %q, want number", headerD1TimeMs, w.Header().Get(headerD1TimeMs))
	}
}

// TestD1MetricsWithoutQueries は D1 を呼ばないリクエストでも 0 件として載ることを確認する
func TestD1MetricsWithoutQueries(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(D1Metrics())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	if got := w.Header().Get(headerD1Queries); got != "0" {
		t.Errorf("%s = %q, want 0", headerD1Queries, got)
	}
	if got := w.Header().Get(headerD1Status); got != "" {
		t.Errorf("%s = %q, want empty", headerD1Status, got)
	}
}

// TestD1MetricsOnAbort は後段のミドルウェアがエラーで中断した場合もヘッダーが載ることを確認する
// （D1 の 429 が 500 に変換されるケースを k6 側で判別するため）
func TestD1MetricsOnAbort(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := newD1StubServer(t, http.StatusTooManyRequests)

	r := gin.New()
	r.Use(D1Metrics())
	r.GET("/x", func(c *gin.Context) {
		callD1(c, server.URL, 1)
		respondError(c, http.StatusInternalServerError, "internal server error")
	}, func(c *gin.Context) {
		t.Error("handler after abort must not run")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if got := w.Header().Get(headerD1Status); got != "429" {
		t.Errorf("%s = %q, want 429", headerD1Status, got)
	}
}

// TestD1MetricsTotals は総数エンドポイントがプロセス全体の呼び出し数を返すことを確認する
func TestD1MetricsTotals(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := newD1StubServer(t, http.StatusTooManyRequests)
	before := d1.Totals()

	r := gin.New()
	r.GET("/internal/d1-metrics", D1MetricsTotals)
	// ctx に計測を仕込まない呼び出し（レスポンス後の非同期処理を模す）
	r.GET("/bg", func(c *gin.Context) {
		callD1(c, server.URL, 1)
		c.Status(http.StatusNoContent)
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/bg", nil))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/d1-metrics", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got d1.TotalsSnapshot
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v (%s)", err, w.Body.String())
	}
	if got.Calls-before.Calls != 1 || got.RateLimited-before.RateLimited != 1 {
		t.Errorf("delta = calls %d / rate_limited %d, want 1 / 1",
			got.Calls-before.Calls, got.RateLimited-before.RateLimited)
	}
}
