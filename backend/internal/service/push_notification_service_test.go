package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/fcm"
)

// TestPush_BeGitTime_SameAsSendNotification は Queue 経由の begit_time が、
// SendNotification が API の中で送っていたものと同じ宛先・内容になることを確認する
func TestPush_BeGitTime_SameAsSendNotification(t *testing.T) {
	tokens := &mockFCMTokenRepository{
		getTokensByGroupIDFunc: func(ctx context.Context, groupID int64) ([]string, error) {
			return []string{"tokenA", "tokenB"}, nil
		},
	}

	// 今の経路（API の中で送る）
	before := &fakeFCMClient{}
	notifSvc := NewNotificationService(
		&mockSprintRepository{getOrCreateFunc: func(ctx context.Context, groupID int64, durationDays int) (*model.Sprint, error) {
			return &model.Sprint{ID: 7, GroupID: groupID}, nil
		}},
		&mockNotificationRepository{createIfNoActiveFunc: func(ctx context.Context, notif *model.Notification) (*model.Notification, error) {
			notif.ID = 345
			return notif, nil
		}},
		tokens, before,
	)
	if _, err := notifSvc.SendNotification(context.Background(), 12, 2, SendNotificationRequest{}); err != nil {
		t.Fatalf("SendNotification() failed: %v", err)
	}

	// Queue 経由
	after := &fakeFCMClient{}
	if err := NewPushNotificationService(&mockUserByID{}, tokens, after).SendBeGitTime(context.Background(), 12, 345, 7); err != nil {
		t.Fatalf("SendBeGitTime() failed: %v", err)
	}

	if len(after.withDataCalls) != 1 || !reflect.DeepEqual(before.withDataCalls, after.withDataCalls) {
		t.Errorf("begit_time push differs:\n before=%+v\n after =%+v", before.withDataCalls, after.withDataCalls)
	}
}

// TestPush_Reaction_SameAsAddReaction は Queue 経由の reaction が、
// AddReaction が API の中で送っていたものと同じ宛先・内容になることを確認する
func TestPush_Reaction_SameAsAddReaction(t *testing.T) {
	users := &mockUserByID{getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
		return &model.User{ID: id, GitHubLogin: "octocat"}, nil
	}}
	var tokenOwner int64
	tokens := &mockFCMTokenRepository{getTokensByUserIDFunc: func(ctx context.Context, userID int64) ([]string, error) {
		tokenOwner = userID
		return []string{"author-tok"}, nil
	}}

	// 今の経路（API の中で送る）。投稿者=99、actor=2
	before := &fakeFCMClient{}
	postRepo := &mockPostRepository{getByIDFunc: func(ctx context.Context, postID int64) (*model.Post, error) {
		return &model.Post{ID: postID, GroupID: 12, UserID: 99}, nil
	}}
	if _, err := NewReactionServiceWithNotifications(&mockReactionRepository{}, postRepo, users, tokens, before).
		AddReaction(context.Background(), 12, 890, 2, "heart"); err != nil {
		t.Fatalf("AddReaction() failed: %v", err)
	}

	// Queue 経由
	after := &fakeFCMClient{}
	tokenOwner = 0
	if err := NewPushNotificationService(users, tokens, after).SendReaction(context.Background(), 12, 890, 2, 99); err != nil {
		t.Fatalf("SendReaction() failed: %v", err)
	}

	if tokenOwner != 99 {
		t.Errorf("expected tokens of author 99, got %d", tokenOwner)
	}
	if len(after.withDataCalls) != 1 || !reflect.DeepEqual(before.withDataCalls, after.withDataCalls) {
		t.Errorf("reaction push differs:\n before=%+v\n after =%+v", before.withDataCalls, after.withDataCalls)
	}
}

// TestPush_Reaction_SelfIsSuppressed は自分の投稿へのリアクションでは送らないことを確認する
func TestPush_Reaction_SelfIsSuppressed(t *testing.T) {
	fc := &fakeFCMClient{}
	if err := NewPushNotificationService(&mockUserByID{}, &mockFCMTokenRepository{}, fc).SendReaction(context.Background(), 12, 890, 99, 99); err != nil {
		t.Fatalf("SendReaction() failed: %v", err)
	}
	if len(fc.withDataCalls) != 0 {
		t.Errorf("expected no push for self reaction, got %d", len(fc.withDataCalls))
	}
}

// TestPush_Reaction_ActorNotFound は actor がいなければ ErrNotFound（再送しない）になることを確認する
func TestPush_Reaction_ActorNotFound(t *testing.T) {
	users := &mockUserByID{getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
		return nil, repository.ErrNotFound
	}}
	fc := &fakeFCMClient{}
	err := NewPushNotificationService(users, &mockFCMTokenRepository{}, fc).SendReaction(context.Background(), 12, 890, 2, 99)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if len(fc.withDataCalls) != 0 {
		t.Errorf("expected no push, got %d", len(fc.withDataCalls))
	}
}

