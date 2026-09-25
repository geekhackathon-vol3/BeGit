// Package notificationqueue は Go コンテナから Worker の Queue producer へジョブを渡す。
package notificationqueue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client interface {
	Enqueue(ctx context.Context, jobID int64) error
}

type HTTPClient struct {
	baseURL string
	secret  string
	client  *http.Client
}

func NewClient(baseURL, secret string) Client {
	return &HTTPClient{baseURL: strings.TrimRight(baseURL, "/"), secret: secret, client: &http.Client{Timeout: 8 * time.Second}}
}

func (c *HTTPClient) Enqueue(ctx context.Context, jobID int64) error {
	if c.baseURL == "" || c.secret == "" {
		return fmt.Errorf("notification queue is not configured")
	}
	body, _ := json.Marshal(map[string]int64{"job_id": jobID})
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/notification-queue", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Notification-Queue-Secret", c.secret)
		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("notification queue returned %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
		if resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return lastErr
		}
	}
	return lastErr
}
