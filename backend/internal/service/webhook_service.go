package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/irj0927/begit/internal/repository"
)

// WebhookRequest は Webhook イベントのリクエスト情報
type WebhookRequest struct {
	DeliveryID string
	EventType  string
	Payload    []byte
}

// WebhookService は GitHub Webhook イベント処理サービスインターフェース
type WebhookService interface {
	ProcessWebhook(ctx context.Context, req WebhookRequest) error
}

// webhookService は WebhookService インターフェースの実装
type webhookService struct {
	groupRepo  repository.GroupRepository
	sprintRepo repository.SprintRepository
}

// NewWebhookService は WebhookService を作成する
func NewWebhookService(
	groupRepo repository.GroupRepository,
	sprintRepo repository.SprintRepository,
) WebhookService {
	return &webhookService{
		groupRepo:  groupRepo,
		sprintRepo: sprintRepo,
	}
}

// ProcessWebhook は push / pull_request_review イベントを処理する
func (s *webhookService) ProcessWebhook(ctx context.Context, req WebhookRequest) error {
	// push と pull_request_review イベントのみ処理
	if req.EventType != "push" && req.EventType != "pull_request_review" {
		return nil
	}

	// ペイロードから repository.full_name を取得
	var payload map[string]interface{}
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return fmt.Errorf("webhook_service: failed to parse payload: %w", err)
	}

	repoFullName := ""
	if repo, ok := payload["repository"].(map[string]interface{}); ok {
		if name, ok := repo["full_name"].(string); ok {
			repoFullName = name
		}
	}
	if repoFullName == "" {
		log.Printf("webhook_service: no repository.full_name in payload for event %s", req.EventType)
		return nil
	}

	// リポジトリ名から対応するグループを検索
	group, err := s.groupRepo.GetByRepoFullName(ctx, repoFullName)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// 対応グループが見つからない場合は 200 OK で終了（エラーにしない）
			log.Printf("webhook_service: no group found for repo %s, ignoring event", repoFullName)
			return nil
		}
		return fmt.Errorf("webhook_service: GetByRepoFullName failed: %w", err)
	}

	// スプリント情報を更新（GetOrCreate で当日スプリントを確保）
	if s.sprintRepo != nil {
		_, err = s.sprintRepo.GetOrCreateCurrentSprint(ctx, group.ID, group.SprintDurationDays)
		if err != nil {
			log.Printf("webhook_service: GetOrCreateCurrentSprint failed for group %d: %v", group.ID, err)
			// スプリント更新の失敗はエラーにしない（ベストエフォート）
		}
	}

	return nil
}
