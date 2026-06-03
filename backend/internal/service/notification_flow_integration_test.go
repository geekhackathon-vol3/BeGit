package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
)

// pushPayload は最小の push Webhook ペイロードを構築する
func pushPayload(repoFullName, senderLogin string) []byte {
	p := map[string]interface{}{
		"repository": map[string]interface{}{"full_name": repoFullName},
		"sender":     map[string]interface{}{"login": senderLogin},
		"ref":        "refs/heads/main",
		"commits":    []interface{}{map[string]interface{}{"message": "feat: x"}},
	}
	b, _ := json.Marshal(p)
	return b
}

// TestIntegration_Webhook_To_NiceWork_OwnerOnly_Idempotent は
// Webhook(push) → ② 発火（本人のみ送信・冪等）を webhook_service + 実 nicework_service で検証する。
func TestIntegration_Webhook_To_NiceWork_OwnerOnly_Idempotent(t *testing.T) {
	// 実依存（モック repo）
	userRepo := &mockUserRepoForNiceWork{
		getByLoginFunc: func(ctx context.Context, login string) (*model.User, error) {
			return &model.User{ID: 10, GitHubLogin: login}, nil
		},
	}
	groupRepo := &mockGroupRepository{
		getByRepoFullNameFunc: func(ctx context.Context, repoFullName string) (*model.Group, error) {
			return &model.Group{ID: 12, RepoFullName: repoFullName, SprintDurationDays: 7}, nil
		},
		isMemberFunc: func(ctx context.Context, groupID, userID int64) (bool, error) { return true, nil },
	}
	sprintRepo := &mockSprintRepository{
		getCurrentFunc: func(ctx context.Context, groupID int64) (*model.Sprint, error) {
			return &model.Sprint{ID: 7, GroupID: groupID}, nil
		},
	}
	notifRepo := &mockNotificationRepository{
		getLatestInSprintBeforeFunc: func(ctx context.Context, sprintID int64, before time.Time) (*model.Notification, error) {
			return &model.Notification{ID: 345, SprintID: 7, SentAt: time.Now().Add(-20 * time.Minute)}, nil
		},
	}
	// 冪等: 1回目は作成、2回目は UNIQUE 違反
	draftCount := 0
	postRepo := &mockPostRepository{
		createDraftFunc: func(ctx context.Context, post *model.Post) (*model.Post, error) {
			draftCount++
			if draftCount >= 2 {
				return nil, repository.ErrConstraintViolation
			}
			post.ID = 890
			return post, nil
		},
	}
	ft := &mockFCMTokenRepository{
		getTokensByUserIDFunc:  func(ctx context.Context, userID int64) ([]string, error) { return []string{"author-tok"}, nil },
		getTokensByGroupIDFunc: func(ctx context.Context, groupID int64) ([]string, error) { return []string{"a", "b", "c"}, nil },
	}
	fc := &fakeFCMClient{}

	niceWork := NewNiceWorkService(userRepo, groupRepo, sprintRepo, notifRepo, postRepo, ft, fc)
	webhook := NewWebhookServiceWithNiceWork(groupRepo, sprintRepo, niceWork)

	req := WebhookRequest{DeliveryID: "d1", EventType: "push", Payload: pushPayload("o/r", "octocat")}

	// 1回目: ② 発火 → 本人のみ（author-tok 1件）へ nice_work
	if err := webhook.ProcessWebhook(context.Background(), req); err != nil {
		t.Fatalf("ProcessWebhook #1 failed: %v", err)
	}
	if len(fc.withDataCalls) != 1 {
		t.Fatalf("expected 1 nice_work send, got %d", len(fc.withDataCalls))
	}
	c := fc.withDataCalls[0]
	if c.data["type"] != "nice_work" || len(c.tokens) != 1 || c.tokens[0] != "author-tok" {
		t.Errorf("expected nice_work to author only, got data=%v tokens=%v", c.data, c.tokens)
	}

	// 2回目（同一チャレンジ）: 冪等 skip（再送信なし）
	req2 := WebhookRequest{DeliveryID: "d2", EventType: "push", Payload: pushPayload("o/r", "octocat")}
	if err := webhook.ProcessWebhook(context.Background(), req2); err != nil {
		t.Fatalf("ProcessWebhook #2 failed: %v", err)
	}
	if len(fc.withDataCalls) != 1 {
		t.Errorf("expected no additional send on idempotent re-fire, got %d", len(fc.withDataCalls))
	}
}

