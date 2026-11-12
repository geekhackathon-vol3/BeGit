package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/fcm"
)

// ErrPushUndeliverable は、送り直しても届かない FCM の失敗（fcm.ErrPermanent）
var ErrPushUndeliverable = errors.New("push undeliverable")

// PushNotificationService は Worker が Queue に積んだ FCM 通知を送る（#134）。
// API を Worker に移すと、Worker は通知を Queue に積むだけになる。送信は Queue → Go の裏方のコンテナで行う。
// 送る内容は、Go が API の中で送っていたもの（BuildBeGitTimeWithMode / BuildReaction）と同じ。
//
// 戻り値のエラーで Queue の扱いが決まる（handler の PushNotificationHandler）:
//   - ErrNotFound: 送り先が決まらない（通知・actor がいない）。再送しても変わらないので捨てる
//   - ErrPushUndeliverable: 全トークンが失効・不正（FCM の 400 / 404）。再送しても変わらないので捨てる
//   - それ以外: 一時的な失敗として再送する
//
// begit_time は、届いた時点でチャレンジが終わっていれば（止めた・中断した・発行から1時間）送らずに nil を返す。
//
// FCM の送信は、全トークンで失敗したときだけエラーにする（fcm.Client と同じ）。一部が届いていれば再送しない。
type PushNotificationService interface {
	// SendBeGitTime は ① begit_time を通知の対象メンバー（notification_members）に、通知のゲームモードの文言で送る
	// （NotificationService.SendNotification の Step 4 と同じ）
	SendBeGitTime(ctx context.Context, groupID, notificationID, sprintID int64) error
	// SendReaction は ⑦ reaction を投稿者に送る（reactionService.AddReaction の notifyPostAuthor と同じ）
	SendReaction(ctx context.Context, groupID, postID, actorUserID, authorUserID int64) error
}

// notificationByIDRepo は SendBeGitTime が通知のゲームモードを読むのに使う
type notificationByIDRepo interface {
	GetByID(ctx context.Context, notifID int64) (*model.Notification, error)
}

// PushTokenRepository は Queue の通知の宛先のトークンを引く。begit_time は発行時に固定した対象メンバー
// （0016 のトリガーが作る notification_members）、reaction は投稿者。本番の fcmTokenRepository は両方を満たす。
// グループ全員を引くメソッドは持たせない（spotlight の対象外にも届いてしまうため）
type PushTokenRepository interface {
	GetTokensByUserID(ctx context.Context, userID int64) ([]string, error)
	GetTokensByNotificationID(ctx context.Context, notificationID int64) ([]string, error)
}

type pushNotificationService struct {
	userRepo     userByIDRepo
	notifRepo    notificationByIDRepo
	fcmTokenRepo PushTokenRepository
	// nil 可。FCM の認証情報がない環境（検証環境など）では送らずに成功扱いにする（Go の API と同じ）
	fcmClient fcm.Client
	// now は今の時刻（テストで差し替える）
	now func() time.Time
}

// NewPushNotificationService は PushNotificationService を作成する
func NewPushNotificationService(userRepo userByIDRepo, notifRepo notificationByIDRepo, fcmTokenRepo PushTokenRepository, fcmClient fcm.Client) PushNotificationService {
	return &pushNotificationService{userRepo: userRepo, notifRepo: notifRepo, fcmTokenRepo: fcmTokenRepo, fcmClient: fcmClient, now: time.Now}
}

func (s *pushNotificationService) SendBeGitTime(ctx context.Context, groupID, notificationID, sprintID int64) error {
	if s.fcmClient == nil {
		log.Printf("push: fcm not configured, skipped (type=begit_time, notification_id=%d)", notificationID)
		return nil
	}
	// ゲームモードは Queue のメッセージに載せず、D1 の通知から読む（正は notifications.game_mode の1か所）
	notif, err := s.notifRepo.GetByID(ctx, notificationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("push: GetByID notification %d failed: %w", notificationID, err)
	}
	// Queue の遅延・再送で、チャレンジが終わってから届くことがある（API の中で送っていたときは起きなかった）。
	// 止めた・中断した・発行から1時間たった（進行中の判定 datetime(sent_at, '+1 hour') > now と同じ境界）通知には送らずに成功にする（ack）
	if reason := beGitTimeOver(notif, s.now()); reason != "" {
		log.Printf("push: begit_time skipped (notification_id=%d, reason=%s)", notificationID, reason)
		return nil
	}
	// 宛先は発行時に固定した対象メンバー（notification_members。通知の INSERT と同じ文でトリガーが作る）。0人なら送らずに成功
	tokens, err := s.fcmTokenRepo.GetTokensByNotificationID(ctx, notificationID)
	if err != nil {
		return fmt.Errorf("push: GetTokensByNotificationID notification %d failed: %w", notificationID, err)
	}
	if notif.SprintID != sprintID {
		log.Printf("push: begit_time sprint_id differs (message=%d, notification=%d), using the notification's", sprintID, notif.SprintID)
	}
	return s.send(ctx, tokens, BuildBeGitTimeWithMode(groupID, notificationID, notif.SprintID, notif.GameMode))
}

func (s *pushNotificationService) SendReaction(ctx context.Context, groupID, postID, actorUserID, authorUserID int64) error {
	// 自己抑制（notifyPostAuthor と同じ）。Worker も積まないが、念のためここでも見る
	if actorUserID == authorUserID {
		return nil
	}
	if s.fcmClient == nil {
		log.Printf("push: fcm not configured, skipped (type=reaction, post_id=%d)", postID)
		return nil
	}
	actor, err := s.userRepo.GetByID(ctx, actorUserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("push: GetByID actor %d failed: %w", actorUserID, err)
	}
	tokens, err := s.fcmTokenRepo.GetTokensByUserID(ctx, authorUserID)
	if err != nil {
		return fmt.Errorf("push: GetTokensByUserID author %d failed: %w", authorUserID, err)
	}
	return s.send(ctx, tokens, BuildReaction(groupID, postID, actor.GitHubLogin))
}

func (s *pushNotificationService) send(ctx context.Context, tokens []string, payload Payload) error {
	if len(tokens) == 0 {
		return nil
	}
	err := s.fcmClient.SendToTokensWithData(ctx, tokens, payload.Notification, payload.Data)
	logFCMSend(payload.Data["type"], len(tokens), err)
	if errors.Is(err, fcm.ErrPermanent) {
		return fmt.Errorf("%w: %w", ErrPushUndeliverable, err)
	}
	return err
}

// beGitTimeOver はチャレンジが終わっていればその理由を返す。進行中なら ""
func beGitTimeOver(notif *model.Notification, now time.Time) string {
	switch {
	case notif.StoppedAt != nil:
		return "stopped"
	case notif.EndedAt != nil:
		return "ended"
	case !now.Before(notif.SentAt.Add(challengeWindow)):
		return "expired"
	}
	return ""
}
