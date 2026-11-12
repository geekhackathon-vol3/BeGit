package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/fcm"
)

// audienceTokens は対象メンバーのトークンを引ける FCM トークンリポジトリ（本番の fcmTokenRepository と同じく
// repository.NotificationFCMTokenRepository を満たす）。共有の mockFCMTokenRepository に足すと、
// SendNotification の既存のテストが対象メンバーの経路に切り替わってしまうので、ここだけで包む
type audienceTokens struct {
	*mockFCMTokenRepository
	byNotification func(ctx context.Context, notificationID int64) ([]string, error)
}

func (a *audienceTokens) GetTokensByNotificationID(ctx context.Context, notificationID int64) ([]string, error) {
	if a.byNotification != nil {
		return a.byNotification(ctx, notificationID)
	}
	return []string{}, nil
}

func (a *audienceTokens) GetTokensByNotificationIDExcludingResponders(ctx context.Context, notificationID int64) ([]string, error) {
	return nil, errors.New("not used")
}

// notifWithMode は GetByID で指定のゲームモードの、発行したばかりの通知を返す通知リポジトリ
func notifWithMode(mode model.GameMode) *mockNotificationRepository {
	return &mockNotificationRepository{getByIDFunc: func(ctx context.Context, notifID int64) (*model.Notification, error) {
		return &model.Notification{ID: notifID, SprintID: 7, GameMode: mode, SentAt: time.Now()}, nil
	}}
}

