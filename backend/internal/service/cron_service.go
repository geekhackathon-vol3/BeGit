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

// Cron delivery kind 定数（notification_deliveries.kind）
const (
	deliveryBeGitTime      = "begit_time"
	deliveryReminder30     = "challenge_reminder_30"
	deliveryReminder10     = "challenge_reminder_10"
	deliveryChallengeEnd   = "challenge_end"
	deliverySprintReminder = "sprint_reminder"
	deliverySprintEnd      = "sprint_end"
	deliverySprintStart    = "sprint_start"
	deliveryReleaseTimeout = 5 * time.Second
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
	notifRepo         repository.NotificationRepository
	sprintRepo        repository.SprintRepository
	groupRepo         repository.GroupRepository
	postRepo          repository.PostRepository
	deliveryRepo      repository.NotificationDeliveryRepository
	fcmTokenRepo      repository.FCMTokenRepository
	fcmClient         fcm.Client
	externalPublisher ExternalNotificationPublisher
	scheduleRepo      repository.BeGitTimeScheduleRepository
	pointsRepo        *repository.PointsRepository
}

func AttachPointsToCron(cron CronService, points *repository.PointsRepository) {
	if s, ok := cron.(*cronService); ok {
		s.pointsRepo = points
	}
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

// NewCronServiceWithPublisher は時刻起点イベントを Push と外部チャネルへ配送する。
func NewCronServiceWithPublisher(
	notifRepo repository.NotificationRepository,
	sprintRepo repository.SprintRepository,
	groupRepo repository.GroupRepository,
	postRepo repository.PostRepository,
	deliveryRepo repository.NotificationDeliveryRepository,
	fcmTokenRepo repository.FCMTokenRepository,
	fcmClient fcm.Client,
	externalPublisher ExternalNotificationPublisher,
	scheduleRepos ...repository.BeGitTimeScheduleRepository,
) CronService {
	result := &cronService{
		notifRepo: notifRepo, sprintRepo: sprintRepo, groupRepo: groupRepo, postRepo: postRepo,
		deliveryRepo: deliveryRepo, fcmTokenRepo: fcmTokenRepo, fcmClient: fcmClient,
		externalPublisher: externalPublisher,
	}
	if len(scheduleRepos) > 0 {
		result.scheduleRepo = scheduleRepos[0]
	}
	return result
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

// runMinutely は自動開催、残り時間リマインド、③ challenge_end を発火する。
func (s *cronService) runMinutely(ctx context.Context) error {
	now := time.Now().UTC()
	if s.pointsRepo != nil {
		if err := s.pointsRepo.FinalizeBonuses(ctx, 0); err != nil {
			log.Printf("cron_service: FinalizeBonuses failed: %v", err)
		}
	}
	s.runScheduledBeGitTimes(ctx, now)
	s.runChallengeReminders(ctx, 30)
	s.runChallengeReminders(ctx, 10)

	due, err := s.notifRepo.ListChallengeEndDue(ctx)
	if err != nil {
		return fmt.Errorf("cron_service: ListChallengeEndDue failed: %w", err)
	}

	for i := range due {
		notif := due[i]
		externalFields := map[string]string{}
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
				onTime, late, missed := countStatuses(statuses)
				externalFields["達成"] = fmt.Sprintf("🌱 %d On Time  ·  %d Late  ·  %d Missed", onTime, late, missed)
			}
		}

		// 全員へ challenge_end を送信（FCM 成功後に delivery INSERT で冪等化）
		if err := s.sendToGroupIfNotSent(ctx, deliveryChallengeEnd, notif.ID, sprint.GroupID, BuildChallengeEnd(sprint.GroupID, notif.ID)); err != nil {
			log.Printf("cron_service: sendToGroupIfNotSent challenge_end %d failed: %v", notif.ID, err)
		}
		s.publishExternal(ctx, deliveryChallengeEnd, notif.ID, sprint.GroupID, externalFields)
	}

	return nil
}

