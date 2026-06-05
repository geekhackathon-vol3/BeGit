package service

import (
	"context"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
)

// TestE2E_Cron_DoubleTrigger_NoDuplicateSends は Cron の二重起動シナリオで
// notification_deliveries により重複送信ゼロになることを検証する（③ minutely / ④⑤⑥ daily）。
//
// 観察可能条件（手動 E2E）:
//
//	cd backend && wrangler dev --test-scheduled
//	curl "http://localhost:8787/__scheduled?cron=*+*+*+*+*"   # → kind=minutely
//	curl "http://localhost:8787/__scheduled?cron=0+0+*+*+*"   # → kind=daily
//	いずれも複数回叩いても notification_deliveries により challenge_end / sprint_* は各1回のみ送信される。
func TestE2E_Cron_DoubleTrigger_NoDuplicateSends(t *testing.T) {
	notifRepo := &mockNotificationRepository{
		listChallengeEndDueFunc: func(ctx context.Context) ([]model.Notification, error) {
			return []model.Notification{{ID: 345, SprintID: 7, SentAt: time.Now().Add(-2 * time.Hour)}}, nil
		},
		listBySprintIDFunc: func(ctx context.Context, sprintID int64) ([]model.Notification, error) {
			return []model.Notification{{ID: 345, SprintID: sprintID}}, nil
		},
	}
	sprintRepo := &mockSprintRepository{
		getByIDFunc: func(ctx context.Context, sprintID int64) (*model.Sprint, error) {
			return &model.Sprint{ID: 7, GroupID: 12}, nil
		},
		listReminderDueFunc: func(ctx context.Context) ([]model.Sprint, error) {
			return []model.Sprint{{ID: 7, GroupID: 12}}, nil
		},
		listEndedFunc: func(ctx context.Context) ([]model.Sprint, error) {
			return []model.Sprint{{ID: 7, GroupID: 12}}, nil
		},
		listActiveFunc: func(ctx context.Context) ([]model.Sprint, error) {
			return []model.Sprint{{ID: 8, GroupID: 12}}, nil
		},
	}
	groupRepo := &mockGroupRepository{
		getMembersFunc: func(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
			return []model.GroupMember{{UserID: 1}, {UserID: 2}}, nil
		},
	}
	// ③ minutely では missed を書かない、⑤ daily では書く、を観測
	missedCallsByKindPhase := 0
	postRepo := &mockPostRepository{
		getByUserAndNotifFunc: func(ctx context.Context, userID, notifID int64) (*model.Post, error) {
			return nil, repository.ErrNotFound
		},
		createMissedFunc: func(ctx context.Context, notifID, userID, groupID int64) error {
			missedCallsByKindPhase++
			return nil
		},
	}
	delivery := newMockDeliveryRepo()
	ft := &mockFCMTokenRepository{
		getTokensByGroupIDFunc: func(ctx context.Context, groupID int64) ([]string, error) {
			return []string{"t1", "t2"}, nil
		},
	}
	fc := &fakeFCMClient{}

	svc := NewCronService(notifRepo, sprintRepo, groupRepo, postRepo, delivery, ft, fc)

	// ── minutely を2回起動（二重起動）──
	for i := 0; i < 2; i++ {
		if err := svc.RunCron(context.Background(), "minutely"); err != nil {
			t.Fatalf("RunCron(minutely) #%d failed: %v", i+1, err)
		}
	}
	// challenge_end は1回のみ。③ では missed を永続化しない。
	if len(fc.withDataCalls) != 1 {
		t.Fatalf("expected exactly 1 challenge_end send across double minutely, got %d", len(fc.withDataCalls))
	}
	if fc.withDataCalls[0].data["type"] != "challenge_end" {
		t.Errorf("expected challenge_end, got %v", fc.withDataCalls[0].data)
	}
	if missedCallsByKindPhase != 0 {
		t.Errorf("③ minutely must NOT persist missed, got %d CreateMissed calls", missedCallsByKindPhase)
	}

	// ── daily を2回起動（二重起動）──
	beforeDaily := len(fc.withDataCalls)
	for i := 0; i < 2; i++ {
		if err := svc.RunCron(context.Background(), "daily"); err != nil {
			t.Fatalf("RunCron(daily) #%d failed: %v", i+1, err)
		}
	}
	// daily の追加送信は reminder/end/start の各1回（計3件）のみ。再実行で重複しない。
	dailySends := len(fc.withDataCalls) - beforeDaily
	if dailySends != 3 {
		t.Fatalf("expected exactly 3 daily sends across double daily, got %d", dailySends)
	}
	types := map[string]int{}
	for _, c := range fc.withDataCalls[beforeDaily:] {
		types[c.data["type"]]++
	}
	for _, want := range []string{"sprint_reminder", "sprint_end", "sprint_start"} {
		if types[want] != 1 {
			t.Errorf("expected exactly 1 %q send, got %d", want, types[want])
		}
	}
	// ⑤ で missed 確定（1 notif × 2 members = 2 件、二重起動でも delivery 冪等により1回のみ実行）
	if missedCallsByKindPhase != 2 {
		t.Errorf("⑤ should finalize missed for 2 members exactly once, got %d", missedCallsByKindPhase)
	}
}
