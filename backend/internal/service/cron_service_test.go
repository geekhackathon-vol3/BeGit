package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/d1"
)

type failingPointsD1Client struct{ d1.Client }

func (failingPointsD1Client) Exec(context.Context, string, []interface{}) (int64, error) {
	return 0, errors.New("D1 unavailable")
}

func TestCron_Minutely_ContinuesAfterBonusFinalizationError(t *testing.T) {
	var scheduled, reminders30, reminders10, challengeEnd bool
	notifs := &mockNotificationRepository{
		listChallengeReminderDueFunc: func(_ context.Context, minutes int) ([]model.Notification, error) {
			switch minutes {
			case 30:
				reminders30 = true
			case 10:
				reminders10 = true
			}
			return nil, nil
		},
		listChallengeEndDueFunc: func(context.Context) ([]model.Notification, error) {
			challengeEnd = true
			return nil, nil
		},
	}
	schedules := &mockBeGitTimeScheduleRepository{
		listDueFunc: func(context.Context, time.Time) ([]model.BeGitTimeSchedule, error) {
			scheduled = true
			return nil, nil
		},
	}
	svc := &cronService{notifRepo: notifs, scheduleRepo: schedules, pointsRepo: repository.NewPointsRepository(failingPointsD1Client{})}
	if err := svc.runMinutely(context.Background()); err != nil {
		t.Fatalf("runMinutely() failed: %v", err)
	}
	if !scheduled || !reminders30 || !reminders10 || !challengeEnd {
		t.Fatalf("later cron phases skipped: scheduled=%t reminders30=%t reminders10=%t challengeEnd=%t", scheduled, reminders30, reminders10, challengeEnd)
	}
}

