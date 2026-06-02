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

// mockNiceWorkService は ② 委譲の呼び出しを記録するモック
type mockNiceWorkService struct {
	calls []niceWorkCall
}

type niceWorkCall struct {
	groupID     int64
	senderLogin string
	postType    string
	detected    ActivityData
}

func (m *mockNiceWorkService) HandleActivity(ctx context.Context, groupID int64, senderLogin, postType string, detected ActivityData) error {
	m.calls = append(m.calls, niceWorkCall{groupID, senderLogin, postType, detected})
	return nil
}

func webhookGroupRepo() *mockGroupRepository {
	return &mockGroupRepository{
		getByRepoFullNameFunc: func(ctx context.Context, repoFullName string) (*model.Group, error) {
			return &model.Group{ID: 12, RepoFullName: repoFullName, SprintDurationDays: 7}, nil
		},
	}
}

// TestWebhook_DelegatesNiceWork_ForThreeEvents は push/issues/pr_review で ② が駆動されることを確認する
func TestWebhook_DelegatesNiceWork_ForThreeEvents(t *testing.T) {
	cases := []struct {
		event    string
		payload  map[string]interface{}
		wantType string
	}{
		{"push", map[string]interface{}{"repository": map[string]interface{}{"full_name": "o/r"}, "sender": map[string]interface{}{"login": "octocat"}, "ref": "refs/heads/main", "commits": []interface{}{map[string]interface{}{"message": "fix"}}}, "commit"},
		{"issues", map[string]interface{}{"action": "opened", "repository": map[string]interface{}{"full_name": "o/r"}, "sender": map[string]interface{}{"login": "octocat"}}, "issue"},
		{"pull_request_review", map[string]interface{}{"repository": map[string]interface{}{"full_name": "o/r"}, "sender": map[string]interface{}{"login": "octocat"}}, "review"},
	}

	for _, tc := range cases {
		nw := &mockNiceWorkService{}
		svc := NewWebhookServiceWithNiceWork(webhookGroupRepo(), &mockSprintRepository{}, nw)
		data, _ := json.Marshal(tc.payload)
		if err := svc.ProcessWebhook(context.Background(), WebhookRequest{DeliveryID: "d", EventType: tc.event, Payload: data}); err != nil {
			t.Fatalf("ProcessWebhook(%s) failed: %v", tc.event, err)
		}
		if len(nw.calls) != 1 {
			t.Fatalf("event %s: expected 1 nicework delegation, got %d", tc.event, len(nw.calls))
		}
		c := nw.calls[0]
		if c.groupID != 12 || c.senderLogin != "octocat" || c.postType != tc.wantType {
			t.Errorf("event %s: unexpected delegation %+v", tc.event, c)
		}
	}
}

// TestWebhook_IssuesNonOpened_NoDelegation は issues(action!=opened) で委譲しないことを確認する
func TestWebhook_IssuesNonOpened_NoDelegation(t *testing.T) {
	nw := &mockNiceWorkService{}
	svc := NewWebhookServiceWithNiceWork(webhookGroupRepo(), &mockSprintRepository{}, nw)
	payload := map[string]interface{}{"action": "closed", "repository": map[string]interface{}{"full_name": "o/r"}, "sender": map[string]interface{}{"login": "octocat"}}
	data, _ := json.Marshal(payload)
	if err := svc.ProcessWebhook(context.Background(), WebhookRequest{DeliveryID: "d", EventType: "issues", Payload: data}); err != nil {
		t.Fatalf("ProcessWebhook failed: %v", err)
	}
	if len(nw.calls) != 0 {
		t.Error("expected no delegation for non-opened issues")
	}
}

// TestWebhook_GroupNotFound_NoDelegation は対応グループ無しで委譲しないことを確認する
func TestWebhook_GroupNotFound_NoDelegation(t *testing.T) {
	nw := &mockNiceWorkService{}
	groupRepo := &mockGroupRepository{
		getByRepoFullNameFunc: func(ctx context.Context, repoFullName string) (*model.Group, error) {
			return nil, repository.ErrNotFound
		},
	}
	svc := NewWebhookServiceWithNiceWork(groupRepo, &mockSprintRepository{}, nw)
	payload := map[string]interface{}{"repository": map[string]interface{}{"full_name": "x/y"}, "sender": map[string]interface{}{"login": "octocat"}}
	data, _ := json.Marshal(payload)
	if err := svc.ProcessWebhook(context.Background(), WebhookRequest{DeliveryID: "d", EventType: "push", Payload: data}); err != nil {
		t.Fatalf("ProcessWebhook failed: %v", err)
	}
	if len(nw.calls) != 0 {
		t.Error("expected no delegation when group not found")
	}
}

// Ensure errors package is used
var _ = errors.New
