package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/fcm"
)

// Cron delivery kind 定数（notification_deliveries.kind）
const (
	deliveryChallengeEnd   = "challenge_end"
	deliverySprintReminder = "sprint_reminder"
	deliverySprintEnd      = "sprint_end"
	deliverySprintStart    = "sprint_start"
)

// ErrInvalidCronKind は kind が不正な場合に返す（400 にマップ）
var ErrInvalidCronKind = errors.New("invalid cron kind")

// CronService は ③④⑤⑥ の時刻起点通知を kind 別に冪等発火するサービス
type CronService interface {
	// RunCron は kind（"minutely" | "daily"）に応じて時刻起点通知を発火する。
	// kind が不正な場合は ErrInvalidCronKind を返す。
	RunCron(ctx context.Context, kind string) error
}

// cronService は CronService インターフェースの実装
type cronService struct {
	notifRepo    repository.NotificationRepository
	sprintRepo   repository.SprintRepository
	groupRepo    repository.GroupRepository
	postRepo     repository.PostRepository
	deliveryRepo repository.NotificationDeliveryRepository
	fcmTokenRepo repository.FCMTokenRepository
	fcmClient    fcm.Client
}

// NewCronService は CronService を作成する
func NewCronService(
	notifRepo repository.NotificationRepository,
	sprintRepo repository.SprintRepository,
	groupRepo repository.GroupRepository,
	postRepo repository.PostRepository,
	deliveryRepo repository.NotificationDeliveryRepository,
	fcmTokenRepo repository.FCMTokenRepository,
	fcmClient fcm.Client,
) CronService {
	return &cronService{
		notifRepo:    notifRepo,
		sprintRepo:   sprintRepo,
		groupRepo:    groupRepo,
		postRepo:     postRepo,
		deliveryRepo: deliveryRepo,
		fcmTokenRepo: fcmTokenRepo,
		fcmClient:    fcmClient,
	}
}

// RunCron は kind に応じて通知を発火する
func (s *cronService) RunCron(ctx context.Context, kind string) error {
	switch kind {
	case "minutely":
		return s.runMinutely(ctx)
	case "daily":
		return s.runDaily(ctx)
	default:
		return ErrInvalidCronKind
	}
}

// runMinutely は ③ challenge_end を発火する（発行+1h 到達かつ未送信の anchor）。
func (s *cronService) runMinutely(ctx context.Context) error {
	due, err := s.notifRepo.ListChallengeEndDue(ctx)
	if err != nil {
		return fmt.Errorf("cron_service: ListChallengeEndDue failed: %w", err)
	}

	for i := range due {
		notif := due[i]
		sprint, err := s.sprintRepo.GetByID(ctx, notif.SprintID)
		if err != nil {
			log.Printf("cron_service: GetByID sprint %d failed: %v", notif.SprintID, err)
			continue
		}

		// サマリ算出（On Time/Late/Missed）。③ 時点では missed を永続化しない（確定は ⑤）。
		members, err := s.groupRepo.GetMembers(ctx, sprint.GroupID)
		if err != nil {
			log.Printf("cron_service: GetMembers group %d failed: %v", sprint.GroupID, err)
		} else {
			statuses, err := computeMemberStatuses(ctx, s.postRepo, &notif, members)
			if err != nil {
				log.Printf("cron_service: computeMemberStatuses notif=%d failed: %v", notif.ID, err)
			} else {
				log.Printf("cron_service: challenge_end summary notif=%d %s", notif.ID, summarize(statuses))
			}
		}

		// 全員へ challenge_end を送信（FCM 成功後に delivery INSERT で冪等化）
		if err := s.sendToGroupIfNotSent(ctx, deliveryChallengeEnd, notif.ID, sprint.GroupID, BuildChallengeEnd(sprint.GroupID, notif.ID)); err != nil {
			log.Printf("cron_service: sendToGroupIfNotSent challenge_end %d failed: %v", notif.ID, err)
		}
	}

	return nil
}

// runDaily は ④ sprint_reminder / ⑤ sprint_end / ⑥ sprint_start を発火する。
func (s *cronService) runDaily(ctx context.Context) error {
	// ④ 終了3日前リマインダー
	reminders, err := s.sprintRepo.ListReminderDue(ctx)
	if err != nil {
		return fmt.Errorf("cron_service: ListReminderDue failed: %w", err)
	}
	for _, sp := range reminders {
		s.fireSprintNotification(ctx, deliverySprintReminder, sp, BuildSprintReminder(sp.GroupID, sp.ID))
	}

	// ⑤ 終了通知（missed 確定 → まとめ → sprint_end）
	ended, err := s.sprintRepo.ListEnded(ctx)
	if err != nil {
		return fmt.Errorf("cron_service: ListEnded failed: %w", err)
	}
	for _, sp := range ended {
		// sprint_end が既に配信済みなら missed 確定もスキップ（delivery 冪等で「1回のみ実行」を担保）。
		// 未ガードだと終了スプリントが ListEnded に残り続ける限り毎日 missed 確定が走り、
		// 二重起動・連日実行で冗長な書き込みが発生する。
		alreadySent, err := s.deliveryRepo.HasBeenSent(ctx, deliverySprintEnd, sp.ID)
		if err != nil {
			log.Printf("cron_service: HasBeenSent sprint_end %d failed: %v", sp.ID, err)
			continue
		}
		if alreadySent {
			continue
		}
		// missed 確定（⑤ で初めて永続化）
		s.finalizeMissed(ctx, sp)
		// FCM 送信成功後に delivery INSERT で冪等化
		if err := s.sendToGroupIfNotSent(ctx, deliverySprintEnd, sp.ID, sp.GroupID, BuildSprintEnd(sp.GroupID, sp.ID)); err != nil {
			log.Printf("cron_service: sendToGroupIfNotSent sprint_end %d failed: %v", sp.ID, err)
		}
	}

	// ⑥ 新スプリント開始（アクティブスプリント。冪等は deliveries で担保）
	active, err := s.sprintRepo.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("cron_service: ListActive failed: %w", err)
	}
	for _, sp := range active {
		s.fireSprintNotification(ctx, deliverySprintStart, sp, BuildSprintStart(sp.GroupID, sp.ID))
	}

	return nil
}

