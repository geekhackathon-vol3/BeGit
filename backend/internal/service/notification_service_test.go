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
	getActiveInSprintFunc       func(ctx context.Context, sprintID int64, now time.Time) (*model.Notification, error)
	stopFunc                    func(ctx context.Context, notifID, userID int64) error
	getLatestInSprintBeforeFunc func(ctx context.Context, sprintID int64, before time.Time) (*model.Notification, error)
	hasActiveInSprintFunc       func(ctx context.Context, sprintID int64) (bool, error)
	createIfNoActiveFunc        func(ctx context.Context, notif *model.Notification) (*model.Notification, error)
	// createIfNoActiveAllowMultiple は CreateIfNoActive に渡された allowMultiplePerSprint の記録
	createIfNoActiveAllowMultiple []bool
	listChallengeEndDueFunc       func(ctx context.Context) ([]model.Notification, error)
	listBySprintIDFunc            func(ctx context.Context, sprintID int64) ([]model.Notification, error)
	getActiveByGroupFunc          func(ctx context.Context, groupID int64) (*model.Notification, error)
	endIfActiveFunc               func(ctx context.Context, notifID int64) (bool, error)
}

func (m *mockNotificationRepository) GetActiveByGroup(ctx context.Context, groupID int64) (*model.Notification, error) {
	if m.getActiveByGroupFunc != nil {
		return m.getActiveByGroupFunc(ctx, groupID)
	}
	return nil, repository.ErrNotFound
}

