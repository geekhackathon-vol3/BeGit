package service

import (
	"context"
	"testing"
)

// TestFCMTokenService_DeleteFCMTokens はリポジトリの DeleteByUserID を呼ぶことを確認する
func TestFCMTokenService_DeleteFCMTokens(t *testing.T) {
	var deletedUserID int64
	repo := &mockFCMTokenRepository{
		deleteByUserIDFunc: func(ctx context.Context, userID int64) error {
			deletedUserID = userID
			return nil
		},
	}

	svc := NewFCMTokenService(repo)
	if err := svc.DeleteFCMTokens(context.Background(), 7); err != nil {
		t.Fatalf("DeleteFCMTokens() failed: %v", err)
	}
	if deletedUserID != 7 {
		t.Errorf("expected userID 7, got %d", deletedUserID)
	}
}
