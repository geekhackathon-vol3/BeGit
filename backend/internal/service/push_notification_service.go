package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/fcm"
)

// ErrPushUndeliverable は、送り直しても届かない FCM の失敗（fcm.ErrPermanent）
var ErrPushUndeliverable = errors.New("push undeliverable")

// PushNotificationService は Worker が Queue に積んだ FCM 通知を送る（#134）。
// API を Worker に移すと、Worker は通知を Queue に積むだけになる。送信は Queue → Go の裏方のコンテナで行う。
// 送る内容は、Go が API の中で送っていたもの（BuildBeGitTime / BuildReaction）と同じ。
//
// 戻り値のエラーで Queue の扱いが決まる（handler の PushNotificationHandler）:
//   - ErrNotFound: 送り先が決まらない（actor がいない）。再送しても変わらないので捨てる
//   - ErrPushUndeliverable: 全トークンが失効・不正（FCM の 400 / 404）。再送しても変わらないので捨てる
//   - それ以外: 一時的な失敗として再送する
//
// FCM の送信は、全トークンで失敗したときだけエラーにする（fcm.Client と同じ）。一部が届いていれば再送しない。
type PushNotificationService interface {
	// SendBeGitTime は ① begit_time をグループ全員に送る（NotificationService.SendNotification の Step 4 と同じ）
	SendBeGitTime(ctx context.Context, groupID, notificationID, sprintID int64) error
	// SendReaction は ⑦ reaction を投稿者に送る（reactionService.AddReaction の notifyPostAuthor と同じ）
	SendReaction(ctx context.Context, groupID, postID, actorUserID, authorUserID int64) error
}

type pushNotificationService struct {
	userRepo     userByIDRepo
	fcmTokenRepo repository.FCMTokenRepository
	// nil 可。FCM の認証情報がない環境（検証環境など）では送らずに成功扱いにする（Go の API と同じ）
	fcmClient fcm.Client
}

// NewPushNotificationService は PushNotificationService を作成する
func NewPushNotificationService(userRepo userByIDRepo, fcmTokenRepo repository.FCMTokenRepository, fcmClient fcm.Client) PushNotificationService {
	return &pushNotificationService{userRepo: userRepo, fcmTokenRepo: fcmTokenRepo, fcmClient: fcmClient}
}

func (s *pushNotificationService) SendBeGitTime(ctx context.Context, groupID, notificationID, sprintID int64) error {
	if s.fcmClient == nil {
		log.Printf("push: fcm not configured, skipped (type=begit_time, notification_id=%d)", notificationID)
		return nil
	}
	tokens, err := s.fcmTokenRepo.GetTokensByGroupID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("push: GetTokensByGroupID group %d failed: %w", groupID, err)
	}
	return s.send(ctx, tokens, BuildBeGitTime(groupID, notificationID, sprintID))
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