func (m *mockNotificationRepository) EndIfActive(ctx context.Context, notifID int64) (bool, error) {
	if m.endIfActiveFunc != nil {
		return m.endIfActiveFunc(ctx, notifID)
	}
	return false, nil
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

func (m *mockNotificationRepository) GetActiveInSprint(ctx context.Context, sprintID int64, now time.Time) (*model.Notification, error) {
	if m.getActiveInSprintFunc != nil {
		return m.getActiveInSprintFunc(ctx, sprintID, now)
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

func (m *mockNotificationRepository) CreateIfNoActive(ctx context.Context, notif *model.Notification, allowMultiplePerSprint bool) (*model.Notification, error) {
	m.createIfNoActiveAllowMultiple = append(m.createIfNoActiveAllowMultiple, allowMultiplePerSprint)
	if m.createIfNoActiveFunc != nil {
		return m.createIfNoActiveFunc(ctx, notif)
	}
	notif.ID = 1
	notif.SentAt = time.Now()
	return notif, nil
}

func (m *mockNotificationRepository) Stop(ctx context.Context, notifID, userID int64) error {
	if m.stopFunc != nil {
		return m.stopFunc(ctx, notifID, userID)
	}
	return nil
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
	recordResponseFunc    func(ctx context.Context, notifID, userID, groupID int64) error
	latestResponseFunc    func(ctx context.Context, userID, groupID int64) (int64, error)
}

func (m *mockPostRepository) RecordNotificationResponse(ctx context.Context, notifID, userID, groupID int64) error {
	if m.recordResponseFunc != nil {
		return m.recordResponseFunc(ctx, notifID, userID, groupID)
	}
	return nil
}

func (m *mockPostRepository) LatestNotificationResponse(ctx context.Context, userID, groupID int64) (int64, error) {
	if m.latestResponseFunc != nil {
		return m.latestResponseFunc(ctx, userID, groupID)
	}
	posts, err := m.ListByGroupID(ctx, groupID)
	if err != nil {
		return 0, err
	}
	latestNotificationID := int64(0)
	for _, post := range posts {
		if post.UserID != userID || post.NotificationID == nil || isMissedPost(&post) {
			continue
		}
		if *post.NotificationID > latestNotificationID {
			latestNotificationID = *post.NotificationID
		}
	}
	return latestNotificationID, nil
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
		createIfNoActiveFunc: func(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
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

func TestNotificationService_GetActiveNotification(t *testing.T) {
	sentAt := time.Now().UTC().Add(-10 * time.Minute)
	notifRepo := &mockNotificationRepository{
		getActiveInSprintFunc: func(ctx context.Context, sprintID int64, now time.Time) (*model.Notification, error) {
			if sprintID != 1 {
				t.Fatalf("expected sprint 1, got %d", sprintID)
			}
			return &model.Notification{ID: 12, SprintID: sprintID, SentBy: 7, SentAt: sentAt}, nil
		},
	}
	svc := NewNotificationService(&mockSprintRepository{}, notifRepo, &mockFCMTokenRepository{}, nil)

	active, err := svc.GetActiveNotification(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetActiveNotification() failed: %v", err)
	}
	if active == nil || active.NotificationID != 12 || active.SentBy != 7 || !active.ExpiresAt.Equal(sentAt.Add(challengeWindow)) {
		t.Fatalf("unexpected active notification: %+v", active)
	}
}

// TestNotificationService_SendNotification_OncePerSprintByDefault は既定（設定なし）で
// 1スプリント1人1回の判定を有効にしてリポジトリへ渡すことを確認する
func TestNotificationService_SendNotification_OncePerSprintByDefault(t *testing.T) {
	notifRepo := &mockNotificationRepository{}
	svc := NewNotificationService(&mockSprintRepository{}, notifRepo, &mockFCMTokenRepository{}, nil)

	if _, err := svc.SendNotification(context.Background(), 1, 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notifRepo.createIfNoActiveAllowMultiple) != 1 || notifRepo.createIfNoActiveAllowMultiple[0] {
		t.Fatalf("expected allowMultiplePerSprint=false, got %v", notifRepo.createIfNoActiveAllowMultiple)
	}
}

// TestNotificationService_SendNotification_AllowMultiplePerSprint は設定で許可した場合、
// 同一ユーザーの2回目の発行も（アクティブなチャレンジが無ければ）成功することを確認する
func TestNotificationService_SendNotification_AllowMultiplePerSprint(t *testing.T) {
	notifRepo := &mockNotificationRepository{}
	svc := NewNotificationServiceFull(&mockSprintRepository{}, notifRepo, &mockFCMTokenRepository{}, nil, nil, nil, true)

	for i := 0; i < 2; i++ {
		if _, err := svc.SendNotification(context.Background(), 1, 2); err != nil {
			t.Fatalf("send #%d: unexpected error: %v", i+1, err)
		}
	}
	for i, allow := range notifRepo.createIfNoActiveAllowMultiple {
		if !allow {
			t.Fatalf("send #%d: expected allowMultiplePerSprint=true", i+1)
		}
	}
}

// TestNotificationService_SendNotification_ActiveConflict はアクティブ通知が存在する場合に 409(ErrConflict) を返すことを確認する
func TestNotificationService_SendNotification_ActiveConflict(t *testing.T) {
	sprintRepo := &mockSprintRepository{}
	notifRepo := &mockNotificationRepository{
		// アクティブ通知が存在する状況は CreateIfNoActive が原子的に ErrConstraintViolation を返して表現する
		createIfNoActiveFunc: func(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
			return nil, repository.ErrConstraintViolation
		},
	}
	fcmTokenRepo := &mockFCMTokenRepository{}
	fcmClient := &fakeFCMClient{}

	svc := NewNotificationService(sprintRepo, notifRepo, fcmTokenRepo, fcmClient)

	_, err := svc.SendNotification(context.Background(), 1, 2)
	if !errors.Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict when active challenge exists, got %v", err)
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
		createIfNoActiveFunc: func(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
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
	// sentAt から 61 分後 = 締切(sentAt + 1h)を明確に超過 → Late。
	// postedAt を sentAt 基準で導出し、time.Now() を2回呼ぶことで生じる
	// 締切ちょうど（On Time 誤判定）のフレーキーを排除する。
	postedAt := sentAt.Add(61 * time.Minute)

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

// TestNotificationService_SendNotification_FCMFailure_DoesNotFail は FCM 失敗でも ① 発行が成功することを確認する
func TestNotificationService_SendNotification_FCMFailure_DoesNotFail(t *testing.T) {
	sprintRepo := &mockSprintRepository{
		getOrCreateFunc: func(ctx context.Context, groupID int64, durationDays int) (*model.Sprint, error) {
			return &model.Sprint{ID: 7, GroupID: groupID}, nil
		},
	}
	notifRepo := &mockNotificationRepository{
		hasActiveInSprintFunc: func(ctx context.Context, sprintID int64) (bool, error) { return false, nil },
		createFunc: func(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
			notif.ID = 345
			return notif, nil
		},
	}
	ft := &mockFCMTokenRepository{
		getTokensByGroupIDFunc: func(ctx context.Context, groupID int64) ([]string, error) {
			return []string{"a", "b"}, nil
		},
	}
	svc := NewNotificationService(sprintRepo, notifRepo, ft, &failingFCMClient{})
	if _, err := svc.SendNotification(context.Background(), 12, 2); err != nil {
		t.Fatalf("SendNotification() should succeed even if FCM fails, got: %v", err)
	}
}

// TestNotificationService_GetActiveChallenge_None は進行中の通知が無ければ (nil, nil) を返すことを確認する
func TestNotificationService_GetActiveChallenge_None(t *testing.T) {
	svc := NewNotificationServiceWithGroupRepo(&mockSprintRepository{}, &mockNotificationRepository{}, &mockGroupRepository{}, &mockPostRepository{})
	active, err := svc.GetActiveChallenge(context.Background(), 12, 10)
	if err != nil {
		t.Fatalf("GetActiveChallenge() failed: %v", err)
	}
	if active != nil {
		t.Errorf("expected nil, got %+v", active)
	}
}

// TestNotificationService_GetActiveChallenge_NoPost は進行中通知はあるが自分の投稿が無い場合、MyPost が nil で締め切りが sent_at+1h になることを確認する
func TestNotificationService_GetActiveChallenge_NoPost(t *testing.T) {
	sentAt := time.Now().Add(-10 * time.Minute)
	notifRepo := &mockNotificationRepository{
		getActiveByGroupFunc: func(ctx context.Context, groupID int64) (*model.Notification, error) {
			if groupID != 12 {
				t.Errorf("expected groupID 12, got %d", groupID)
			}
			return &model.Notification{ID: 8, SprintID: 5, SentBy: 23, SentAt: sentAt}, nil
		},
	}
	svc := NewNotificationServiceWithGroupRepo(&mockSprintRepository{}, notifRepo, &mockGroupRepository{}, &mockPostRepository{})
	active, err := svc.GetActiveChallenge(context.Background(), 12, 10)
	if err != nil {
		t.Fatalf("GetActiveChallenge() failed: %v", err)
	}
	if active == nil || active.Notification.ID != 8 {
		t.Fatalf("expected active notification 8, got %+v", active)
	}
	if !active.EndsAt.Equal(sentAt.Add(time.Hour)) {
		t.Errorf("expected EndsAt = sent_at + 1h, got %v", active.EndsAt)
	}
	if active.MyPost != nil {
		t.Errorf("expected MyPost nil, got %+v", active.MyPost)
	}
}

// TestNotificationService_GetActiveChallenge_WithDraft は自分の Nice Work! draft があれば MyPost に含まれることを確認する
func TestNotificationService_GetActiveChallenge_WithDraft(t *testing.T) {
	notifRepo := &mockNotificationRepository{
		getActiveByGroupFunc: func(ctx context.Context, groupID int64) (*model.Notification, error) {
			return &model.Notification{ID: 8, SprintID: 5, SentBy: 23, SentAt: time.Now().Add(-10 * time.Minute)}, nil
		},
	}
	status := "on_time"
	postRepo := &mockPostRepository{
		getByUserAndNotifFunc: func(ctx context.Context, userID, notifID int64) (*model.Post, error) {
			if userID != 10 || notifID != 8 {
				t.Errorf("expected (user 10, notif 8), got (%d, %d)", userID, notifID)
			}
			return &model.Post{ID: 41, UserID: 10, IsDraft: true, Status: &status}, nil
		},
	}
	svc := NewNotificationServiceWithGroupRepo(&mockSprintRepository{}, notifRepo, &mockGroupRepository{}, postRepo)
	active, err := svc.GetActiveChallenge(context.Background(), 12, 10)
	if err != nil {
		t.Fatalf("GetActiveChallenge() failed: %v", err)
	}
	if active == nil || active.MyPost == nil {
		t.Fatalf("expected MyPost, got %+v", active)
	}
	if active.MyPost.ID != 41 || !active.MyPost.IsDraft || active.MyPost.Status == nil || *active.MyPost.Status != "on_time" {
		t.Errorf("unexpected MyPost: %+v", active.MyPost)
	}
}

// endChallengeDeps は EndChallenge テスト用の共通依存（通知 8 = sprint 5 = group 12、発行者 23）を返す
func endChallengeDeps(t *testing.T) (*mockNotificationRepository, *mockSprintRepository) {
	t.Helper()
	notifRepo := &mockNotificationRepository{
		getByIDFunc: func(ctx context.Context, notifID int64) (*model.Notification, error) {
			if notifID != 8 {
				return nil, repository.ErrNotFound
			}
			return &model.Notification{ID: 8, SprintID: 5, SentBy: 23, SentAt: time.Now().Add(-10 * time.Minute)}, nil
		},
	}
	sprintRepo := &mockSprintRepository{
		getByIDFunc: func(ctx context.Context, sprintID int64) (*model.Sprint, error) {
			return &model.Sprint{ID: 5, GroupID: 12}, nil
		},
	}
	return notifRepo, sprintRepo
}

// TestNotificationService_EndChallenge_NotIssuer_Forbidden は発行者以外が中断しようとすると ErrForbidden になることを確認する
func TestNotificationService_EndChallenge_NotIssuer_Forbidden(t *testing.T) {
	notifRepo, sprintRepo := endChallengeDeps(t)
	ended := false
	notifRepo.endIfActiveFunc = func(ctx context.Context, notifID int64) (bool, error) { ended = true; return true, nil }
	svc := NewNotificationServiceWithGroupRepo(sprintRepo, notifRepo, &mockGroupRepository{}, &mockPostRepository{})

	_, err := svc.EndChallenge(context.Background(), 12, 8, 24)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
	if ended {
		t.Error("EndIfActive must not be called for a non-issuer")
	}
}

// TestNotificationService_EndChallenge_WrongGroup_NotFound は通知が別グループのものなら ErrNotFound になることを確認する
func TestNotificationService_EndChallenge_WrongGroup_NotFound(t *testing.T) {
	notifRepo, sprintRepo := endChallengeDeps(t)
	svc := NewNotificationServiceWithGroupRepo(sprintRepo, notifRepo, &mockGroupRepository{}, &mockPostRepository{})

	if _, err := svc.EndChallenge(context.Background(), 99, 8, 23); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for wrong group, got %v", err)
	}
	if _, err := svc.EndChallenge(context.Background(), 12, 404, 23); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for unknown notification, got %v", err)
	}
}

// TestNotificationService_EndChallenge_NotActive_Conflict は既に中断済み／1時間経過済み（UPDATE 0 行）なら ErrConflict になることを確認する
func TestNotificationService_EndChallenge_NotActive_Conflict(t *testing.T) {
	notifRepo, sprintRepo := endChallengeDeps(t)
	notifRepo.endIfActiveFunc = func(ctx context.Context, notifID int64) (bool, error) { return false, nil }
	svc := NewNotificationServiceWithGroupRepo(sprintRepo, notifRepo, &mockGroupRepository{}, &mockPostRepository{})

	if _, err := svc.EndChallenge(context.Background(), 12, 8, 23); !errors.Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

// TestNotificationService_EndChallenge_OK は発行者が中断すると ended_at 付きの通知が返ることを確認する
func TestNotificationService_EndChallenge_OK(t *testing.T) {
	notifRepo, sprintRepo := endChallengeDeps(t)
	endedAt := time.Now().Truncate(time.Second)
	notifRepo.endIfActiveFunc = func(ctx context.Context, notifID int64) (bool, error) {
		if notifID != 8 {
			t.Errorf("expected notifID 8, got %d", notifID)
		}
		// 中断後の GetByID は ended_at 付きを返す
		notifRepo.getByIDFunc = func(ctx context.Context, notifID int64) (*model.Notification, error) {
			return &model.Notification{ID: 8, SprintID: 5, SentBy: 23, SentAt: endedAt.Add(-10 * time.Minute), EndedAt: &endedAt}, nil
		}
		return true, nil
	}
	svc := NewNotificationServiceWithGroupRepo(sprintRepo, notifRepo, &mockGroupRepository{}, &mockPostRepository{})

	notif, err := svc.EndChallenge(context.Background(), 12, 8, 23)
	if err != nil {
		t.Fatalf("EndChallenge() failed: %v", err)
	}
	if notif.EndedAt == nil || !notif.EndedAt.Equal(endedAt) {
		t.Errorf("expected EndedAt %v, got %v", endedAt, notif.EndedAt)
	}
}

// TestNotificationService_GetStatus_EndedAtIsDeadline は途中中断された通知では ended_at が締め切りになり、
// ended_at 後（sent_at + 1h 前）の投稿が Late、ended_at 前の投稿が On Time になることを確認する
func TestNotificationService_GetStatus_EndedAtIsDeadline(t *testing.T) {
	sentAt := time.Now().Add(-40 * time.Minute)
	endedAt := sentAt.Add(15 * time.Minute) // 発行15分後に中断
	notifRepo := &mockNotificationRepository{
		getByIDFunc: func(ctx context.Context, notifID int64) (*model.Notification, error) {
			return &model.Notification{ID: 1, SprintID: 1, SentBy: 10, SentAt: sentAt, EndedAt: &endedAt}, nil
		},
	}
	groupRepo := &mockGroupRepository{
		getMembersFunc: func(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
			return []model.GroupMember{{UserID: 10, Login: "before"}, {UserID: 11, Login: "after"}}, nil
		},
	}
	postRepo := &mockPostRepository{
		getByUserAndNotifFunc: func(ctx context.Context, userID, notifID int64) (*model.Post, error) {
			switch userID {
			case 10:
				return &model.Post{ID: 1, UserID: 10, CreatedAt: endedAt.Add(-time.Minute)}, nil
			case 11:
				return &model.Post{ID: 2, UserID: 11, CreatedAt: endedAt.Add(time.Minute)}, nil // 中断後・1時間以内
			}
			return nil, repository.ErrNotFound
		},
	}
	svc := NewNotificationServiceWithGroupRepo(&mockSprintRepository{}, notifRepo, groupRepo, postRepo)

	status, err := svc.GetNotificationStatus(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("GetNotificationStatus() failed: %v", err)
	}
	if status.Members[0].Status != "On Time" {
		t.Errorf("post before ended_at: expected On Time, got %s", status.Members[0].Status)
	}
	if status.Members[1].Status != "Late" {
		t.Errorf("post after ended_at: expected Late, got %s", status.Members[1].Status)
	}
}

// TestChallengeDeadline は締め切り算出（sent_at + 1h と ended_at の早い方）を確認する
func TestChallengeDeadline(t *testing.T) {
	sentAt := time.Date(2026, 9, 18, 1, 25, 46, 0, time.UTC)
	early := sentAt.Add(15 * time.Minute)
	late := sentAt.Add(2 * time.Hour)
	for name, tc := range map[string]struct {
		endedAt *time.Time
		want    time.Time
	}{
		"no end":          {nil, sentAt.Add(time.Hour)},
		"ended early":     {&early, early},
		"ended after 1h?": {&late, sentAt.Add(time.Hour)},
	} {
		got := challengeDeadline(&model.Notification{SentAt: sentAt, EndedAt: tc.endedAt})
		if !got.Equal(tc.want) {
			t.Errorf("[%s] expected %v, got %v", name, tc.want, got)
		}
	}
}