// TestPush_BeGitTime_SameAsSendNotification は Queue 経由の begit_time が、
// SendNotification が API の中で送っていたものと同じ宛先（対象メンバー）・内容（モード別の文言）になることを確認する
func TestPush_BeGitTime_SameAsSendNotification(t *testing.T) {
	for _, mode := range []model.GameMode{model.GameModeEveryone, model.GameModeSpotlight} {
		t.Run(string(mode), func(t *testing.T) {
			var askedNotification []int64
			tokens := &audienceTokens{
				mockFCMTokenRepository: &mockFCMTokenRepository{
					getTokensByGroupIDFunc: func(ctx context.Context, groupID int64) ([]string, error) {
						return []string{"everyone-in-group"}, nil
					},
				},
				byNotification: func(ctx context.Context, notificationID int64) ([]string, error) {
					askedNotification = append(askedNotification, notificationID)
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
			if _, err := notifSvc.SendNotification(context.Background(), 12, 2, SendNotificationRequest{GameMode: mode}); err != nil {
				t.Fatalf("SendNotification() failed: %v", err)
			}

			// Queue 経由
			after := &fakeFCMClient{}
			if err := NewPushNotificationService(&mockUserByID{}, notifWithMode(mode), tokens, after).SendBeGitTime(context.Background(), 12, 345, 7); err != nil {
				t.Fatalf("SendBeGitTime() failed: %v", err)
			}

			if len(after.withDataCalls) != 1 || !reflect.DeepEqual(before.withDataCalls, after.withDataCalls) {
				t.Errorf("begit_time push differs:\n before=%+v\n after =%+v", before.withDataCalls, after.withDataCalls)
			}
			if !reflect.DeepEqual(askedNotification, []int64{345, 345}) {
				t.Errorf("expected tokens of notification 345 on both paths, got %v", askedNotification)
			}
			if got := after.withDataCalls[0].data["game_mode"]; got != string(mode) {
				t.Errorf("expected game_mode %q in data, got %q", mode, got)
			}
		})
	}
}

// TestPush_BeGitTime_NotificationNotFound は通知が消えていれば ErrNotFound（再送しない）になり、送らないことを確認する
func TestPush_BeGitTime_NotificationNotFound(t *testing.T) {
	fc := &fakeFCMClient{}
	notifs := &mockNotificationRepository{getByIDFunc: func(ctx context.Context, id int64) (*model.Notification, error) {
		return nil, repository.ErrNotFound
	}}
	err := NewPushNotificationService(&mockUserByID{}, notifs, &audienceTokens{mockFCMTokenRepository: &mockFCMTokenRepository{}}, fc).
		SendBeGitTime(context.Background(), 12, 345, 7)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if len(fc.withDataCalls) != 0 {
		t.Errorf("expected no push, got %d", len(fc.withDataCalls))
	}
}

// TestPush_BeGitTime_SkipsFinishedChallenge は、Queue の遅延・再送でチャレンジが終わってから届いた begit_time を
// 送らずに成功（ack）にし、1時間以内なら送ることを確認する
func TestPush_BeGitTime_SkipsFinishedChallenge(t *testing.T) {
	sentAt := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	stoppedOrEndedAt := sentAt.Add(10 * time.Minute)
	cases := []struct {
		name     string
		stopped  *time.Time
		ended    *time.Time
		elapsed  time.Duration
		wantSent bool
	}{
		{"発行直後", nil, nil, 5 * time.Second, true},
		{"1時間の1秒前", nil, nil, time.Hour - time.Second, true},
		{"ちょうど1時間", nil, nil, time.Hour, false},
		{"1時間と1秒", nil, nil, time.Hour + time.Second, false},
		{"止めた後", &stoppedOrEndedAt, nil, 20 * time.Minute, false},
		{"中断した後", nil, &stoppedOrEndedAt, 20 * time.Minute, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			notifs := &mockNotificationRepository{getByIDFunc: func(ctx context.Context, id int64) (*model.Notification, error) {
				return &model.Notification{ID: id, SprintID: 7, GameMode: model.GameModeEveryone, SentAt: sentAt, StoppedAt: tc.stopped, EndedAt: tc.ended}, nil
			}}
			tokens := &audienceTokens{mockFCMTokenRepository: &mockFCMTokenRepository{}, byNotification: func(ctx context.Context, id int64) ([]string, error) {
				return []string{"t"}, nil
			}}
			fc := &fakeFCMClient{}
			svc := NewPushNotificationService(&mockUserByID{}, notifs, tokens, fc).(*pushNotificationService)
			svc.now = func() time.Time { return sentAt.Add(tc.elapsed) }
			if err := svc.SendBeGitTime(context.Background(), 12, 345, 7); err != nil {
				t.Fatalf("SendBeGitTime() failed: %v", err)
			}
			if sent := len(fc.withDataCalls) == 1; sent != tc.wantSent {
				t.Errorf("sent=%v, want %v", sent, tc.wantSent)
			}
		})
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
	if err := NewPushNotificationService(users, &mockNotificationRepository{}, &audienceTokens{mockFCMTokenRepository: tokens}, after).SendReaction(context.Background(), 12, 890, 2, 99); err != nil {
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
	if err := NewPushNotificationService(&mockUserByID{}, &mockNotificationRepository{}, &audienceTokens{mockFCMTokenRepository: &mockFCMTokenRepository{}}, fc).SendReaction(context.Background(), 12, 890, 99, 99); err != nil {
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
	err := NewPushNotificationService(users, &mockNotificationRepository{}, &audienceTokens{mockFCMTokenRepository: &mockFCMTokenRepository{}}, fc).SendReaction(context.Background(), 12, 890, 2, 99)
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
			tokens := &audienceTokens{mockFCMTokenRepository: &mockFCMTokenRepository{}, byNotification: func(ctx context.Context, id int64) ([]string, error) { return nil, d1Down }}
			return NewPushNotificationService(&mockUserByID{}, notifWithMode(model.GameModeEveryone), tokens, &fakeFCMClient{}).SendBeGitTime(context.Background(), 12, 345, 7)
		}},
		{"begit_time notification", func() error {
			notifs := &mockNotificationRepository{getByIDFunc: func(ctx context.Context, id int64) (*model.Notification, error) { return nil, d1Down }}
			return NewPushNotificationService(&mockUserByID{}, notifs, &audienceTokens{mockFCMTokenRepository: &mockFCMTokenRepository{}}, &fakeFCMClient{}).SendBeGitTime(context.Background(), 12, 345, 7)
		}},
		{"begit_time fcm", func() error {
			tokens := &audienceTokens{mockFCMTokenRepository: &mockFCMTokenRepository{}, byNotification: func(ctx context.Context, id int64) ([]string, error) { return []string{"t"}, nil }}
			return NewPushNotificationService(&mockUserByID{}, notifWithMode(model.GameModeEveryone), tokens, &failingFCMClient{}).SendBeGitTime(context.Background(), 12, 345, 7)
		}},
		{"reaction actor", func() error {
			users := &mockUserByID{getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) { return nil, d1Down }}
			return NewPushNotificationService(users, &mockNotificationRepository{}, &audienceTokens{mockFCMTokenRepository: &mockFCMTokenRepository{}}, &fakeFCMClient{}).SendReaction(context.Background(), 12, 890, 2, 99)
		}},
		{"reaction tokens", func() error {
			tokens := &mockFCMTokenRepository{getTokensByUserIDFunc: func(ctx context.Context, userID int64) ([]string, error) { return nil, d1Down }}
			return NewPushNotificationService(&mockUserByID{}, &mockNotificationRepository{}, &audienceTokens{mockFCMTokenRepository: tokens}, &fakeFCMClient{}).SendReaction(context.Background(), 12, 890, 2, 99)
		}},
		{"reaction fcm", func() error {
			tokens := &mockFCMTokenRepository{getTokensByUserIDFunc: func(ctx context.Context, userID int64) ([]string, error) { return []string{"t"}, nil }}
			return NewPushNotificationService(&mockUserByID{}, &mockNotificationRepository{}, &audienceTokens{mockFCMTokenRepository: tokens}, &failingFCMClient{}).SendReaction(context.Background(), 12, 890, 2, 99)
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
	svc := NewPushNotificationService(&mockUserByID{}, notifWithMode(model.GameModeEveryone), &audienceTokens{mockFCMTokenRepository: &mockFCMTokenRepository{}}, fc)
	if err := svc.SendBeGitTime(context.Background(), 12, 345, 7); err != nil {
		t.Errorf("SendBeGitTime() with no tokens: %v", err)
	}
	if err := svc.SendReaction(context.Background(), 12, 890, 2, 99); err != nil {
		t.Errorf("SendReaction() with no tokens: %v", err)
	}
	if len(fc.withDataCalls) != 0 {
		t.Errorf("expected no push, got %d", len(fc.withDataCalls))
	}

	noFCM := NewPushNotificationService(&mockUserByID{}, &mockNotificationRepository{}, &audienceTokens{mockFCMTokenRepository: &mockFCMTokenRepository{}}, nil)
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
	tokens := &audienceTokens{
		mockFCMTokenRepository: &mockFCMTokenRepository{
			getTokensByUserIDFunc: func(ctx context.Context, userID int64) ([]string, error) { return []string{"stale"}, nil },
		},
		byNotification: func(ctx context.Context, id int64) ([]string, error) { return []string{"stale"}, nil },
	}
	svc := NewPushNotificationService(&mockUserByID{}, notifWithMode(model.GameModeEveryone), tokens, &permanentFCMClient{})
	if err := svc.SendBeGitTime(context.Background(), 12, 345, 7); !errors.Is(err, ErrPushUndeliverable) {
		t.Errorf("SendBeGitTime: expected ErrPushUndeliverable, got %v", err)
	}
	if err := svc.SendReaction(context.Background(), 12, 890, 2, 99); !errors.Is(err, ErrPushUndeliverable) {
		t.Errorf("SendReaction: expected ErrPushUndeliverable, got %v", err)
	}
}