// TestIntegration_BeGitTime_NonCoexistence_409 は ① のアクティブ通知存在時に 409 を返すことを検証する。
func TestIntegration_BeGitTime_NonCoexistence_409(t *testing.T) {
	sprintRepo := &mockSprintRepository{
		getOrCreateFunc: func(ctx context.Context, groupID int64, durationDays int) (*model.Sprint, error) {
			return &model.Sprint{ID: 7, GroupID: groupID}, nil
		},
	}
	notifRepo := &mockNotificationRepository{
		// 時間的非共存は CreateIfNoActive が原子的に ErrConstraintViolation を返して表現する
		createIfNoActiveFunc: func(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
			return nil, repository.ErrConstraintViolation
		},
	}
	fc := &fakeFCMClient{}
	svc := NewNotificationService(sprintRepo, notifRepo, &mockFCMTokenRepository{}, fc)

	_, err := svc.SendNotification(context.Background(), 12, 2)
	if err != ErrConflict {
		t.Errorf("expected ErrConflict (409) for active challenge, got %v", err)
	}
	if len(fc.withDataCalls) != 0 {
		t.Error("expected no begit_time send on conflict")
	}
}

// TestIntegration_Reaction_SelfSuppression は ⑦ の自己抑制（自己操作で非送信）を検証する。
func TestIntegration_Reaction_SelfSuppression(t *testing.T) {
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 12, UserID: 5}, nil
		},
	}
	fc := &fakeFCMClient{}
	svc := NewReactionServiceWithNotifications(&mockReactionRepository{}, postRepo, &mockUserByID{}, &mockFCMTokenRepository{}, fc)

	// 投稿者 5 が自分の投稿にリアクション
	if _, err := svc.AddReaction(context.Background(), 12, 890, 5, "heart"); err != nil {
		t.Fatalf("AddReaction failed: %v", err)
	}
	if len(fc.withDataCalls) != 0 {
		t.Error("expected no self-reaction notification")
	}
}

// TestIntegration_DraftConfirm_FeedTransition は draft 確定で feed 表示に遷移することを検証する。
// 確定前: GetDraft 可能 / feed 非表示。確定後: feed 表示。
func TestIntegration_DraftConfirm_FeedTransition(t *testing.T) {
	isDraft := true
	postRepo := &mockPostRepository{
		getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
			return &model.Post{ID: postID, GroupID: 12, UserID: 7, IsDraft: isDraft}, nil
		},
		confirmDraftFunc: func(ctx context.Context, postID int64) error {
			isDraft = false
			return nil
		},
		// feed 一覧は is_draft=0 のみ返す（repository 実装の責務を模す）
		listByGroupIDFunc: func(ctx context.Context, groupID int64) ([]model.Post, error) {
			if isDraft {
				return []model.Post{}, nil // draft 中はフィードに出ない
			}
			return []model.Post{{ID: 890, GroupID: 12, UserID: 7, IsDraft: false}}, nil
		},
		hasPostedInSprintFunc: func(ctx context.Context, userID, sprintID int64) (bool, error) { return true, nil },
	}
	sprintRepo := &mockSprintRepository{
		getCurrentFunc: func(ctx context.Context, groupID int64) (*model.Sprint, error) {
			return &model.Sprint{ID: 7, GroupID: groupID}, nil
		},
	}
	svc := NewPostService(nil, sprintRepo, postRepo, &mockGroupRepository{}, nil, nil)

	// 確定前: draft 取得可能
	if _, err := svc.GetDraft(context.Background(), 12, 890, 7); err != nil {
		t.Fatalf("GetDraft before confirm failed: %v", err)
	}
	// 確定前: feed 非表示
	feeds, _ := svc.ListPosts(context.Background(), 12, 7)
	if len(feeds) != 0 {
		t.Errorf("expected draft hidden from feed before confirm, got %d", len(feeds))
	}

	// 確定
	if _, err := svc.ConfirmPost(context.Background(), ConfirmPostRequest{}, 12, 890, 7); err != nil {
		t.Fatalf("ConfirmPost failed: %v", err)
	}

	// 確定後: feed 表示
	feeds, _ = svc.ListPosts(context.Background(), 12, 7)
	if len(feeds) != 1 {
		t.Errorf("expected post visible in feed after confirm, got %d", len(feeds))
	}
}
