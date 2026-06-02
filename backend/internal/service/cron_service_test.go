package service

import (
	"context"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
)

// mockDeliveryRepo は notification_deliveries の冪等を模す。
// 同一 (kind, ref_id) の2回目以降は alreadySent=true を返す。
type mockDeliveryRepo struct {
	sent map[string]bool
}

func newMockDeliveryRepo() *mockDeliveryRepo {
	return &mockDeliveryRepo{sent: map[string]bool{}}
}

func (m *mockDeliveryRepo) MarkSent(ctx context.Context, kind string, refID int64) (bool, error) {
	key := kind + ":" + itoa(refID)
	if m.sent[key] {
		return true, nil
	}
	m.sent[key] = true
	return false, nil
}

func itoa(v int64) string {
	return s(v)
}

// TestCron_InvalidKind は kind 不正で ErrInvalidCronKind を返すことを確認する
func TestCron_InvalidKind(t *testing.T) {
	svc := NewCronService(&mockNotificationRepository{}, &mockSprintRepository{}, &mockGroupRepository{}, &mockPostRepository{}, newMockDeliveryRepo(), &mockFCMTokenRepository{}, &fakeFCMClient{})
	err := svc.RunCron(context.Background(), "weekly")
	if err != ErrInvalidCronKind {
		t.Errorf("expected ErrInvalidCronKind, got %v", err)
	}
}

// TestCron_Minutely_ChallengeEndOnce は minutely で challenge_end が1回のみ送信（再実行 skip）されることを確認する
func TestCron_Minutely_ChallengeEndOnce(t *testing.T) {
	notifRepo := &mockNotificationRepository{
		listChallengeEndDueFunc: func(ctx context.Context) ([]model.Notification, error) {
			return []model.Notification{{ID: 345, SprintID: 7, SentAt: time.Now().Add(-2 * time.Hour)}}, nil
		},
	}
	sprintRepo := &mockSprintRepository{
		getByIDFunc: func(ctx context.Context, sprintID int64) (*model.Sprint, error) {
			return &model.Sprint{ID: 7, GroupID: 12}, nil
		},
	}
	groupRepo := &mockGroupRepository{
		getMembersFunc: func(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
			return []model.GroupMember{{UserID: 1}, {UserID: 2}}, nil
		},
	}
	postRepo := &mockPostRepository{
		getByUserAndNotifFunc: func(ctx context.Context, userID, notifID int64) (*model.Post, error) {
			return nil, repository.ErrNotFound
		},
	}
	delivery := newMockDeliveryRepo()
	fcmTokenRepo := &mockFCMTokenRepository{
		getTokensByGroupIDFunc: func(ctx context.Context, groupID int64) ([]string, error) {
			return []string{"t1", "t2"}, nil
		},
	}
	fc := &fakeFCMClient{}

	svc := NewCronService(notifRepo, sprintRepo, groupRepo, postRepo, delivery, fcmTokenRepo, fc)

	// 1回目: 送信される
	if err := svc.RunCron(context.Background(), "minutely"); err != nil {
		t.Fatalf("RunCron(minutely) #1 failed: %v", err)
	}
	if len(fc.withDataCalls) != 1 {
		t.Fatalf("expected 1 challenge_end send, got %d", len(fc.withDataCalls))
	}
	if fc.withDataCalls[0].data["type"] != "challenge_end" || fc.withDataCalls[0].data["notification_id"] != "345" {
		t.Errorf("unexpected challenge_end data: %v", fc.withDataCalls[0].data)
	}

	// 2回目（二重起動）: delivery 冪等で skip
	if err := svc.RunCron(context.Background(), "minutely"); err != nil {
		t.Fatalf("RunCron(minutely) #2 failed: %v", err)
	}
	if len(fc.withDataCalls) != 1 {
		t.Errorf("expected no additional send on re-run, got total %d", len(fc.withDataCalls))
	}
}

