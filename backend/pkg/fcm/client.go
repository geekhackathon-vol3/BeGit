// Package fcm は Firebase Cloud Messaging (FCM) HTTP v1 API クライアントを提供する。
// Service Account JSON から golang.org/x/oauth2/google でアクセストークンを取得して FCM に送信する。
package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2/google"
)

// Notification は FCM プッシュ通知のコンテンツ
type Notification struct {
	Title string
	Body  string
}

// Client は FCM HTTP v1 API インターフェース
type Client interface {
	// SendToTokens は tokens の各デバイスへ Push 通知を送信する
	// tokens が空の場合は何もせず正常終了する
	SendToTokens(ctx context.Context, tokens []string, notification Notification) error
}

// fcmClient は Client インターフェースの実装
type fcmClient struct {
	httpClient  *http.Client
	fcmEndpoint string // テスト用にオーバーライド可能
	projectID   string
	accessToken string // テスト用にオーバーライド可能
}

// serviceAccountJSON は Firebase Service Account JSON の最小構造
type serviceAccountJSON struct {
	ProjectID string `json:"project_id"`
}

// NewClient は Service Account JSON から FCM クライアントを作成する
// serviceAccountJSON は Firebase Service Account の JSON 文字列
func NewClient(ctx context.Context, serviceAccountJSONStr string) (Client, error) {
	// project_id を取得
	var sa serviceAccountJSON
	if err := json.Unmarshal([]byte(serviceAccountJSONStr), &sa); err != nil {
		return nil, fmt.Errorf("fcm: failed to parse service account JSON: %w", err)
	}
	if sa.ProjectID == "" {
		return nil, fmt.Errorf("fcm: project_id not found in service account JSON")
	}

	// Google Service Account 認証設定
	config, err := google.JWTConfigFromJSON(
		[]byte(serviceAccountJSONStr),
		"https://www.googleapis.com/auth/firebase.messaging",
	)
	if err != nil {
		return nil, fmt.Errorf("fcm: failed to parse JWT config: %w", err)
	}

	httpClient := config.Client(ctx)

	return &fcmClient{
		httpClient: httpClient,
		projectID:  sa.ProjectID,
	}, nil
}

// fcmMessageRequest は FCM HTTP v1 API のリクエストボディ
type fcmMessageRequest struct {
	Message struct {
		Token        string `json:"token"`
		Notification struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		} `json:"notification"`
	} `json:"message"`
}

// getEndpoint は FCM API の URL を返す
func (c *fcmClient) getEndpoint() string {
	if c.fcmEndpoint != "" {
		return strings.Replace(c.fcmEndpoint, "{project_id}", c.projectID, 1)
	}
	return fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", c.projectID)
}

// SendToTokens は tokens の各デバイスへ Push 通知を送信する
func (c *fcmClient) SendToTokens(ctx context.Context, tokens []string, notification Notification) error {
	if len(tokens) == 0 {
		return nil
	}

	// ベストエフォートで全トークンに送信を試みる
	var lastErr error
	successCount := 0
	for _, token := range tokens {
		if err := c.sendToToken(ctx, token, notification); err != nil {
			// 個別トークンの失敗はログに記録するが続行する
			lastErr = err
			continue
		}
		successCount++
	}

	// 全て失敗した場合のみエラーを返す
	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("fcm: all tokens failed, last error: %w", lastErr)
	}

	return nil
}

// sendToToken は1つのデバイストークンへ Push 通知を送信する
func (c *fcmClient) sendToToken(ctx context.Context, token string, notification Notification) error {
	var req fcmMessageRequest
	req.Message.Token = token
	req.Message.Notification.Title = notification.Title
	req.Message.Notification.Body = notification.Body

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("fcm: failed to marshal request: %w", err)
	}

	endpoint := c.getEndpoint()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("fcm: failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// アクセストークンを手動で設定する場合（テスト用）
	if c.accessToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("fcm: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fcm: unexpected status %d", resp.StatusCode)
	}

	return nil
}
