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

// challengeWindow は ① BeGit Time! のチャレンジ有効期間（発行から1時間）
const challengeWindow = time.Hour

// ActivityData は ② Nice Work! の検知データ（GitHub アクティビティから取得）。
// draft 投稿のプレフィル元になる。
type ActivityData struct {
	CommitCount         int
	Additions           int
	Deletions           int
	RepoFullName        string
	BranchName          string
	LatestCommitMessage string
	// DetectionTime はアクティビティ検知時刻（UTC）。anchor 特定と on_time/late 判定に使用する。
	// ゼロ値の場合は time.Now() で補完される（後方互換）。
	DetectionTime time.Time
}

// userByLoginRepo は login からユーザーを引く最小インターフェース（repository.UserRepository が満たす）
type userByLoginRepo interface {
	GetByGitHubLogin(ctx context.Context, login string) (*model.User, error)
}

// NiceWorkService は ② Nice Work! 発火サービスインターフェース
type NiceWorkService interface {
	// HandleActivity は Webhook 検知アクティビティから ② を発火する。
	// 非メンバー / anchor 無し / 既発火 はいずれも no-op（送信しない、エラーにしない）。
	HandleActivity(ctx context.Context, groupID int64, senderLogin, postType string, detected ActivityData) error
}

// niceWorkService は NiceWorkService インターフェースの実装
type niceWorkService struct {
	userRepo     userByLoginRepo
	groupRepo    repository.GroupRepository
	sprintRepo   repository.SprintRepository
	notifRepo    repository.NotificationRepository
	postRepo     repository.PostRepository
	fcmTokenRepo repository.FCMTokenRepository
	fcmClient    fcm.Client
}

// NewNiceWorkService は NiceWorkService を作成する
func NewNiceWorkService(
	userRepo userByLoginRepo,
	groupRepo repository.GroupRepository,
	sprintRepo repository.SprintRepository,
	notifRepo repository.NotificationRepository,
	postRepo repository.PostRepository,
	fcmTokenRepo repository.FCMTokenRepository,
	fcmClient fcm.Client,
) NiceWorkService {
	return &niceWorkService{
		userRepo:     userRepo,
		groupRepo:    groupRepo,
		sprintRepo:   sprintRepo,
		notifRepo:    notifRepo,
		postRepo:     postRepo,
		fcmTokenRepo: fcmTokenRepo,
		fcmClient:    fcmClient,
	}
}

// HandleActivity は ② を発火する。
func (s *niceWorkService) HandleActivity(ctx context.Context, groupID int64, senderLogin, postType string, detected ActivityData) error {
	if senderLogin == "" {
		return nil
	}

	// Step 1: 送信者 login → user。未登録なら ② 対象外（no-op）。
	user, err := s.userRepo.GetByGitHubLogin(ctx, senderLogin)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("nicework_service: GetByGitHubLogin failed: %w", err)
	}

	// Step 2: メンバー判定。非メンバーは no-op（Req2.2）。
	isMember, err := s.groupRepo.IsMember(ctx, groupID, user.ID)
	if err != nil {
		return fmt.Errorf("nicework_service: IsMember failed: %w", err)
	}
	if !isMember {
		return nil
	}

	// Step 3: 現スプリントを取得（無ければ no-op）。
	sprint, err := s.sprintRepo.GetCurrentSprint(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("nicework_service: GetCurrentSprint failed: %w", err)
	}

	// Step 4: anchor 特定（検知時刻以前で最新の同一スプリント内 BeGit Time! 通知）。
	// anchor 無し（チャレンジ未発行）は no-op（Req2.4）。
	detectionTime := detected.DetectionTime
	if detectionTime.IsZero() {
		// 後方互換: DetectionTime が未設定の場合は time.Now() で補完
		detectionTime = time.Now().UTC()
	}
	anchor, err := s.notifRepo.GetLatestInSprintBefore(ctx, sprint.ID, detectionTime)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("nicework_service: GetLatestInSprintBefore failed: %w", err)
	}

	// Step 5: on_time / late を確定（検知時刻 vs anchor.sent_at + 1h）。
	status := NiceWorkStatusOnTime
	statusStr := "on_time"
	if detectionTime.After(anchor.SentAt.Add(challengeWindow)) {
		status = NiceWorkStatusLate
		statusStr = "late"
	}

	// Step 6: draft INSERT を UNIQUE(notification_id,user_id) で先取り。
	// 違反＝既発火 → 再発火しない（冪等 skip、Req2.6）。
	repoFullName := detected.RepoFullName
	branchName := detected.BranchName
	latestMsg := detected.LatestCommitMessage
	draft := &model.Post{
		NotificationID:      &anchor.ID,
		UserID:              user.ID,
		GroupID:             groupID,
		PostType:            postType,
		RepoFullName:        strOrNil(repoFullName),
		BranchName:          strOrNil(branchName),
		CommitCount:         detected.CommitCount,
		Additions:           detected.Additions,
		Deletions:           detected.Deletions,
		LatestCommitMessage: strOrNil(latestMsg),
		Status:              &statusStr,
	}
	created, err := s.postRepo.CreateDraft(ctx, draft)
	if err != nil {
		if errors.Is(err, repository.ErrConstraintViolation) {
			// 既に当該チャレンジで発火済み → no-op
			return nil
		}
		return fmt.Errorf("nicework_service: CreateDraft failed: %w", err)
	}

	// Step 7: 本人のみへ nice_work data 送信（グループ全体へは送らない、Req2.8）。ベストエフォート。
	if s.fcmTokenRepo != nil && s.fcmClient != nil {
		tokens, err := s.fcmTokenRepo.GetTokensByUserID(ctx, user.ID)
		if err != nil {
			log.Printf("nicework_service: GetTokensByUserID failed for user %d: %v", user.ID, err)
		} else if len(tokens) > 0 {
			payload := BuildNiceWork(groupID, anchor.ID, created.ID, status)
			logFCMSend(payload.Data["type"], len(tokens), s.fcmClient.SendToTokensWithData(ctx, tokens, payload.Notification, payload.Data))
		}
	}

	return nil
}

// strOrNil は空文字を nil に変換する（DB の NULL 用）
func strOrNil(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
