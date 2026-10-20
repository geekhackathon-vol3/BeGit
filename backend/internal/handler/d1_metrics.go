package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/pkg/d1"
)

// 計測用（#126 Before 計測）。D1 呼び出しの内訳をレスポンスヘッダーで返す。
// コンテナの標準出力は Workers Logs に出ないため、k6 がヘッダーをそのまま集計する。
const (
	headerD1Queries = "X-D1-Queries"
	headerD1TimeMs  = "X-D1-Time-Ms"
	headerD1Status  = "X-D1-Status"
)

// D1Metrics はリクエストごとの D1 呼び出しを計測し、X-D1-* ヘッダーとして返すミドルウェア。
// ヘッダーはレスポンスの書き出し直前の値で確定するため、レスポンス後の非同期呼び出しは含まない
// （そちらは D1MetricsTotals で把握する）。
func D1Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, m := d1.WithMetrics(c.Request.Context())
		c.Request = c.Request.WithContext(ctx)

		w := &d1MetricsWriter{ResponseWriter: c.Writer, metrics: m}
		c.Writer = w
		c.Next()
		// ボディを書かないレスポンス（c.Status のみ）は gin が最後にヘッダーを書き出すため、ここで載せる
		w.setHeaders()
	}
}

// D1MetricsTotals はプロセス起動以降の D1 呼び出しの総数を返す。
// k6 は各回の前後でこれを取得し、差分を上限の枠の消費量として使う。
func D1MetricsTotals(c *gin.Context) {
	c.JSON(http.StatusOK, d1.Totals())
}

// d1MetricsWriter はヘッダーが書き出される直前に X-D1-* を載せる ResponseWriter
type d1MetricsWriter struct {
	gin.ResponseWriter
	metrics *d1.RequestMetrics
	done    bool
}

func (w *d1MetricsWriter) setHeaders() {
	if w.done {
		return
	}
	w.done = true

	s := w.metrics.Snapshot()
	statuses := make([]string, len(s.Statuses))
	for i, st := range s.Statuses {
		statuses[i] = strconv.Itoa(st)
	}
	h := w.Header()
	h.Set(headerD1Queries, strconv.Itoa(s.Queries))
	h.Set(headerD1TimeMs, strconv.FormatFloat(float64(s.Duration)/float64(time.Millisecond), 'f', 1, 64))
	h.Set(headerD1Status, strings.Join(statuses, ","))
}

func (w *d1MetricsWriter) WriteHeaderNow() {
	w.setHeaders()
	w.ResponseWriter.WriteHeaderNow()
}

func (w *d1MetricsWriter) Write(data []byte) (int, error) {
	w.setHeaders()
	return w.ResponseWriter.Write(data)
}

func (w *d1MetricsWriter) WriteString(s string) (int, error) {
	w.setHeaders()
	return w.ResponseWriter.WriteString(s)
}
