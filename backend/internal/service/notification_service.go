package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/fcm"
)

// NotificationStatus は通知ステータス情報
type NotificationStatus struct {
	NotificationID int64
	Members        []MemberStatus
}

// MemberStatus はメンバーごとの投稿ステータス
type MemberStatus struct {
	UserID    int64
	Login     string
	AvatarURL string
	Status    string // "On Time" | "Late" | "Missed"
}

// NotificationService は BeGit Time 通知サービスインターフェース
type NotificationService interface {
	SendNotification(ctx context.Context, groupID, userID int64) (*model.Notification, error)
	GetNotificationStatus(ctx context.Context, notifID, groupID int64) (*NotificationStatus, error)
}

// notificationService は NotificationService インターフェースの実装
type notificationService struct {
	sprintRepo   repository.SprintRepository
	notifRepo    repository.NotificationRepository
	fcmTokenRepo repository.FCMTokenRepository
	fcmClient    fcm.Client
	groupRepo    repository.GroupRepository
	postRepo     repository.PostRepository
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
) NotificationService {
	return &notificationService{
		sprintRepo:   sprintRepo,
		notifRepo:    notifRepo,
		fcmTokenRepo: fcmTokenRepo,
		fcmClient:    fcmClient,
		groupRepo:    groupRepo,
		postRepo:     postRepo,
	}
}

// SendNotification は現スプリントの取得/作成 → 時間非共存判定 → 通知 INSERT → FCM 送信を行う
func (s *notificationService) SendNotification(ctx context.Context, groupID, userID int64) (*model.Notification, error) {
	// Step 1: 現在のスプリントを取得または作成
	sprint, err := s.sprintRepo.GetOrCreateCurrentSprint(ctx, groupID, 7)
	if err != nil {
		return nil, fmt.Errorf("notification_service: get/create sprint failed: %w", err)
	}

	// Step 2 & 3: 時間的非共存 + UNIQUE 制約を原子的に保証する CREATE。
	// CreateIfNoActive は同一スプリント内にアクティブ通知が無く、かつ UNIQUE(sprint_id,sent_by) を
	// 満たす場合のみ INSERT する（WHERE NOT EXISTS で原子的）。
	notif, err := s.notifRepo.CreateIfNoActive(ctx, &model.Notification{
		SprintID: sprint.ID,
		SentBy:   userID,
		Message:  "今、なに作ってる？",
	})
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

	return notif, nil
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

// computeMemberStatuses は1通知に対する各メンバーの On Time / Late / Missed を算出する。
// GetNotificationStatus（API）と Cron（③/⑤ サマリ）で共通利用する（Req3.4）。
// 判定基準: post.created_at <= notif.sent_at + 1h → On Time、超過 → Late、投稿無し → Missed。
// repository エラーは errors.Is(err, repository.ErrNotFound) のみ Missed にマップし、それ以外はエラーを返す。
func computeMemberStatuses(
	ctx context.Context,
	postRepo repository.PostRepository,
	notif *model.Notification,
	members []model.GroupMember,
) ([]MemberStatus, error) {
	deadline := notif.SentAt.Add(challengeWindow)
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
