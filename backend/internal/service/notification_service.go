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
	notifRepo repository.NotificationRepository,
	groupRepo repository.GroupRepository,
	postRepo repository.PostRepository,
) NotificationService {
	return &notificationService{
		notifRepo: notifRepo,
		groupRepo: groupRepo,
		postRepo:  postRepo,
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

// SendNotification は現スプリントの取得/作成 → 通知 INSERT → FCM 送信を行う
func (s *notificationService) SendNotification(ctx context.Context, groupID, userID int64) (*model.Notification, error) {
	// Step 1: 現在のスプリントを取得または作成
	sprint, err := s.sprintRepo.GetOrCreateCurrentSprint(ctx, groupID, 7)
	if err != nil {
		return nil, fmt.Errorf("notification_service: get/create sprint failed: %w", err)
	}

	// Step 2: 通知レコードを INSERT（UNIQUE 違反 → ErrConflict）
	notif, err := s.notifRepo.Create(ctx, &model.Notification{
		SprintID: sprint.ID,
		SentBy:   userID,
		Message:  "今なに作ってる？",
	})
	if err != nil {
		if errors.Is(err, repository.ErrConstraintViolation) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("notification_service: create notification failed: %w", err)
	}

	// Step 3: FCM でグループ全メンバーに Push 通知送信（エラーは無視）
	if s.fcmTokenRepo != nil && s.fcmClient != nil {
		tokens, err := s.fcmTokenRepo.GetTokensByGroupID(ctx, groupID)
		if err == nil && len(tokens) > 0 {
			_ = s.fcmClient.SendToTokens(ctx, tokens, fcm.Notification{
				Title: "BeGit Time!",
				Body:  "今なに作ってる？チームへの通知が届きました",
			})
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

	// グループのメンバー一覧を取得
	members, err := s.groupRepo.GetMembers(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("notification_service: GetMembers failed: %w", err)
	}

	// 各メンバーの投稿ステータスを算出
	memberStatuses := make([]MemberStatus, 0, len(members))
	for _, member := range members {
		post, err := s.postRepo.GetByUserAndNotification(ctx, member.UserID, notifID)

		var status string
		if errors.Is(err, repository.ErrNotFound) || post == nil {
			status = "Missed"
		} else if err != nil {
			status = "Missed"
		} else {
			// post.created_at と notif.sent_at + 1h を比較
			deadline := notif.SentAt.Add(3600e9) // 1時間 = 3600秒
			if post.CreatedAt.After(deadline) {
				status = "Late"
			} else {
				status = "On Time"
			}
		}

		memberStatuses = append(memberStatuses, MemberStatus{
			UserID:    member.UserID,
			Login:     member.Login,
			AvatarURL: member.AvatarURL,
			Status:    status,
		})
	}

	return &NotificationStatus{
		NotificationID: notifID,
		Members:        memberStatuses,
	}, nil
}
