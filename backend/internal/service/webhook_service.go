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
	groupRepo   repository.GroupRepository
	sprintRepo  repository.SprintRepository
	niceWorkSvc NiceWorkService // ② Nice Work! 発火への委譲（nil 可）
}

// NewWebhookService は WebhookService を作成する（② 委譲無し。既存配線互換）
func NewWebhookService(
	groupRepo repository.GroupRepository,
	sprintRepo repository.SprintRepository,
) WebhookService {
	return &webhookService{
		groupRepo:  groupRepo,
		sprintRepo: sprintRepo,
	}
}

// NewWebhookServiceWithNiceWork は ② Nice Work! 発火委譲付きの WebhookService を作成する
func NewWebhookServiceWithNiceWork(
	groupRepo repository.GroupRepository,
	sprintRepo repository.SprintRepository,
	niceWorkSvc NiceWorkService,
) WebhookService {
	return &webhookService{
		groupRepo:   groupRepo,
		sprintRepo:  sprintRepo,
		niceWorkSvc: niceWorkSvc,
	}
}

// postTypeForEvent は GitHub イベント種別を posts.post_type にマップする
func postTypeForEvent(eventType string) string {
	switch eventType {
	case "push":
		return "commit"
	case "issues":
		return "issue"
	case "pull_request_review":
		return "review"
	default:
		return "commit"
	}
}

// ProcessWebhook は push / issues(opened) / pull_request_review イベントを処理し、② を駆動する
func (s *webhookService) ProcessWebhook(ctx context.Context, req WebhookRequest) error {
	// 検知対象イベントのみ処理
	if req.EventType != "push" && req.EventType != "issues" && req.EventType != "pull_request_review" {
		return nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return fmt.Errorf("webhook_service: failed to parse payload: %w", err)
	}

	// issues は action=opened のみ対象（Req2.1）
	if req.EventType == "issues" {
		action, _ := payload["action"].(string)
		if action != "opened" {
			return nil
		}
	}

	// ペイロードから repository.full_name を取得
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
			// 対応グループが見つからない場合は 200 OK で終了（エラーにしない、Req8.5）
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

	// ② Nice Work! 発火へ委譲（メンバー判定・anchor・冪等は nicework_service が担う）
	if s.niceWorkSvc != nil {
		senderLogin := senderLoginFromPayload(payload)
		if senderLogin != "" {
			detected := detectedActivity(req.EventType, repoFullName, payload)
			if err := s.niceWorkSvc.HandleActivity(ctx, group.ID, senderLogin, postTypeForEvent(req.EventType), detected); err != nil {
				log.Printf("webhook_service: HandleActivity failed for group %d: %v", group.ID, err)
				// ② 発火の失敗は Webhook 受理を失敗させない（ベストエフォート）
			}
		}
	}

	return nil
}

// senderLoginFromPayload は Webhook ペイロードから送信者 login を取得する
func senderLoginFromPayload(payload map[string]interface{}) string {
	if sender, ok := payload["sender"].(map[string]interface{}); ok {
		if login, ok := sender["login"].(string); ok {
			return login
		}
	}
	return ""
}

// detectedActivity は ② の draft プレフィル用にペイロードから検知データを抽出する
func detectedActivity(eventType, repoFullName string, payload map[string]interface{}) ActivityData {
	data := ActivityData{RepoFullName: repoFullName}

	if eventType == "push" {
		// ref（refs/heads/main）から branch_name を抽出
		if ref, ok := payload["ref"].(string); ok {
			const prefix = "refs/heads/"
			if len(ref) > len(prefix) && ref[:len(prefix)] == prefix {
				data.BranchName = ref[len(prefix):]
			}
		}
		// commits 配列からコミット数と最新メッセージ
		if commits, ok := payload["commits"].([]interface{}); ok {
			data.CommitCount = len(commits)
			if len(commits) > 0 {
				if last, ok := commits[len(commits)-1].(map[string]interface{}); ok {
					if msg, ok := last["message"].(string); ok {
						data.LatestCommitMessage = msg
					}
				}
			}
		}
	}

	return data
}
