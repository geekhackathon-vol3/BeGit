package repository

import (
	"context"
	"strings"
	"testing"
)

// TestFCMTokenRepository_DeleteByUserID は user_id 指定の DELETE を実行することを確認する
func TestFCMTokenRepository_DeleteByUserID(t *testing.T) {
	var gotSQL string
	var gotParams []interface{}
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			gotSQL = sql
			gotParams = params
			return 1, nil
		},
	}

	repo := NewFCMTokenRepository(mock)
	if err := repo.DeleteByUserID(context.Background(), 42); err != nil {
		t.Fatalf("DeleteByUserID() failed: %v", err)
	}
	if !strings.Contains(gotSQL, "DELETE FROM fcm_tokens") {
		t.Errorf("unexpected SQL: %s", gotSQL)
	}
	if len(gotParams) != 1 || gotParams[0] != int64(42) {
		t.Errorf("unexpected params: %v", gotParams)
	}
}
