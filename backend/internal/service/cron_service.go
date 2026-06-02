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

		// 冪等: delivery INSERT が成功した場合のみ送信。再実行は UNIQUE 違反で skip。
		alreadySent, err := s.deliveryRepo.MarkSent(ctx, deliveryChallengeEnd, notif.ID)
		if err != nil {
			log.Printf("cron_service: MarkSent challenge_end %d failed: %v", notif.ID, err)
			continue
		}
		if alreadySent {
			continue
		}

		// サマリ算出（On Time/Late/Missed）。③ 時点では missed を永続化しない（確定は ⑤）。
		members, err := s.groupRepo.GetMembers(ctx, sprint.GroupID)
		if err != nil {
			log.Printf("cron_service: GetMembers group %d failed: %v", sprint.GroupID, err)
		} else {
			statuses := computeMemberStatuses(ctx, s.postRepo, &notif, members)
			log.Printf("cron_service: challenge_end summary notif=%d %s", notif.ID, summarize(statuses))
		}

		// 全員へ challenge_end（ベストエフォート）
		s.sendToGroup(ctx, sprint.GroupID, BuildChallengeEnd(sprint.GroupID, notif.ID))
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
		alreadySent, err := s.deliveryRepo.MarkSent(ctx, deliverySprintEnd, sp.ID)
		if err != nil {
			log.Printf("cron_service: MarkSent sprint_end %d failed: %v", sp.ID, err)
			continue
		}
		if alreadySent {
			continue
		}
		// missed 確定（⑤ で初めて永続化）
		s.finalizeMissed(ctx, sp)
		s.sendToGroup(ctx, sp.GroupID, BuildSprintEnd(sp.GroupID, sp.ID))
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

// fireSprintNotification は delivery INSERT 成功時のみグループ全員へ送信する（冪等）。
func (s *cronService) fireSprintNotification(ctx context.Context, kind string, sp model.Sprint, payload Payload) {
	alreadySent, err := s.deliveryRepo.MarkSent(ctx, kind, sp.ID)
	if err != nil {
		log.Printf("cron_service: MarkSent %s sprint %d failed: %v", kind, sp.ID, err)
		return
	}
	if alreadySent {
		return
	}
	s.sendToGroup(ctx, sp.GroupID, payload)
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
	_ = s.fcmClient.SendToTokensWithData(ctx, tokens, payload.Notification, payload.Data)
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
