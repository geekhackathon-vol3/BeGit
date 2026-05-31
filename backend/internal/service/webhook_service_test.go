package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
)

// TestWebhookService_ProcessWebhook_PushEvent は push イベントが正常完了することを確認する
func TestWebhookService_ProcessWebhook_PushEvent(t *testing.T) {
	groupRepo := &mockGroupRepository{
		getByRepoFullNameFunc: func(ctx context.Context, repoFullName string) (*model.Group, error) {
			return nil, repository.ErrNotFound // グループが見つからない場合も正常終了
		},
	}
	sprintRepo := &mockSprintRepository{}

	svc := NewWebhookService(groupRepo, sprintRepo)

	payload := map[string]interface{}{
		"repository": map[string]interface{}{
			"full_name": "owner/repo",
		},
	}
	data, _ := json.Marshal(payload)

	err := svc.ProcessWebhook(context.Background(), WebhookRequest{
		DeliveryID: "test-delivery-id",
		EventType:  "push",
		Payload:    data,
	})
	if err != nil {
		t.Errorf("ProcessWebhook() should not return error for push event, got: %v", err)
	}
}

// TestWebhookService_ProcessWebhook_GroupNotFound は対応グループが見つからない場合に正常終了することを確認する
func TestWebhookService_ProcessWebhook_GroupNotFound(t *testing.T) {
	groupRepo := &mockGroupRepository{
		getByRepoFullNameFunc: func(ctx context.Context, repoFullName string) (*model.Group, error) {
			return nil, repository.ErrNotFound
		},
	}
	sprintRepo := &mockSprintRepository{}

	svc := NewWebhookService(groupRepo, sprintRepo)

	payload := map[string]interface{}{
		"repository": map[string]interface{}{
			"full_name": "unknown/repo",
		},
	}
	data, _ := json.Marshal(payload)

	err := svc.ProcessWebhook(context.Background(), WebhookRequest{
		DeliveryID: "test-delivery-id",
		EventType:  "push",
		Payload:    data,
	})
	if err != nil {
		t.Errorf("ProcessWebhook() should return nil when group not found, got: %v", err)
	}
}

// Ensure errors package is used
var _ = errors.New