func (s *cronService) runScheduledBeGitTimes(ctx context.Context, now time.Time) {
	if s.scheduleRepo == nil {
		return
	}
	schedules, err := s.scheduleRepo.ListDue(ctx, now)
	if err != nil {
		log.Printf("cron_service: ListDue schedules failed: %v", err)
		return
	}
	for i := range schedules {
		schedule := schedules[i]
		if schedule.NextRunAt == nil {
			continue
		}
		group, err := s.groupRepo.GetByID(ctx, schedule.GroupID)
		if err != nil {
			log.Printf("cron_service: GetByID group %d failed: %v", schedule.GroupID, err)
			continue
		}
		if group.CompletedAt != nil {
			s.advanceSchedule(ctx, schedule, now)
			continue
		}
		durationDays := group.SprintDurationDays
		if durationDays <= 0 {
			durationDays = 7
		}
		sprint, err := s.sprintRepo.GetOrCreateCurrentSprint(ctx, schedule.GroupID, durationDays)
		if err != nil {
			log.Printf("cron_service: GetOrCreateCurrentSprint group %d failed: %v", schedule.GroupID, err)
			continue
		}

		notif, err := s.notifRepo.CreateIfNoActive(ctx, &model.Notification{
			SprintID: sprint.ID, SentBy: group.OwnerUserID,
			Message: "今、なに作ってる？", Source: "scheduled",
		}, true)
		if errors.Is(err, repository.ErrConstraintViolation) {
			// A previous run may have inserted the scheduled challenge before it could advance the schedule.
			// Retry that delivery; a manually started challenge simply causes this occurrence to be skipped.
			active, activeErr := s.notifRepo.GetActiveInSprint(ctx, sprint.ID, now)
			if activeErr == nil && active.Source == "scheduled" {
				notif = active
			} else {
				s.advanceSchedule(ctx, schedule, now)
				continue
			}
		} else if err != nil {
			log.Printf("cron_service: scheduled CreateIfNoActive group %d failed: %v", schedule.GroupID, err)
			continue
		}

		if err := s.sendToGroupIfNotSent(ctx, deliveryBeGitTime, notif.ID, schedule.GroupID, BuildBeGitTime(schedule.GroupID, notif.ID, sprint.ID)); err != nil {
			log.Printf("cron_service: scheduled begit_time delivery %d failed: %v", notif.ID, err)
		} else {
			s.publishExternal(ctx, model.EventBeGitTime, notif.ID, schedule.GroupID, map[string]string{
				"開始時刻": schedule.LocalTime,
			})
		}
		s.advanceSchedule(ctx, schedule, now)
	}
}

func (s *cronService) advanceSchedule(ctx context.Context, schedule model.BeGitTimeSchedule, after time.Time) {
	next, err := NextBeGitTimeRun(after, schedule.LocalTime, schedule.Timezone, schedule.Weekdays)
	if err != nil {
		log.Printf("cron_service: calculate next run group %d failed: %v", schedule.GroupID, err)
		return
	}
	if _, err := s.scheduleRepo.Advance(ctx, schedule.GroupID, *schedule.NextRunAt, next); err != nil {
		log.Printf("cron_service: advance schedule group %d failed: %v", schedule.GroupID, err)
	}
}

func (s *cronService) runChallengeReminders(ctx context.Context, remainingMinutes int) {
	due, err := s.notifRepo.ListChallengeReminderDue(ctx, remainingMinutes)
	if err != nil {
		log.Printf("cron_service: ListChallengeReminderDue remaining=%d failed: %v", remainingMinutes, err)
		return
	}
	kind := deliveryReminder30
	if remainingMinutes == 10 {
		kind = deliveryReminder10
	}
	for i := range due {
		notif := due[i]
		sprint, err := s.sprintRepo.GetByID(ctx, notif.SprintID)
		if err != nil {
			log.Printf("cron_service: reminder sprint %d lookup failed: %v", notif.SprintID, err)
			continue
		}
		if err := s.sendToUnpostedIfNotSent(ctx, kind, notif.ID, sprint.GroupID, BuildChallengeReminder(sprint.GroupID, notif.ID, remainingMinutes)); err != nil {
			log.Printf("cron_service: reminder notif=%d remaining=%d failed: %v", notif.ID, remainingMinutes, err)
		}
	}
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
	s.publishExternal(ctx, kind, sp.ID, sp.GroupID, map[string]string{
		"期間": fmt.Sprintf("%s – %s", sp.StartedAt.Format("1/2"), sp.EndsAt.Format("1/2")),
	})
}

