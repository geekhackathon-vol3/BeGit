package d1

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// newMetricsTestClient は計測用 Transport を挟んだ d1Client を返すヘルパー
func newMetricsTestClient(server *httptest.Server) *d1Client {
	return &d1Client{
		httpClient:  &http.Client{Transport: NewMetricsTransport(server.Client().Transport)},
		accountID:   "test_account",
		databaseID:  "test_db",
		apiToken:    "test_token",
		apiEndpoint: server.URL + "/accounts/{account_id}/d1/database/{database_id}/query",
	}
}

// TestMetricsTransportRecordsPerRequest はリクエスト単位でクエリ数とステータスが記録されることを確認する
func TestMetricsTransportRecordsPerRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockD1Response([]map[string]interface{}{{"id": float64(1)}}, true, ""))
	}))
	defer server.Close()

	client := newMetricsTestClient(server)
	ctx, m := WithMetrics(context.Background())

	for i := 0; i < 3; i++ {
		if _, err := client.Query(ctx, "SELECT 1", nil); err != nil {
			t.Fatalf("Query failed: %v", err)
		}
	}

	got := m.Snapshot()
	if got.Queries != 3 {
		t.Errorf("Queries = %d, want 3", got.Queries)
	}
	if len(got.Statuses) != 3 || got.Statuses[0] != http.StatusOK {
		t.Errorf("Statuses = %v, want [200 200 200]", got.Statuses)
	}
	if got.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", got.Duration)
	}
}

// TestMetricsTransportRecordsRateLimit は 429 がステータスとして残り、総数にも数えられることを確認する
func TestMetricsTransportRecordsRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write(mockD1Response(nil, false, "rate limited"))
	}))
	defer server.Close()

	client := newMetricsTestClient(server)
	ctx, m := WithMetrics(context.Background())
	before := Totals()

	_, err := client.Query(ctx, "SELECT 1", nil)
	if !errors.Is(err, ErrD1API) {
		t.Fatalf("err = %v, want ErrD1API", err)
	}

	got := m.Snapshot()
	if len(got.Statuses) != 1 || got.Statuses[0] != http.StatusTooManyRequests {
		t.Errorf("Statuses = %v, want [429]", got.Statuses)
	}
	after := Totals()
	if after.Calls-before.Calls != 1 || after.RateLimited-before.RateLimited != 1 {
		t.Errorf("Totals delta = %+v, want Calls=1 RateLimited=1", TotalsSnapshot{
			Calls:       after.Calls - before.Calls,
			RateLimited: after.RateLimited - before.RateLimited,
		})
	}
}

// TestMetricsTransportWithoutRequestMetrics は ctx に計測が無くても総数だけは数えることを確認する
// （レスポンス後に非同期で走る D1 呼び出しを想定）
func TestMetricsTransportWithoutRequestMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockD1Response(nil, true, ""))
	}))
	defer server.Close()

	client := newMetricsTestClient(server)
	before := Totals()

	if _, err := client.Exec(context.Background(), "UPDATE t SET a = 1", nil); err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if delta := Totals().Calls - before.Calls; delta != 1 {
		t.Errorf("Totals.Calls delta = %d, want 1", delta)
	}
}

// TestMetricsTransportRecordsTransportError は通信エラー時にステータス 0 として記録されることを確認する
func TestMetricsTransportRecordsTransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	client := newMetricsTestClient(server)
	server.Close() // 接続できない状態にする

	ctx, m := WithMetrics(context.Background())
	if _, err := client.Query(ctx, "SELECT 1", nil); err == nil {
		t.Fatal("expected error, got nil")
	}

	got := m.Snapshot()
	if got.Queries != 1 || len(got.Statuses) != 1 || got.Statuses[0] != 0 {
		t.Errorf("Snapshot = %+v, want Queries=1 Statuses=[0]", got)
	}
}

// TestRequestMetricsConcurrent は同一リクエスト内の並行呼び出しでも取りこぼさないことを確認する（-race で検証）
func TestRequestMetricsConcurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockD1Response([]map[string]interface{}{{"id": float64(1)}}, true, ""))
	}))
	defer server.Close()

	client := newMetricsTestClient(server)
	ctx, m := WithMetrics(context.Background())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.Query(ctx, "SELECT 1", nil)
		}()
	}
	wg.Wait()

	if got := m.Snapshot().Queries; got != 20 {
		t.Errorf("Queries = %d, want 20", got)
	}
}
