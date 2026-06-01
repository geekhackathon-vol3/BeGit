package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
)

// mockSprintRepository はテスト用のスプリントリポジトリモック
type mockSprintRepository struct {
	getOrCreateFunc    func(ctx context.Context, groupID int64, durationDays int) (*model.Sprint, error)
	getCurrentFunc     func(ctx context.Context, groupID int64) (*model.Sprint, error)
	getByIDFunc        func(ctx context.Context, sprintID int64) (*model.Sprint, error)
}

func (m *mockSprintRepository) GetOrCreateCurrentSprint(ctx context.Context, groupID int64, durationDays int) (*model.Sprint, error) {
	if m.getOrCreateFunc != nil {
		return m.getOrCreateFunc(ctx, groupID, durationDays)
	}
	return &model.Sprint{ID: 1, GroupID: groupID}, nil
}

func (m *mockSprintRepository) GetCurrentSprint(ctx context.Context, groupID int64) (*model.Sprint, error) {
	if m.getCurrentFunc != nil {
		return m.getCurrentFunc(ctx, groupID)
	}
	return &model.Sprint{ID: 1, GroupID: groupID}, nil
}

func (m *mockSprintRepository) GetByID(ctx context.Context, sprintID int64) (*model.Sprint, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, sprintID)
	}
	return &model.Sprint{ID: sprintID, GroupID: 1}, nil
}

// mockNotificationRepository はテスト用の通知リポジトリモック
type mockNotificationRepository struct {
	createFunc  func(ctx context.Context, notif *model.Notification) (*model.Notification, error)
	getByIDFunc func(ctx context.Context, notifID int64) (*model.Notification, error)
}

func (m *mockNotificationRepository) Create(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, notif)
	}
	notif.ID = 1
	notif.SentAt = time.Now()
	return notif, nil
}

func (m *mockNotificationRepository) GetByID(ctx context.Context, notifID int64) (*model.Notification, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, notifID)
	}
	return nil, repository.ErrNotFound
}

// mockPostRepository はテスト用の投稿リポジトリモック
type mockPostRepository struct {
	createFunc                 func(ctx context.Context, post *model.Post) (*model.Post, error)
	listByGroupIDFunc          func(ctx context.Context, groupID int64) ([]model.Post, error)
	hasPostedInSprintFunc      func(ctx context.Context, userID, sprintID int64) (bool, error)
	getByUserAndNotifFunc      func(ctx context.Context, userID, notifID int64) (*model.Post, error)
	getByIDFunc                func(ctx context.Context, postID int64) (*model.Post, error)
}

func (m *mockPostRepository) Create(ctx context.Context, post *model.Post) (*model.Post, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, post)
	}
	post.ID = 1
	return post, nil
}

func (m *mockPostRepository) ListByGroupID(ctx context.Context, groupID int64) ([]model.Post, error) {
	if m.listByGroupIDFunc != nil {
		return m.listByGroupIDFunc(ctx, groupID)
	}
	return []model.Post{}, nil
}

func (m *mockPostRepository) HasPostedInSprint(ctx context.Context, userID, sprintID int64) (bool, error) {
	if m.hasPostedInSprintFunc != nil {
		return m.hasPostedInSprintFunc(ctx, userID, sprintID)
	}
	return false, nil
}

func (m *mockPostRepository) GetByUserAndNotification(ctx context.Context, userID, notifID int64) (*model.Post, error) {
	if m.getByUserAndNotifFunc != nil {
		return m.getByUserAndNotifFunc(ctx, userID, notifID)
	}
	return nil, repository.ErrNotFound
}

func (m *mockPostRepository) GetByID(ctx context.Context, postID int64) (*model.Post, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, postID)
	}
	return nil, repository.ErrNotFound
}

// mockFCMTokenRepository はテスト用の FCM トークンリポジトリモック
type mockFCMTokenRepository struct {
	upsertFunc             func(ctx context.Context, userID int64, token string) error
	getTokensByGroupIDFunc func(ctx context.Context, groupID int64) ([]string, error)
}

func (m *mockFCMTokenRepository) Upsert(ctx context.Context, userID int64, token string) error {
	if m.upsertFunc != nil {
		return m.upsertFunc(ctx, userID, token)
	}
	return nil
}

func (m *mockFCMTokenRepository) GetTokensByGroupID(ctx context.Context, groupID int64) ([]string, error) {
	if m.getTokensByGroupIDFunc != nil {
		return m.getTokensByGroupIDFunc(ctx, groupID)
	}
	return []string{}, nil
}

// mockFCMClient はテスト用の FCM クライアントモック
type mockFCMClient struct {
	sendToTokensFunc func(ctx context.Context, tokens []string, notification fcmNotification) error
}

// fcmNotification はモックで使う通知型（pkg/fcm の Notification と分離）
type fcmNotification struct {
	Title string
	Body  string
}