func (s *cronService) publishExternal(ctx context.Context, eventType string, refID, groupID int64, extraFields map[string]string) {
	if s.externalPublisher == nil {
		return
	}
	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		log.Printf("cron_service: external group lookup failed: %v", err)
		return
	}
	title, body := "🌱 BeGit", "チームの新しいお知らせです。"
	fields := make(map[string]string, len(extraFields)+1)
	for key, value := range extraFields {
		fields[key] = value
	}
	switch eventType {
	case model.EventBeGitTime:
		title, body = "🐙 BeGit Time!", "今、なに作ってる？1時間の進捗共有を始めよう。"
	case model.EventChallengeEnd:
		title, body = "✨ BeGit Time終了！", "みんなの1時間が草になりました。結果を見て、お互いの進捗を讃えよう。"
	case model.EventSprintReminder:
		title, body = "🌿 Sprint終了まであと3日", "ラストスパート。小さな進捗も、今日の草にしよう。"
		fields["残り時間"] = "3日"
	case model.EventSprintEnd:
		title, body = "🏁 Sprint完了！", "今週もおつかれさまでした。チームで育てた草を振り返ろう。"
	case model.EventSprintStart:
		title, body = "🌱 New Sprint", "新しいSprintが始まりました。今週も、つくったものを残していこう。"
	}
	event := model.NotificationEvent{
		Key: fmt.Sprintf("%s:%d", eventType, refID), Type: eventType, GroupID: groupID,
		GroupName: group.Name, Title: title, Body: body, AccentColor: "#39D353", Fields: fields, OccurredAt: time.Now().UTC(),
	}
	if err := s.externalPublisher.Publish(ctx, event); err != nil {
		log.Printf("cron_service: external publish failed event=%s: %v", event.Key, err)
	}
}

func countStatuses(statuses []MemberStatus) (onTime, late, missed int) {
	for _, status := range statuses {
		switch status.Status {
		case "On Time":
			onTime++
		case "Late":
			late++
		default:
			missed++
		}
	}
	return
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

// sendToGroupIfNotSent は先に UNIQUE 配信権を取得し、並行 Cron でも一度だけ送る。
// FCM 失敗時は配信権を解放して次回の再試行を許可する。
func (s *cronService) sendToGroupIfNotSent(ctx context.Context, kind string, refID, groupID int64, payload Payload) error {
	alreadySent, err := s.deliveryRepo.MarkSent(ctx, kind, refID)
	if err != nil {
		return fmt.Errorf("delivery claim failed: %w", err)
	}
	if alreadySent {
		return nil
	}

	if s.fcmTokenRepo == nil || s.fcmClient == nil {
		return nil
	}
	tokens, err := s.fcmTokenRepo.GetTokensByGroupID(ctx, groupID)
	if err != nil {
		s.releaseDelivery(kind, refID)
		return fmt.Errorf("GetTokensByGroupID failed: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}

	if err := s.fcmClient.SendToTokensWithData(ctx, tokens, payload.Notification, payload.Data); err != nil {
		log.Printf("cron_service: FCM send failed (type=%s, tokens=%d): %v (will retry next run)", payload.Data["type"], len(tokens), err)
		s.releaseDelivery(kind, refID)
		return fmt.Errorf("FCM send failed: %w", err)
	}
	log.Printf("fcm sent ok (type=%s, tokens=%d)", payload.Data["type"], len(tokens))
	return nil
}

func (s *cronService) sendToUnpostedIfNotSent(ctx context.Context, kind string, refID, groupID int64, payload Payload) error {
	alreadySent, err := s.deliveryRepo.MarkSent(ctx, kind, refID)
	if err != nil {
		return fmt.Errorf("delivery claim failed: %w", err)
	}
	if alreadySent {
		return nil
	}
	if s.fcmTokenRepo == nil || s.fcmClient == nil {
		return nil
	}
	tokens, err := s.fcmTokenRepo.GetTokensByGroupIDExcludingResponders(ctx, groupID, refID)
	if err != nil {
		s.releaseDelivery(kind, refID)
		return fmt.Errorf("GetTokensByGroupIDExcludingResponders failed: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}
	if err := s.fcmClient.SendToTokensWithData(ctx, tokens, payload.Notification, payload.Data); err != nil {
		s.releaseDelivery(kind, refID)
		return fmt.Errorf("FCM send failed: %w", err)
	}
	log.Printf("fcm sent ok (type=%s, tokens=%d)", payload.Data["type"], len(tokens))
	return nil
}

func (s *cronService) releaseDelivery(kind string, refID int64) {
	releaseCtx, cancel := context.WithTimeout(context.Background(), deliveryReleaseTimeout)
	defer cancel()
	if err := s.deliveryRepo.Release(releaseCtx, kind, refID); err != nil {
		log.Printf("cron_service: release delivery kind=%s ref=%d failed: %v", kind, refID, err)
	}
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
