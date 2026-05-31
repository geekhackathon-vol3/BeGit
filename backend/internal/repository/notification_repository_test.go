package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// TestNotificationRepository_Create_Conflict は UNIQUE 制約違反が ErrConstraintViolation を返すことを確認する
func TestNotificationRepository_Create_Conflict(t *testing.T) {
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			return 0, d1.ErrConstraintViolation
		},
	}

	repo := NewNotificationRepository(mock)
	_, err := repo.Create(context.Background(), &model.Notification{
		SprintID: 1,
		SentBy:   2,
		Message:  "test",
	})
	if !errors.Is(err, ErrConstraintViolation) {
		t.Errorf("expected ErrConstraintViolation, got %v", err)
	}
}

// TestWebhookRepository_InsertDelivery_Duplicate は同じ delivery_id で2回呼んだとき isDuplicate=true が返ることを確認する
func TestWebhookRepository_InsertDelivery_Duplicate(t *testing.T) {
	callCount := 0
	mock := &mockD1Client{
		execFunc: func(ctx context.Context, sql string, params []interface{}) (int64, error) {
			callCount++
			if callCount >= 2 {
				return 0, d1.ErrConstraintViolation
			}
			return 1, nil
		},
	}

	repo := NewWebhookRepository(mock)

	// 1回目: 正常
	isDuplicate1, err1 := repo.InsertDelivery(context.Background(), "delivery-uuid-123", "push")
	if err1 != nil {
		t.Fatalf("InsertDelivery() #1 failed: %v", err1)
	}
	if isDuplicate1 {
		t.Error("expected isDuplicate=false for first call")
	}

	// 2回目: UNIQUE 制約違反 → isDuplicate=true, err=nil
	isDuplicate2, err2 := repo.InsertDelivery(context.Background(), "delivery-uuid-123", "push")
	if err2 != nil {
		t.Fatalf("InsertDelivery() #2 should not return error, got: %v", err2)
	}
	if !isDuplicate2 {
		t.Error("expected isDuplicate=true for second call with same delivery_id")
	}
}