// TestNotificationService_SendNotification_Conflict は同一スプリントで2回目の通知発行を試みると ErrConflict が返ることを確認する
func TestNotificationService_SendNotification_Conflict(t *testing.T) {
	sprintRepo := &mockSprintRepository{}
	notifRepo := &mockNotificationRepository{
		createFunc: func(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
			return nil, repository.ErrConstraintViolation
		},
	}
	fcmTokenRepo := &mockFCMTokenRepository{}

	svc := NewNotificationService(sprintRepo, notifRepo, fcmTokenRepo, nil)

	_, err := svc.SendNotification(context.Background(), 1, 2)
	if !errors.Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

// TestNotificationService_GetStatus_OnTime は通知後59分の投稿が "On Time" になることを確認する
func TestNotificationService_GetStatus_OnTime(t *testing.T) {
	sentAt := time.Now().Add(-59 * time.Minute)
	postedAt := time.Now().Add(-1 * time.Minute) // sentAt から 58分後

	notifRepo := &mockNotificationRepository{
		getByIDFunc: func(ctx context.Context, notifID int64) (*model.Notification, error) {
			return &model.Notification{
				ID:       1,
				SprintID: 1,
				SentBy:   10,
				SentAt:   sentAt,
			}, nil
		},
	}

	groupRepo := &mockGroupRepository{
		getMembersFunc: func(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
			return []model.GroupMember{
				{UserID: 10, Login: "user1", AvatarURL: "https://example.com/1.png"},
			}, nil
		},
	}

	postRepo := &mockPostRepository{
		getByUserAndNotifFunc: func(ctx context.Context, userID, notifID int64) (*model.Post, error) {
			return &model.Post{
				ID:        1,
				UserID:    10,
				CreatedAt: postedAt,
			}, nil
		},
	}

	svc := NewNotificationServiceWithGroupRepo(&mockSprintRepository{}, notifRepo, groupRepo, postRepo)

	status, err := svc.GetNotificationStatus(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("GetNotificationStatus() failed: %v", err)
	}
	if len(status.Members) != 1 {
		t.Fatalf("expected 1 member status, got %d", len(status.Members))
	}
	if status.Members[0].Status != "On Time" {
		t.Errorf("expected status=On Time (59 min), got %s", status.Members[0].Status)
	}
}

// TestNotificationService_GetStatus_Late は通知後61分の投稿が "Late" になることを確認する
func TestNotificationService_GetStatus_Late(t *testing.T) {
	sentAt := time.Now().Add(-61 * time.Minute)
	postedAt := time.Now().Add(-1 * time.Minute) // sentAt から 60分後

	notifRepo := &mockNotificationRepository{
		getByIDFunc: func(ctx context.Context, notifID int64) (*model.Notification, error) {
			return &model.Notification{
				ID:       1,
				SprintID: 1,
				SentBy:   10,
				SentAt:   sentAt,
			}, nil
		},
	}

	groupRepo := &mockGroupRepository{
		getMembersFunc: func(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
			return []model.GroupMember{
				{UserID: 10, Login: "user1", AvatarURL: "https://example.com/1.png"},
			}, nil
		},
	}

	postRepo := &mockPostRepository{
		getByUserAndNotifFunc: func(ctx context.Context, userID, notifID int64) (*model.Post, error) {
			return &model.Post{
				ID:        1,
				UserID:    10,
				CreatedAt: postedAt,
			}, nil
		},
	}

	svc := NewNotificationServiceWithGroupRepo(&mockSprintRepository{}, notifRepo, groupRepo, postRepo)

	status, err := svc.GetNotificationStatus(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("GetNotificationStatus() failed: %v", err)
	}
	if status.Members[0].Status != "Late" {
		t.Errorf("expected status=Late (61 min), got %s", status.Members[0].Status)
	}
}

// TestNotificationService_GetStatus_Missed は投稿なしの場合が "Missed" になることを確認する
func TestNotificationService_GetStatus_Missed(t *testing.T) {
	sentAt := time.Now().Add(-30 * time.Minute)

	notifRepo := &mockNotificationRepository{
		getByIDFunc: func(ctx context.Context, notifID int64) (*model.Notification, error) {
			return &model.Notification{
				ID:       1,
				SprintID: 1,
				SentBy:   10,
				SentAt:   sentAt,
			}, nil
		},
	}

	groupRepo := &mockGroupRepository{
		getMembersFunc: func(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
			return []model.GroupMember{
				{UserID: 10, Login: "user1", AvatarURL: "https://example.com/1.png"},
			}, nil
		},
	}

	postRepo := &mockPostRepository{
		getByUserAndNotifFunc: func(ctx context.Context, userID, notifID int64) (*model.Post, error) {
			return nil, repository.ErrNotFound
		},
	}

	svc := NewNotificationServiceWithGroupRepo(&mockSprintRepository{}, notifRepo, groupRepo, postRepo)

	status, err := svc.GetNotificationStatus(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("GetNotificationStatus() failed: %v", err)
	}
	if status.Members[0].Status != "Missed" {
		t.Errorf("expected status=Missed (no post), got %s", status.Members[0].Status)
	}
}