// TestPush_TransientErrors は D1 や FCM の失敗が、再送の対象（ErrNotFound 以外のエラー）になることを確認する
func TestPush_TransientErrors(t *testing.T) {
	d1Down := errors.New("d1 down")
	cases := []struct {
		name string
		run  func() error
	}{
		{"begit_time tokens", func() error {
			tokens := &mockFCMTokenRepository{getTokensByGroupIDFunc: func(ctx context.Context, groupID int64) ([]string, error) { return nil, d1Down }}
			return NewPushNotificationService(&mockUserByID{}, tokens, &fakeFCMClient{}).SendBeGitTime(context.Background(), 12, 345, 7)
		}},
		{"begit_time fcm", func() error {
			tokens := &mockFCMTokenRepository{getTokensByGroupIDFunc: func(ctx context.Context, groupID int64) ([]string, error) { return []string{"t"}, nil }}
			return NewPushNotificationService(&mockUserByID{}, tokens, &failingFCMClient{}).SendBeGitTime(context.Background(), 12, 345, 7)
		}},
		{"reaction actor", func() error {
			users := &mockUserByID{getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) { return nil, d1Down }}
			return NewPushNotificationService(users, &mockFCMTokenRepository{}, &fakeFCMClient{}).SendReaction(context.Background(), 12, 890, 2, 99)
		}},
		{"reaction tokens", func() error {
			tokens := &mockFCMTokenRepository{getTokensByUserIDFunc: func(ctx context.Context, userID int64) ([]string, error) { return nil, d1Down }}
			return NewPushNotificationService(&mockUserByID{}, tokens, &fakeFCMClient{}).SendReaction(context.Background(), 12, 890, 2, 99)
		}},
		{"reaction fcm", func() error {
			tokens := &mockFCMTokenRepository{getTokensByUserIDFunc: func(ctx context.Context, userID int64) ([]string, error) { return []string{"t"}, nil }}
			return NewPushNotificationService(&mockUserByID{}, tokens, &failingFCMClient{}).SendReaction(context.Background(), 12, 890, 2, 99)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil || errors.Is(err, ErrNotFound) {
				t.Errorf("expected a retryable error, got %v", err)
			}
		})
	}
}

// TestPush_NoTokensOrNoFCM は送り先がない・FCM が未設定のとき、送らずに成功扱いにすることを確認する
func TestPush_NoTokensOrNoFCM(t *testing.T) {
	fc := &fakeFCMClient{}
	svc := NewPushNotificationService(&mockUserByID{}, &mockFCMTokenRepository{}, fc)
	if err := svc.SendBeGitTime(context.Background(), 12, 345, 7); err != nil {
		t.Errorf("SendBeGitTime() with no tokens: %v", err)
	}
	if err := svc.SendReaction(context.Background(), 12, 890, 2, 99); err != nil {
		t.Errorf("SendReaction() with no tokens: %v", err)
	}
	if len(fc.withDataCalls) != 0 {
		t.Errorf("expected no push, got %d", len(fc.withDataCalls))
	}

	noFCM := NewPushNotificationService(&mockUserByID{}, &mockFCMTokenRepository{}, nil)
	if err := noFCM.SendBeGitTime(context.Background(), 12, 345, 7); err != nil {
		t.Errorf("SendBeGitTime() without FCM: %v", err)
	}
	if err := noFCM.SendReaction(context.Background(), 12, 890, 2, 99); err != nil {
		t.Errorf("SendReaction() without FCM: %v", err)
	}
}

// permanentFCMClient は全トークンが失効している（FCM が 404 を返した）ときの fcm.Client
type permanentFCMClient struct{ failingFCMClient }

func (f *permanentFCMClient) SendToTokensWithData(ctx context.Context, tokens []string, notification fcm.Notification, data map[string]string) error {
	return fmt.Errorf("fcm: all tokens failed, last error: %w", fcm.ErrPermanent)
}

// TestPush_PermanentFCMFailure は送り直しても届かない FCM の失敗が ErrPushUndeliverable（再送しない）になることを確認する
func TestPush_PermanentFCMFailure(t *testing.T) {
	tokens := &mockFCMTokenRepository{
		getTokensByGroupIDFunc: func(ctx context.Context, groupID int64) ([]string, error) { return []string{"stale"}, nil },
		getTokensByUserIDFunc:  func(ctx context.Context, userID int64) ([]string, error) { return []string{"stale"}, nil },
	}
	svc := NewPushNotificationService(&mockUserByID{}, tokens, &permanentFCMClient{})
	if err := svc.SendBeGitTime(context.Background(), 12, 345, 7); !errors.Is(err, ErrPushUndeliverable) {
		t.Errorf("SendBeGitTime: expected ErrPushUndeliverable, got %v", err)
	}
	if err := svc.SendReaction(context.Background(), 12, 890, 2, 99); !errors.Is(err, ErrPushUndeliverable) {
		t.Errorf("SendReaction: expected ErrPushUndeliverable, got %v", err)
	}
}
