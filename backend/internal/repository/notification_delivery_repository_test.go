package repository

import (
	"context"
	"testing"

	"github.com/irj0927/begit/pkg/d1"
)

// TestNotificationDeliveryRepository_MarkSent_New は初回 INSERT で sent=false（=新規送信対象）を返すことを確認する
func TestNotificationDeliveryRepository_MarkSent_New(t *testing.T) {
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			return 1, nil
		},
	}

	repo := NewNotificationDeliveryRepository(mock)
	alreadySent, err := repo.MarkSent(context.Background(), "challenge_end", 345)
	if err != nil {
		t.Fatalf("MarkSent() failed: %v", err)
	}
	if alreadySent {
		t.Error("expected alreadySent=false for first INSERT")
	}
}

// TestNotificationDeliveryRepository_MarkSent_Duplicate は同一 (kind, ref_id) の2回目 INSERT が送信済みと判定されることを確認する
func TestNotificationDeliveryRepository_MarkSent_Duplicate(t *testing.T) {
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			return 0, d1.ErrConstraintViolation
		},
	}

	repo := NewNotificationDeliveryRepository(mock)
	alreadySent, err := repo.MarkSent(context.Background(), "challenge_end", 345)
	if err != nil {
		t.Fatalf("MarkSent() should not error on UNIQUE violation, got: %v", err)
	}
	if !alreadySent {
		t.Error("expected alreadySent=true for duplicate INSERT")
	}
}