// TestCron_Minutely_DoesNotPersistMissed は ③ 時点で missed を永続化しないことを確認する
func TestCron_Minutely_DoesNotPersistMissed(t *testing.T) {
	notifRepo := &mockNotificationRepository{
		listChallengeEndDueFunc: func(ctx context.Context) ([]model.Notification, error) {
			return []model.Notification{{ID: 345, SprintID: 7, SentAt: time.Now().Add(-2 * time.Hour)}}, nil
		},
	}
	sprintRepo := &mockSprintRepository{
		getByIDFunc: func(ctx context.Context, sprintID int64) (*model.Sprint, error) {
			return &model.Sprint{ID: 7, GroupID: 12}, nil
		},
	}
	groupRepo := &mockGroupRepository{
		getMembersFunc: func(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
			return []model.GroupMember{{UserID: 1}}, nil
		},
	}
	missedCalled := false
	postRepo := &mockPostRepository{
		getByUserAndNotifFunc: func(ctx context.Context, userID, notifID int64) (*model.Post, error) {
			return nil, repository.ErrNotFound
		},
		createMissedFunc: func(ctx context.Context, notifID, userID, groupID int64) error {
			missedCalled = true
			return nil
		},
	}

	svc := NewCronService(notifRepo, sprintRepo, groupRepo, postRepo, newMockDeliveryRepo(), &mockFCMTokenRepository{}, &fakeFCMClient{})
	if err := svc.RunCron(context.Background(), "minutely"); err != nil {
		t.Fatalf("RunCron(minutely) failed: %v", err)
	}
	if missedCalled {
		t.Error("③ minutely should NOT persist missed (finalized in ⑤)")
	}
}

// TestCron_Daily_Idempotent は daily の各種別が1回のみ送信され、⑤ で missed を確定することを確認する
func TestCron_Daily_Idempotent(t *testing.T) {
	notifRepo := &mockNotificationRepository{
		listBySprintIDFunc: func(ctx context.Context, sprintID int64) ([]model.Notification, error) {
			return []model.Notification{{ID: 100, SprintID: sprintID}}, nil
		},
	}
	sprintRepo := &mockSprintRepository{
		listReminderDueFunc: func(ctx context.Context) ([]model.Sprint, error) {
			return []model.Sprint{{ID: 1, GroupID: 12}}, nil
		},
		listEndedFunc: func(ctx context.Context) ([]model.Sprint, error) {
			return []model.Sprint{{ID: 2, GroupID: 12}}, nil
		},
		listActiveFunc: func(ctx context.Context) ([]model.Sprint, error) {
			return []model.Sprint{{ID: 3, GroupID: 12}}, nil
		},
	}
	groupRepo := &mockGroupRepository{
		getMembersFunc: func(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
			return []model.GroupMember{{UserID: 1}}, nil
		},
	}
	missedCount := 0
	postRepo := &mockPostRepository{
		createMissedFunc: func(ctx context.Context, notifID, userID, groupID int64) error {
			missedCount++
			return nil
		},
	}
	delivery := newMockDeliveryRepo()
	fcmTokenRepo := &mockFCMTokenRepository{
		getTokensByGroupIDFunc: func(ctx context.Context, groupID int64) ([]string, error) {
			return []string{"t"}, nil
		},
	}
	fc := &fakeFCMClient{}

	svc := NewCronService(notifRepo, sprintRepo, groupRepo, postRepo, delivery, fcmTokenRepo, fc)

	// 1回目: reminder + end + start = 3 件送信、missed 確定 1 件
	if err := svc.RunCron(context.Background(), "daily"); err != nil {
		t.Fatalf("RunCron(daily) #1 failed: %v", err)
	}
	if len(fc.withDataCalls) != 3 {
		t.Fatalf("expected 3 daily sends, got %d", len(fc.withDataCalls))
	}
	if missedCount != 1 {
		t.Errorf("⑤ should finalize missed once, got %d", missedCount)
	}
	types := map[string]bool{}
	for _, c := range fc.withDataCalls {
		types[c.data["type"]] = true
	}
	for _, want := range []string{"sprint_reminder", "sprint_end", "sprint_start"} {
		if !types[want] {
			t.Errorf("expected daily send of type %q", want)
		}
	}

	// 2回目（再実行）: 全て delivery 冪等で skip
	if err := svc.RunCron(context.Background(), "daily"); err != nil {
		t.Fatalf("RunCron(daily) #2 failed: %v", err)
	}
	if len(fc.withDataCalls) != 3 {
		t.Errorf("expected no additional daily sends on re-run, got %d", len(fc.withDataCalls))
	}
}
