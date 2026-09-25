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

// NotificationStatus は通知ステータス情報
type NotificationStatus struct {
	NotificationID int64
	Members        []MemberStatus
}

// ActiveNotification は現在進行中のBeGit Time情報。
type ActiveNotification struct {
	NotificationID int64
	SentBy         int64
	SentAt         time.Time
	ExpiresAt      time.Time
}

// MemberStatus はメンバーごとの投稿ステータス
type MemberStatus struct {
	UserID    int64
	Login     string
	AvatarURL string
	Status    string // "On Time" | "Late" | "Missed"
}

// ActiveChallenge はグループで進行中の BeGit Time チャレンジと、要求ユーザー自身の投稿状況
type ActiveChallenge struct {
	Notification model.Notification
	// EndsAt は締め切り（sent_at + 1h）。進行中なので ended_at は常に nil
	EndsAt time.Time
	// MyPost は要求ユーザーのこの通知への投稿（draft 含む）。無ければ nil
	MyPost *model.Post
}

// NotificationService は BeGit Time 通知サービスインターフェース
type NotificationService interface {
	SendNotification(ctx context.Context, groupID, userID int64) (*model.Notification, error)
	GetActiveNotification(ctx context.Context, groupID int64) (*ActiveNotification, error)
	GetNotificationStatus(ctx context.Context, notifID, groupID int64) (*NotificationStatus, error)
	StopNotification(ctx context.Context, groupID, notifID, userID int64) error
	// GetActiveChallenge はグループで進行中のチャレンジを返す。進行中が無ければ (nil, nil)。
	GetActiveChallenge(ctx context.Context, groupID, userID int64) (*ActiveChallenge, error)
	// EndChallenge は発行者が進行中のチャレンジを途中中断する（締め切りを今にする）。
	// 通知がグループに属さない → ErrNotFound、発行者以外 → ErrForbidden、進行中でない → ErrConflict。
	EndChallenge(ctx context.Context, groupID, notifID, userID int64) (*model.Notification, error)
}

// notificationService は NotificationService インターフェースの実装
type notificationService struct {
	sprintRepo        repository.SprintRepository
	notifRepo         repository.NotificationRepository
	fcmTokenRepo      repository.FCMTokenRepository
	fcmClient         fcm.Client
	groupRepo         repository.GroupRepository
	postRepo          repository.PostRepository
	externalPublisher ExternalNotificationPublisher
	// allowMultiplePerSprint が true なら「1スプリント1人1回」を適用しない（BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT）。
	// ゼロ値 false = 制限あり。1時間の時間的非共存ルールは常に適用する。
	allowMultiplePerSprint bool
}

// NewNotificationService は NotificationService を作成する（SendNotification 用）
func NewNotificationService(
	sprintRepo repository.SprintRepository,
	notifRepo repository.NotificationRepository,
	fcmTokenRepo repository.FCMTokenRepository,
	fcmClient fcm.Client,
) NotificationService {
	return &notificationService{
		sprintRepo:   sprintRepo,
		notifRepo:    notifRepo,
		fcmTokenRepo: fcmTokenRepo,
		fcmClient:    fcmClient,
	}
}

// NewNotificationServiceWithGroupRepo は GetNotificationStatus も含むフル実装を作成する
func NewNotificationServiceWithGroupRepo(
	sprintRepo repository.SprintRepository,
	notifRepo repository.NotificationRepository,
	groupRepo repository.GroupRepository,
	postRepo repository.PostRepository,
) NotificationService {
	return &notificationService{
		sprintRepo: sprintRepo,
		notifRepo:  notifRepo,
		groupRepo:  groupRepo,
		postRepo:   postRepo,
	}
}

// NewNotificationServiceFull は全依存関係を持つ NotificationService を作成する
func NewNotificationServiceFull(
	sprintRepo repository.SprintRepository,
	notifRepo repository.NotificationRepository,
	fcmTokenRepo repository.FCMTokenRepository,
	fcmClient fcm.Client,
	groupRepo repository.GroupRepository,
	postRepo repository.PostRepository,
	allowMultiplePerSprint bool,
) NotificationService {
	return &notificationService{
		sprintRepo:             sprintRepo,
		notifRepo:              notifRepo,
		fcmTokenRepo:           fcmTokenRepo,
		fcmClient:              fcmClient,
		groupRepo:              groupRepo,
		postRepo:               postRepo,
		allowMultiplePerSprint: allowMultiplePerSprint,
	}
}

// NewNotificationServiceFullWithPublisher はスマホ Push に加えて外部チャネルへも通知する。
func NewNotificationServiceFullWithPublisher(
	sprintRepo repository.SprintRepository,
	notifRepo repository.NotificationRepository,
	fcmTokenRepo repository.FCMTokenRepository,
	fcmClient fcm.Client,
	groupRepo repository.GroupRepository,
	postRepo repository.PostRepository,
	allowMultiplePerSprint bool,
	externalPublisher ExternalNotificationPublisher,
) NotificationService {
	return &notificationService{
		sprintRepo: sprintRepo, notifRepo: notifRepo, fcmTokenRepo: fcmTokenRepo, fcmClient: fcmClient,
		groupRepo: groupRepo, postRepo: postRepo, allowMultiplePerSprint: allowMultiplePerSprint,
		externalPublisher: externalPublisher,
	}
}