// fireSprintNotification は FCM 送信成功後に delivery INSERT で冪等化する。
func (s *cronService) fireSprintNotification(ctx context.Context, kind string, sp model.Sprint, payload Payload) {
	if err := s.sendToGroupIfNotSent(ctx, kind, sp.ID, sp.GroupID, payload); err != nil {
		log.Printf("cron_service: sendToGroupIfNotSent %s sprint %d failed: %v", kind, sp.ID, err)
	}
}

// finalizeMissed は終了スプリントの各通知について未投稿メンバーを missed として確定する。
func (s *cronService) finalizeMissed(ctx context.Context, sp model.Sprint) {
	notifs, err := s.notifRepo.ListBySprintID(ctx, sp.ID)
	if err != nil {
		log.Printf("cron_service: ListBySprintID %d failed: %v", sp.ID, err)
		return
	}
	members, err := s.groupRepo.GetMembers(ctx, sp.GroupID)
	if err != nil {
		log.Printf("cron_service: GetMembers group %d failed: %v", sp.GroupID, err)
		return
	}
	for _, n := range notifs {
		for _, m := range members {
			// 既に投稿/draft がある場合は UNIQUE 違反で skip（CreateMissed が ErrConstraintViolation）
			if err := s.postRepo.CreateMissed(ctx, n.ID, m.UserID, sp.GroupID); err != nil {
				if !errors.Is(err, repository.ErrConstraintViolation) {
					log.Printf("cron_service: CreateMissed notif=%d user=%d failed: %v", n.ID, m.UserID, err)
				}
			}
		}
	}
}

// sendToGroup はグループ全員へ data メッセージを送信する（ベストエフォート）。
func (s *cronService) sendToGroup(ctx context.Context, groupID int64, payload Payload) {
	if s.fcmTokenRepo == nil || s.fcmClient == nil {
		return
	}
	tokens, err := s.fcmTokenRepo.GetTokensByGroupID(ctx, groupID)
	if err != nil {
		log.Printf("cron_service: GetTokensByGroupID group %d failed: %v", groupID, err)
		return
	}
	if len(tokens) == 0 {
		return
	}
	logFCMSend(payload.Data["type"], len(tokens), s.fcmClient.SendToTokensWithData(ctx, tokens, payload.Notification, payload.Data))
}

// sendToGroupIfNotSent は配信済みチェック → FCM 送信 → MarkSent の順で実行し、FCM 成功時のみ配信記録を残す。
// 既送信の場合は何もせず nil を返す（冪等）。
func (s *cronService) sendToGroupIfNotSent(ctx context.Context, kind string, refID, groupID int64, payload Payload) error {
	// Step 1: 配信済みチェック（SELECT で確認）
	alreadySent, err := s.deliveryRepo.HasBeenSent(ctx, kind, refID)
	if err != nil {
		return fmt.Errorf("HasBeenSent check failed: %w", err)
	}
	if alreadySent {
		// 既に送信済み（冪等 skip）
		return nil
	}

	// Step 2: FCM 送信を試行
	if s.fcmTokenRepo == nil || s.fcmClient == nil {
		// FCM 未設定の場合は送信スキップ（配信記録も残さない = 次回リトライ可能）
		return nil
	}
	tokens, err := s.fcmTokenRepo.GetTokensByGroupID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("GetTokensByGroupID failed: %w", err)
	}
	if len(tokens) == 0 {
		// トークン無しの場合も送信スキップ（配信記録を残さない = 次回リトライ可能）
		return nil
	}

	// Step 3: FCM 送信
	if err := s.fcmClient.SendToTokensWithData(ctx, tokens, payload.Notification, payload.Data); err != nil {
		log.Printf("cron_service: FCM send failed (type=%s, tokens=%d): %v (will retry next run)", payload.Data["type"], len(tokens), err)
		return fmt.Errorf("FCM send failed: %w", err)
	}

	// Step 4: FCM 送信成功後に配信記録を残す（次回 skip される）
	if _, err := s.deliveryRepo.MarkSent(ctx, kind, refID); err != nil {
		log.Printf("cron_service: MarkSent after successful send failed (type=%s): %v (sent but not marked, may duplicate)", payload.Data["type"], err)
		return fmt.Errorf("MarkSent failed: %w", err)
	}

	log.Printf("fcm sent ok (type=%s, tokens=%d)", payload.Data["type"], len(tokens))
	return nil
}

// summarize は集計結果を簡潔な文字列にする（ログ用）。
func summarize(statuses []MemberStatus) string {
	var onTime, late, missed int
	for _, st := range statuses {
		switch st.Status {
		case "On Time":
			onTime++
		case "Late":
			late++
		default:
			missed++
		}
	}
	return fmt.Sprintf("OnTime=%d Late=%d Missed=%d", onTime, late, missed)
}