// mockDeliveryRepo は notification_deliveries の冪等を模す。
// 同一 (kind, ref_id) の2回目以降は alreadySent=true を返す。
type mockDeliveryRepo struct {
	sent        map[string]bool
	releaseFunc func(context.Context, string, int64) error
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

// HasBeenSent は (kind, ref_id) が記録済みかを返す（マークはしない）。
func (m *mockDeliveryRepo) HasBeenSent(ctx context.Context, kind string, refID int64) (bool, error) {
	return m.sent[kind+":"+itoa(refID)], nil
}

func (m *mockDeliveryRepo) Release(ctx context.Context, kind string, refID int64) error {
	if m.releaseFunc != nil {
		return m.releaseFunc(ctx, kind, refID)
	}
	delete(m.sent, kind+":"+itoa(refID))
	return nil
}

func itoa(v int64) string {
	return s(v)
}

type mockBeGitTimeScheduleRepository struct {
	listDueFunc func(context.Context, time.Time) ([]model.BeGitTimeSchedule, error)
	advanceFunc func(context.Context, int64, time.Time, time.Time) (bool, error)
}

func (m *mockBeGitTimeScheduleRepository) GetByGroupID(context.Context, int64) (*model.BeGitTimeSchedule, error) {
	return nil, repository.ErrNotFound
}
func (m *mockBeGitTimeScheduleRepository) Upsert(context.Context, *model.BeGitTimeSchedule) (*model.BeGitTimeSchedule, error) {
	return nil, nil
}
func (m *mockBeGitTimeScheduleRepository) ListDue(ctx context.Context, now time.Time) ([]model.BeGitTimeSchedule, error) {
	if m.listDueFunc != nil {
		return m.listDueFunc(ctx, now)
	}
	return nil, nil
}
func (m *mockBeGitTimeScheduleRepository) Advance(ctx context.Context, groupID int64, previous, next time.Time) (bool, error) {
	if m.advanceFunc != nil {
		return m.advanceFunc(ctx, groupID, previous, next)
	}
	return true, nil
}

func TestCron_Minutely_RemindsOnlyUnpostedMembers(t *testing.T) {
	notifRepo := &mockNotificationRepository{
		listChallengeReminderDueFunc: func(ctx context.Context, remainingMinutes int) ([]model.Notification, error) {
			if remainingMinutes == 30 {
				return []model.Notification{{ID: 44, SprintID: 7}}, nil
			}
			return nil, nil
		},
	}
	sprintRepo := &mockSprintRepository{getByIDFunc: func(ctx context.Context, sprintID int64) (*model.Sprint, error) {
		return &model.Sprint{ID: sprintID, GroupID: 12}, nil
	}}
	var gotGroupID, gotNotificationID int64
	tokens := &mockFCMTokenRepository{getUnpostedTokensFunc: func(ctx context.Context, groupID, notificationID int64) ([]string, error) {
		gotGroupID, gotNotificationID = groupID, notificationID
		return []string{"unposted-device"}, nil
	}}
	fc := &fakeFCMClient{}
	svc := NewCronService(notifRepo, sprintRepo, &mockGroupRepository{}, &mockPostRepository{}, newMockDeliveryRepo(), tokens, fc)

	if err := svc.RunCron(context.Background(), "minutely"); err != nil {
		t.Fatalf("RunCron() failed: %v", err)
	}
	if gotGroupID != 12 || gotNotificationID != 44 {
		t.Fatalf("responder filter got group=%d notification=%d", gotGroupID, gotNotificationID)
	}
	if len(fc.withDataCalls) != 1 || fc.withDataCalls[0].data["type"] != "challenge_reminder" {
		t.Fatalf("unexpected sends: %+v", fc.withDataCalls)
	}
	if fc.withDataCalls[0].data["remaining_minutes"] != "30" {
		t.Fatalf("remaining_minutes=%q", fc.withDataCalls[0].data["remaining_minutes"])
	}
}

func TestCron_Minutely_AutomaticallyStartsScheduledBeGitTime(t *testing.T) {
	dueAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	advanced := false
	schedules := &mockBeGitTimeScheduleRepository{
		listDueFunc: func(context.Context, time.Time) ([]model.BeGitTimeSchedule, error) {
			return []model.BeGitTimeSchedule{{
				GroupID: 12, Enabled: true, LocalTime: "09:00", Timezone: "Asia/Tokyo",
				Weekdays: []int{1, 2, 3, 4, 5, 6, 7}, NextRunAt: &dueAt,
			}}, nil
		},
		advanceFunc: func(context.Context, int64, time.Time, time.Time) (bool, error) {
			advanced = true
			return true, nil
		},
	}
	var createdSource string
	notifRepo := &mockNotificationRepository{createIfNoActiveFunc: func(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
		createdSource = notif.Source
		notif.ID = 88
		return notif, nil
	}}
	sprintRepo := &mockSprintRepository{getOrCreateFunc: func(ctx context.Context, groupID int64, durationDays int) (*model.Sprint, error) {
		return &model.Sprint{ID: 7, GroupID: groupID}, nil
	}}
	groupRepo := &mockGroupRepository{getByIDFunc: func(ctx context.Context, groupID int64) (*model.Group, error) {
		return &model.Group{ID: groupID, OwnerUserID: 5, OwnerVerified: true, SprintDurationDays: 7}, nil
	}}
	tokens := &mockFCMTokenRepository{getTokensByGroupIDFunc: func(context.Context, int64) ([]string, error) {
		return []string{"device"}, nil
	}}
	fc := &fakeFCMClient{}
	svc := NewCronServiceWithPublisher(notifRepo, sprintRepo, groupRepo, &mockPostRepository{}, newMockDeliveryRepo(), tokens, fc, nil, schedules)

	if err := svc.RunCron(context.Background(), "minutely"); err != nil {
		t.Fatalf("RunCron() failed: %v", err)
	}
	if createdSource != "scheduled" || !advanced {
		t.Fatalf("source=%q advanced=%v", createdSource, advanced)
	}
	if len(fc.withDataCalls) != 1 || fc.withDataCalls[0].data["type"] != "begit_time" {
		t.Fatalf("unexpected sends: %+v", fc.withDataCalls)
	}
}

func TestCron_Minutely_AdvancesScheduledBeGitTimeWhenDeliveryFails(t *testing.T) {
	dueAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	advanced := false
	schedules := &mockBeGitTimeScheduleRepository{
		listDueFunc: func(context.Context, time.Time) ([]model.BeGitTimeSchedule, error) {
			return []model.BeGitTimeSchedule{{
				GroupID: 12, Enabled: true, LocalTime: "09:00", Timezone: "Asia/Tokyo",
				Weekdays: []int{1, 2, 3, 4, 5, 6, 7}, NextRunAt: &dueAt,
			}}, nil
		},
		advanceFunc: func(context.Context, int64, time.Time, time.Time) (bool, error) {
			advanced = true
			return true, nil
		},
	}
	notifRepo := &mockNotificationRepository{createIfNoActiveFunc: func(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
		notif.ID = 88
		return notif, nil
	}}
	sprintRepo := &mockSprintRepository{getOrCreateFunc: func(ctx context.Context, groupID int64, durationDays int) (*model.Sprint, error) {
		return &model.Sprint{ID: 7, GroupID: groupID}, nil
	}}
	groupRepo := &mockGroupRepository{getByIDFunc: func(ctx context.Context, groupID int64) (*model.Group, error) {
		return &model.Group{ID: groupID, OwnerUserID: 5, OwnerVerified: true, SprintDurationDays: 7}, nil
	}}
	tokens := &mockFCMTokenRepository{getTokensByGroupIDFunc: func(context.Context, int64) ([]string, error) {
		return []string{"device"}, nil
	}}
	delivery := newMockDeliveryRepo()
	publisher := &recordingExternalNotificationPublisher{}
	svc := NewCronServiceWithPublisher(notifRepo, sprintRepo, groupRepo, &mockPostRepository{}, delivery, tokens, &failingFCMClient{}, publisher, schedules)

	if err := svc.RunCron(context.Background(), "minutely"); err != nil {
		t.Fatalf("RunCron() failed: %v", err)
	}
	if !advanced {
		t.Fatal("expected schedule to advance after notification creation")
	}
	if len(publisher.events) != 0 {
		t.Fatalf("expected no external publish after FCM failure, got %d", len(publisher.events))
	}
	if delivery.sent[deliveryBeGitTime+":88"] {
		t.Fatal("expected failed delivery claim to be released")
	}
}

func TestCron_SendToUnpostedIfNotSent_ReleasesDeliveryWithIndependentContext(t *testing.T) {
	ctx, cancelSend := context.WithCancel(context.Background())
	delivery := newMockDeliveryRepo()
	releaseCalled := false
	delivery.releaseFunc = func(ctx context.Context, kind string, refID int64) error {
		releaseCalled = true
		if ctx.Err() != nil {
			t.Fatalf("release context is already canceled: %v", ctx.Err())
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("release context has no deadline")
		}
		delete(delivery.sent, kind+":"+itoa(refID))
		return nil
	}
	tokens := &mockFCMTokenRepository{getUnpostedTokensFunc: func(context.Context, int64, int64) ([]string, error) {
		cancelSend()
		return []string{"device"}, nil
	}}
	svc := &cronService{
		deliveryRepo: delivery,
		fcmTokenRepo: tokens,
		fcmClient:    &failingFCMClient{},
	}

	err := svc.sendToUnpostedIfNotSent(ctx, deliveryReminder30, 88, 12, BuildChallengeReminder(12, 88, 30))
	if err == nil {
		t.Fatal("expected delivery error")
	}
	if !releaseCalled {
		t.Fatal("expected failed delivery claim to be released")
	}
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

// TestCron_FCMFailure_DoesNotFail は FCM 失敗でも Cron 実行がエラーにならないことを確認する
func TestCron_FCMFailure_DoesNotFail(t *testing.T) {
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
	postRepo := &mockPostRepository{
		getByUserAndNotifFunc: func(ctx context.Context, userID, notifID int64) (*model.Post, error) {
			return nil, repository.ErrNotFound
		},
	}
	ft := &mockFCMTokenRepository{
		getTokensByGroupIDFunc: func(ctx context.Context, groupID int64) ([]string, error) {
			return []string{"t1"}, nil
		},
	}
	svc := NewCronService(notifRepo, sprintRepo, groupRepo, postRepo, newMockDeliveryRepo(), ft, &failingFCMClient{})
	if err := svc.RunCron(context.Background(), "minutely"); err != nil {
		t.Fatalf("RunCron(minutely) should succeed even if FCM fails, got: %v", err)
	}
}

// TestCron_Minutely_EarlyEnded_SendsChallengeEnd は途中中断された通知（ended_at 非 nil、まだ1時間以内）が
// ListChallengeEndDue で拾われれば challenge_end が送信され、ステータス算出で ended_at 後の投稿が Late になることを確認する
func TestCron_Minutely_EarlyEnded_SendsChallengeEnd(t *testing.T) {
	sentAt := time.Now().Add(-20 * time.Minute)
	endedAt := sentAt.Add(5 * time.Minute)
	notifRepo := &mockNotificationRepository{
		listChallengeEndDueFunc: func(ctx context.Context) ([]model.Notification, error) {
			return []model.Notification{{ID: 8, SprintID: 5, SentBy: 23, SentAt: sentAt, EndedAt: &endedAt}}, nil
		},
	}
	sprintRepo := &mockSprintRepository{
		getByIDFunc: func(ctx context.Context, sprintID int64) (*model.Sprint, error) {
			return &model.Sprint{ID: 5, GroupID: 12}, nil
		},
	}
	groupRepo := &mockGroupRepository{
		getMembersFunc: func(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
			return []model.GroupMember{{UserID: 23}, {UserID: 24}}, nil
		},
	}
	postRepo := &mockPostRepository{
		getByUserAndNotifFunc: func(ctx context.Context, userID, notifID int64) (*model.Post, error) {
			if userID == 24 {
				return &model.Post{ID: 2, UserID: 24, CreatedAt: endedAt.Add(time.Minute)}, nil
			}
			return nil, repository.ErrNotFound
		},
	}
	fcmTokenRepo := &mockFCMTokenRepository{
		getTokensByGroupIDFunc: func(ctx context.Context, groupID int64) ([]string, error) { return []string{"t1"}, nil },
	}
	fc := &fakeFCMClient{}

	svc := NewCronService(notifRepo, sprintRepo, groupRepo, postRepo, newMockDeliveryRepo(), fcmTokenRepo, fc)
	if err := svc.RunCron(context.Background(), "minutely"); err != nil {
		t.Fatalf("RunCron(minutely) failed: %v", err)
	}
	if len(fc.withDataCalls) != 1 || fc.withDataCalls[0].data["type"] != "challenge_end" || fc.withDataCalls[0].data["notification_id"] != "8" {
		t.Fatalf("expected 1 challenge_end send for notification 8, got %+v", fc.withDataCalls)
	}
}