// SendNotification は現スプリントの取得/作成 → 時間非共存判定 → 通知 INSERT → FCM 送信を行う
func (s *notificationService) SendNotification(ctx context.Context, groupID, userID int64) (*model.Notification, error) {
	// Step 1: 現在のスプリントを取得または作成
	sprint, err := s.sprintRepo.GetOrCreateCurrentSprint(ctx, groupID, 7)
	if err != nil {
		return nil, fmt.Errorf("notification_service: get/create sprint failed: %w", err)
	}

	// Step 2 & 3: 時間的非共存（+ 設定次第で1スプリント1人1回）を原子的に保証する CREATE。
	// CreateIfNoActive は同一スプリント内にアクティブ通知が無く、allowMultiplePerSprint が false なら
	// 同一ユーザーの発行済み通知も無い場合のみ INSERT する（WHERE NOT EXISTS で原子的）。
	notif, err := s.notifRepo.CreateIfNoActive(ctx, &model.Notification{
		SprintID: sprint.ID,
		SentBy:   userID,
		Message:  "今、なに作ってる？",
	}, s.allowMultiplePerSprint)
	if err != nil {
		if errors.Is(err, repository.ErrConstraintViolation) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("notification_service: create notification failed: %w", err)
	}

	// Step 4: FCM でグループ全メンバーに begit_time data メッセージ送信（ベストエフォート）
	if s.fcmTokenRepo != nil && s.fcmClient != nil {
		tokens, err := s.fcmTokenRepo.GetTokensByGroupID(ctx, groupID)
		if err == nil && len(tokens) > 0 {
			payload := BuildBeGitTime(groupID, notif.ID, sprint.ID)
			logFCMSend(payload.Data["type"], len(tokens), s.fcmClient.SendToTokensWithData(ctx, tokens, payload.Notification, payload.Data))
		}
	}

	// 外部通知は Push と独立したベストエフォート経路。ジョブ作成後は Queue が再試行する。
	if s.externalPublisher != nil && s.groupRepo != nil {
		group, groupErr := s.groupRepo.GetByID(ctx, groupID)
		if groupErr == nil {
			issuer := "チームメンバー"
			if members, membersErr := s.groupRepo.GetMembers(ctx, groupID); membersErr == nil {
				for _, member := range members {
					if member.UserID == userID {
						issuer = "@" + member.Login
						break
					}
				}
			}
			event := model.NotificationEvent{
				Key: fmt.Sprintf("begit_time:%d", notif.ID), Type: model.EventBeGitTime,
				GroupID: groupID, GroupName: group.Name, Title: "⏰ BeGit Time!",
				Body:        "今、なに作ってる？ 1時間だけ、みんなで進捗を草にしよう。",
				AccentColor: "#39D353", Fields: map[string]string{"発行者": issuer, "残り時間": "60分"},
				OccurredAt: notif.SentAt,
			}
			if publishErr := s.externalPublisher.Publish(ctx, event); publishErr != nil {
				log.Printf("notification_service: external publish failed event=%s: %v", event.Key, publishErr)
			}
		}
	}

	return notif, nil
}

// GetActiveNotification はグループの現在進行中のBeGit Timeを返す。
func (s *notificationService) GetActiveNotification(ctx context.Context, groupID int64) (*ActiveNotification, error) {
	sprint, err := s.sprintRepo.GetCurrentSprint(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("notification_service: get current sprint failed: %w", err)
	}

	notif, err := s.notifRepo.GetActiveInSprint(ctx, sprint.ID, time.Now().UTC())
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("notification_service: get active notification failed: %w", err)
	}

	return &ActiveNotification{
		NotificationID: notif.ID,
		SentBy:         notif.SentBy,
		SentAt:         notif.SentAt,
		ExpiresAt:      notif.SentAt.Add(challengeWindow),
	}, nil
}

// StopNotification は通知発行者本人のBeGit Timeを停止する。
func (s *notificationService) StopNotification(ctx context.Context, groupID, notifID, userID int64) error {
	notif, err := s.notifRepo.GetByID(ctx, notifID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("notification_service: get notification for stop failed: %w", err)
	}

	sprint, err := s.sprintRepo.GetByID(ctx, notif.SprintID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("notification_service: get sprint for stop failed: %w", err)
	}
	if sprint.GroupID != groupID {
		return ErrNotFound
	}
	if notif.SentBy != userID {
		return ErrForbidden
	}
	if notif.StoppedAt != nil || !time.Now().UTC().Before(notif.SentAt.Add(challengeWindow)) {
		return ErrConflict
	}

	if err := s.notifRepo.Stop(ctx, notifID, userID); err != nil {
		if errors.Is(err, repository.ErrConstraintViolation) {
			return ErrConflict
		}
		return fmt.Errorf("notification_service: stop notification failed: %w", err)
	}
	return nil
}

