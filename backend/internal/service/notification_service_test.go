package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/fcm"
)

// mockSprintRepository はテスト用のスプリントリポジトリモック
type mockSprintRepository struct {
	getOrCreateFunc     func(ctx context.Context, groupID int64, durationDays int) (*model.Sprint, error)
	getCurrentFunc      func(ctx context.Context, groupID int64) (*model.Sprint, error)
	getByIDFunc         func(ctx context.Context, sprintID int64) (*model.Sprint, error)
	listReminderDueFunc func(ctx context.Context) ([]model.Sprint, error)
	listEndedFunc       func(ctx context.Context) ([]model.Sprint, error)
	listActiveFunc      func(ctx context.Context) ([]model.Sprint, error)
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

func (m *mockSprintRepository) ListReminderDue(ctx context.Context) ([]model.Sprint, error) {
	if m.listReminderDueFunc != nil {
		return m.listReminderDueFunc(ctx)
	}
	return nil, nil
}

func (m *mockSprintRepository) ListEnded(ctx context.Context) ([]model.Sprint, error) {
	if m.listEndedFunc != nil {
		return m.listEndedFunc(ctx)
	}
	return nil, nil
}

func (m *mockSprintRepository) ListActive(ctx context.Context) ([]model.Sprint, error) {
	if m.listActiveFunc != nil {
		return m.listActiveFunc(ctx)
	}
	return nil, nil
}

// mockNotificationRepository はテスト用の通知リポジトリモック
type mockNotificationRepository struct {
	createFunc                  func(ctx context.Context, notif *model.Notification) (*model.Notification, error)
	getByIDFunc                 func(ctx context.Context, notifID int64) (*model.Notification, error)
	getLatestInSprintBeforeFunc func(ctx context.Context, sprintID int64, before time.Time) (*model.Notification, error)
	hasActiveInSprintFunc       func(ctx context.Context, sprintID int64) (bool, error)
	listChallengeEndDueFunc     func(ctx context.Context) ([]model.Notification, error)
	listBySprintIDFunc          func(ctx context.Context, sprintID int64) ([]model.Notification, error)
}

func (m *mockNotificationRepository) ListChallengeEndDue(ctx context.Context) ([]model.Notification, error) {
	if m.listChallengeEndDueFunc != nil {
		return m.listChallengeEndDueFunc(ctx)
	}
	return nil, nil
}

func (m *mockNotificationRepository) ListBySprintID(ctx context.Context, sprintID int64) ([]model.Notification, error) {
	if m.listBySprintIDFunc != nil {
		return m.listBySprintIDFunc(ctx, sprintID)
	}
	return nil, nil
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

func (m *mockNotificationRepository) GetLatestInSprintBefore(ctx context.Context, sprintID int64, before time.Time) (*model.Notification, error) {
	if m.getLatestInSprintBeforeFunc != nil {
		return m.getLatestInSprintBeforeFunc(ctx, sprintID, before)
	}
	return nil, repository.ErrNotFound
}

func (m *mockNotificationRepository) HasActiveInSprint(ctx context.Context, sprintID int64) (bool, error) {
	if m.hasActiveInSprintFunc != nil {
		return m.hasActiveInSprintFunc(ctx, sprintID)
	}
	return false, nil
}

// mockPostRepository はテスト用の投稿リポジトリモック
type mockPostRepository struct {
	createFunc            func(ctx context.Context, post *model.Post) (*model.Post, error)
	listByGroupIDFunc     func(ctx context.Context, groupID int64) ([]model.Post, error)
	hasPostedInSprintFunc func(ctx context.Context, userID, sprintID int64) (bool, error)
	getByUserAndNotifFunc func(ctx context.Context, userID, notifID int64) (*model.Post, error)
	getByIDFunc           func(ctx context.Context, postID int64) (*model.Post, error)
	createDraftFunc       func(ctx context.Context, post *model.Post) (*model.Post, error)
	confirmDraftFunc      func(ctx context.Context, postID int64) error
	createMissedFunc      func(ctx context.Context, notifID, userID, groupID int64) error
	updateBodyFunc        func(ctx context.Context, postID int64, body string) error
}

func (m *mockPostRepository) CreateMissed(ctx context.Context, notifID, userID, groupID int64) error {
	if m.createMissedFunc != nil {
		return m.createMissedFunc(ctx, notifID, userID, groupID)
	}
	return nil
}

func (m *mockPostRepository) UpdateBody(ctx context.Context, postID int64, body string) error {
	if m.updateBodyFunc != nil {
		return m.updateBodyFunc(ctx, postID, body)
	}
	return nil
}

func (m *mockPostRepository) CreateDraft(ctx context.Context, post *model.Post) (*model.Post, error) {
	if m.createDraftFunc != nil {
		return m.createDraftFunc(ctx, post)
	}
	post.ID = 1
	post.IsDraft = true
	return post, nil
}

func (m *mockPostRepository) ConfirmDraft(ctx context.Context, postID int64) error {
	if m.confirmDraftFunc != nil {
		return m.confirmDraftFunc(ctx, postID)
	}
	return nil
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
	getTokensByUserIDFunc  func(ctx context.Context, userID int64) ([]string, error)
	deleteByUserIDFunc     func(ctx context.Context, userID int64) error
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

func (m *mockFCMTokenRepository) GetTokensByUserID(ctx context.Context, userID int64) ([]string, error) {
	if m.getTokensByUserIDFunc != nil {
		return m.getTokensByUserIDFunc(ctx, userID)
	}
	return []string{}, nil
}

func (m *mockFCMTokenRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	if m.deleteByUserIDFunc != nil {
		return m.deleteByUserIDFunc(ctx, userID)
	}
	return nil
}

// fakeFCMClient は pkg/fcm.Client を実装するテスト用クライアント。
// 送信された data を捕捉して検証に使う。
type fakeFCMClient struct {
	withDataCalls []fcmWithDataCall
}

type fcmWithDataCall struct {
	tokens       []string
	notification fcm.Notification
	data         map[string]string
}

func (f *fakeFCMClient) SendToTokens(ctx context.Context, tokens []string, notification fcm.Notification) error {
	return f.SendToTokensWithData(ctx, tokens, notification, nil)
}

func (f *fakeFCMClient) SendToTokensWithData(ctx context.Context, tokens []string, notification fcm.Notification, data map[string]string) error {
	f.withDataCalls = append(f.withDataCalls, fcmWithDataCall{tokens: tokens, notification: notification, data: data})
	return nil
}

// failingFCMClient は常に送信失敗する fcm.Client（ベストエフォート検証用）
type failingFCMClient struct{}

func (f *failingFCMClient) SendToTokens(ctx context.Context, tokens []string, notification fcm.Notification) error {
	return errors.New("fcm down")
}

func (f *failingFCMClient) SendToTokensWithData(ctx context.Context, tokens []string, notification fcm.Notification, data map[string]string) error {
	return errors.New("fcm down")
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

// TestNotificationService_SendNotification_ActiveConflict はアクティブ通知が存在する場合に 409(ErrConflict) を返すことを確認する
func TestNotificationService_SendNotification_ActiveConflict(t *testing.T) {
	sprintRepo := &mockSprintRepository{}
	createCalled := false
	notifRepo := &mockNotificationRepository{
		hasActiveInSprintFunc: func(ctx context.Context, sprintID int64) (bool, error) {
			return true, nil
		},
		createFunc: func(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
			createCalled = true
			return notif, nil
		},
	}
	fcmTokenRepo := &mockFCMTokenRepository{}
	fcmClient := &fakeFCMClient{}

	svc := NewNotificationService(sprintRepo, notifRepo, fcmTokenRepo, fcmClient)

	_, err := svc.SendNotification(context.Background(), 1, 2)
	if !errors.Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict when active challenge exists, got %v", err)
	}
	if createCalled {
		t.Error("expected Create NOT to be called when an active challenge exists")
	}
	if len(fcmClient.withDataCalls) != 0 {
		t.Error("expected no FCM send when conflicting")
	}
}

// TestNotificationService_SendNotification_SuccessSendsBeGitTimeData は発行成功で begit_time data がグループ全員へ送られることを確認する
func TestNotificationService_SendNotification_SuccessSendsBeGitTimeData(t *testing.T) {
	sprintRepo := &mockSprintRepository{
		getOrCreateFunc: func(ctx context.Context, groupID int64, durationDays int) (*model.Sprint, error) {
			return &model.Sprint{ID: 7, GroupID: groupID}, nil
		},
	}
	notifRepo := &mockNotificationRepository{
		hasActiveInSprintFunc: func(ctx context.Context, sprintID int64) (bool, error) {
			return false, nil
		},
		createFunc: func(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
			notif.ID = 345
			return notif, nil
		},
	}
	fcmTokenRepo := &mockFCMTokenRepository{
		getTokensByGroupIDFunc: func(ctx context.Context, groupID int64) ([]string, error) {
			return []string{"tokenA", "tokenB"}, nil
		},
	}
	fcmClient := &fakeFCMClient{}

	svc := NewNotificationService(sprintRepo, notifRepo, fcmTokenRepo, fcmClient)

	notif, err := svc.SendNotification(context.Background(), 12, 2)
	if err != nil {
		t.Fatalf("SendNotification() failed: %v", err)
	}
	if notif.ID != 345 {
		t.Errorf("expected notif id=345, got %d", notif.ID)
	}
	if len(fcmClient.withDataCalls) != 1 {
		t.Fatalf("expected 1 FCM send, got %d", len(fcmClient.withDataCalls))
	}
	call := fcmClient.withDataCalls[0]
	if len(call.tokens) != 2 {
		t.Errorf("expected send to 2 tokens (whole group), got %d", len(call.tokens))
	}
	if call.data["type"] != "begit_time" || call.data["notification_id"] != "345" || call.data["sprint_id"] != "7" || call.data["group_id"] != "12" {
		t.Errorf("unexpected begit_time data: %v", call.data)
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
