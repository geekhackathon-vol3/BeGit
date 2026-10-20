package d1

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// 計測用（#126 Before 計測）。D1 REST API への HTTP 呼び出しを数え、
// リクエスト単位の内訳と、プロセス全体の総数を取れるようにする。
// d1.Client インターフェースのデコレーターでは HTTP ステータスが取れないため、
// http.RoundTripper の層で計測する。

// RequestMetrics は 1 リクエスト分の D1 呼び出しの計測値
type RequestMetrics struct {
	mu       sync.Mutex
	queries  int
	duration time.Duration
	statuses []int
}

// RequestMetricsSnapshot は RequestMetrics の読み取り用コピー
type RequestMetricsSnapshot struct {
	Queries  int
	Duration time.Duration
	// Statuses は呼び出し順の HTTP ステータス。通信エラーは 0
	Statuses []int
}

func (m *RequestMetrics) record(status int, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queries++
	m.duration += d
	m.statuses = append(m.statuses, status)
}

// Snapshot は現時点の計測値を返す
func (m *RequestMetrics) Snapshot() RequestMetricsSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return RequestMetricsSnapshot{
		Queries:  m.queries,
		Duration: m.duration,
		Statuses: append([]int(nil), m.statuses...),
	}
}

type metricsKey struct{}

// WithMetrics は ctx にリクエスト単位の計測を仕込む
func WithMetrics(ctx context.Context) (context.Context, *RequestMetrics) {
	m := &RequestMetrics{}
	return context.WithValue(ctx, metricsKey{}, m), m
}

// TotalsSnapshot はプロセス起動以降の D1 呼び出しの総数
type TotalsSnapshot struct {
	Calls       int64 `json:"calls"`
	RateLimited int64 `json:"rate_limited"`
}

var (
	totalCalls       atomic.Int64
	totalRateLimited atomic.Int64
)

// Totals はプロセス起動以降の D1 呼び出しの総数を返す。
// レスポンス後に非同期で走る呼び出しも含むため、上限の枠の消費量の把握に使う
func Totals() TotalsSnapshot {
	return TotalsSnapshot{Calls: totalCalls.Load(), RateLimited: totalRateLimited.Load()}
}

// metricsTransport は D1 REST API への呼び出しを計測する http.RoundTripper
type metricsTransport struct {
	base http.RoundTripper
}

// NewMetricsTransport は base を計測付きでラップする。base が nil なら http.DefaultTransport を使う
// （計測なしの d1Client と同じ接続設定にするため）
func NewMetricsTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &metricsTransport{base: base}
}

func (t *metricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	elapsed := time.Since(start)

	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	totalCalls.Add(1)
	if status == http.StatusTooManyRequests {
		totalRateLimited.Add(1)
	}
	if m, ok := req.Context().Value(metricsKey{}).(*RequestMetrics); ok {
		m.record(status, elapsed)
	}
	return resp, err
}