// GetNotificationStatus はメンバーごとの投稿ステータス（On Time/Late/Missed）を算出する
func (s *notificationService) GetNotificationStatus(ctx context.Context, notifID, groupID int64) (*NotificationStatus, error) {
	// 通知を取得
	notif, err := s.notifRepo.GetByID(ctx, notifID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("notification_service: GetByID failed: %w", err)
	}

	// スプリントを取得してグループIDを確認
	sprint, err := s.sprintRepo.GetByID(ctx, notif.SprintID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("notification_service: GetSprintByID failed: %w", err)
	}

	// 通知のグループが要求されたグループと一致するか確認
	if sprint.GroupID != groupID {
		return nil, ErrNotFound
	}

	// グループのメンバー一覧を取得
	members, err := s.groupRepo.GetMembers(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("notification_service: GetMembers failed: %w", err)
	}

	// 各メンバーの投稿ステータスを算出（③/⑤ Cron サマリと共通）
	memberStatuses, err := computeMemberStatuses(ctx, s.postRepo, notif, members)
	if err != nil {
		return nil, fmt.Errorf("notification_service: computeMemberStatuses failed: %w", err)
	}

	return &NotificationStatus{
		NotificationID: notifID,
		Members:        memberStatuses,
	}, nil
}

// GetActiveChallenge はグループで進行中のチャレンジと要求ユーザーの投稿状況を返す。進行中が無ければ (nil, nil)。
func (s *notificationService) GetActiveChallenge(ctx context.Context, groupID, userID int64) (*ActiveChallenge, error) {
	notif, err := s.notifRepo.GetActiveByGroup(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("notification_service: GetActiveByGroup failed: %w", err)
	}

	active := &ActiveChallenge{
		Notification: *notif,
		EndsAt:       challengeDeadline(notif),
	}

	if s.postRepo != nil {
		post, err := s.postRepo.GetByUserAndNotification(ctx, userID, notif.ID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("notification_service: GetByUserAndNotification failed: %w", err)
		}
		if err == nil {
			active.MyPost = post
		}
	}

	return active, nil
}

// EndChallenge は発行者が進行中のチャレンジを途中中断する（締め切りを今にする）。
func (s *notificationService) EndChallenge(ctx context.Context, groupID, notifID, userID int64) (*model.Notification, error) {
	notif, err := s.notifRepo.GetByID(ctx, notifID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("notification_service: GetByID failed: %w", err)
	}

	// 通知が要求されたグループに属するか（GetNotificationStatus と同じ判定）
	sprint, err := s.sprintRepo.GetByID(ctx, notif.SprintID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("notification_service: GetSprintByID failed: %w", err)
	}
	if sprint.GroupID != groupID {
		return nil, ErrNotFound
	}

	// 中断できるのは発行者のみ
	if notif.SentBy != userID {
		return nil, ErrForbidden
	}

	ended, err := s.notifRepo.EndIfActive(ctx, notifID)
	if err != nil {
		return nil, fmt.Errorf("notification_service: EndIfActive failed: %w", err)
	}
	if !ended {
		// 既に中断済み、または1時間経過済み
		return nil, ErrConflict
	}

	updated, err := s.notifRepo.GetByID(ctx, notifID)
	if err != nil {
		return nil, fmt.Errorf("notification_service: GetByID after end failed: %w", err)
	}
	return updated, nil
}

// computeMemberStatuses は1通知に対する各メンバーの On Time / Late / Missed を算出する。
// GetNotificationStatus（API）と Cron（③/⑤ サマリ）で共通利用する（Req3.4）。
// 判定基準: post.created_at <= 締め切り → On Time、超過 → Late、投稿無し → Missed。
// 締め切りは notif.sent_at + 1h、発行者が途中中断した場合は ended_at（challengeDeadline）。
// repository エラーは errors.Is(err, repository.ErrNotFound) のみ Missed にマップし、それ以外はエラーを返す。
func computeMemberStatuses(
	ctx context.Context,
	postRepo repository.PostRepository,
	notif *model.Notification,
	members []model.GroupMember,
) ([]MemberStatus, error) {
	deadline := challengeDeadline(notif)
	statuses := make([]MemberStatus, 0, len(members))
	for _, member := range members {
		post, err := postRepo.GetByUserAndNotification(ctx, member.UserID, notif.ID)

		var status string
		if err != nil {
			// ErrNotFound は "Missed"（投稿無し）にマップ。それ以外のエラーは呼び出し側へ伝播。
			if errors.Is(err, repository.ErrNotFound) {
				status = "Missed"
			} else {
				return nil, fmt.Errorf("GetByUserAndNotification for user %d failed: %w", member.UserID, err)
			}
		} else if post == nil {
			status = "Missed"
		} else if post.Status != nil && *post.Status == "missed" {
			status = "Missed"
		} else if post.CreatedAt.After(deadline) {
			status = "Late"
		} else {
			status = "On Time"
		}

		statuses = append(statuses, MemberStatus{
			UserID:    member.UserID,
			Login:     member.Login,
			AvatarURL: member.AvatarURL,
			Status:    status,
		})
	}
	return statuses, nil
}
