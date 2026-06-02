package r2

import (
	"context"
	"time"
)

// stubClient は DEV_MODE 用のスタブ実装。
// 実 R2 認証情報なしで auth/post/photo の各フローを動作させるため、
// PutObject は no-op、PresignGetURL はダミー URL を返す。
type stubClient struct{}

// NewStubClient は dev 用のスタブ R2 クライアントを作成する
func NewStubClient() Client {
	return &stubClient{}
}

// PutObject は何もせず成功を返す
func (s *stubClient) PutObject(ctx context.Context, key, contentType string, body []byte) error {
	return nil
}

// PresignGetURL はダミーの取得 URL を返す
func (s *stubClient) PresignGetURL(key string, ttl time.Duration) (string, error) {
	return "https://stub.r2.local/" + key + "?stub=1", nil
}

// DeleteObject は何もせず成功を返す
func (s *stubClient) DeleteObject(ctx context.Context, key string) error {
	return nil
}
