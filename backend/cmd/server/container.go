package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/docs"
	"github.com/irj0927/begit/internal/handler"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/internal/service"
	"github.com/irj0927/begit/pkg/crypto"
	"github.com/irj0927/begit/pkg/d1"
	"github.com/irj0927/begit/pkg/fcm"
	githubpkg "github.com/irj0927/begit/pkg/github"
)

// buildHandler は依存関係を接続してルーターを構築する。
// pkg → repository → service → handler → routing の順に配線する。
func (s *server) buildHandler() (http.Handler, error) {
	cfg := s.cfg

	// pkg 層の初期化
	d1Client := d1.NewClient(cfg.CFAccountID, cfg.D1DatabaseID, cfg.CFAPIToken)

	encryptor, err := crypto.NewEncryptor(cfg.DBEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	// DEV_MODE 時は実 GitHub API の代わりにスタブを注入する。
	// これにより auth/group/post の各 service が GitHub 設定・実トークンなしで動作する。
	var githubClient githubpkg.Client
	if cfg.DevMode {
		githubClient = githubpkg.NewStubClient()
		log.Printf("DEV_MODE enabled: using stub GitHub client and /auth/dev endpoint")
	} else {
		githubClient = githubpkg.NewClient()
	}

	var fcmClient fcm.Client
	if cfg.FirebaseServiceAccountJSON != "" {
		fcmClient, err = fcm.NewClient(context.Background(), cfg.FirebaseServiceAccountJSON)
		if err != nil {
			log.Printf("Warning: failed to create FCM client: %v", err)
		}
	}

	// Repository 層の初期化
	userRepo := repository.NewUserRepository(d1Client)
	groupRepo := repository.NewGroupRepository(d1Client)
	sprintRepo := repository.NewSprintRepository(d1Client)
	notifRepo := repository.NewNotificationRepository(d1Client)
	postRepo := repository.NewPostRepository(d1Client)
	webhookRepo := repository.NewWebhookRepository(d1Client)
	fcmTokenRepo := repository.NewFCMTokenRepository(d1Client)
	reactionRepo := repository.NewReactionRepository(d1Client)
	commentRepo := repository.NewCommentRepository(d1Client)

	// Service 層の初期化
	authSvc := service.NewAuthService(
		service.AuthServiceConfig{
			GitHubClientID:     cfg.GitHubClientID,
			GitHubClientSecret: cfg.GitHubClientSecret,
		},
		githubClient,
		userRepo,
		encryptor,
	)

	groupSvc := service.NewGroupService(
		service.GroupServiceConfig{
			AppBaseURL:          cfg.AppBaseURL,
			GitHubWebhookSecret: cfg.GitHubWebhookSecret,
		},
		githubClient,
		groupRepo,
		userRepo,
	)

	notifSvc := service.NewNotificationServiceFull(
		sprintRepo,
		notifRepo,
		fcmTokenRepo,
		fcmClient,
		groupRepo,
		postRepo,
	)

	postSvc := service.NewPostService(githubClient, sprintRepo, postRepo, groupRepo)

	webhookSvc := service.NewWebhookService(groupRepo, sprintRepo)

	fcmTokenSvc := service.NewFCMTokenService(fcmTokenRepo)

	reactionSvc := service.NewReactionService(reactionRepo, postRepo)

	commentSvc := service.NewCommentService(commentRepo, postRepo)

	githubSvc := service.NewGitHubService(githubClient, groupRepo)

	// Handler 層の初期化
	authHandler := handler.NewAuthHandler(authSvc)
	groupHandler := handler.NewGroupHandler(groupSvc)
	notifHandler := handler.NewNotificationHandler(notifSvc)
	postHandler := handler.NewPostHandler(postSvc)
	webhookHandler := handler.NewWebhookHandler(webhookSvc, webhookRepo, cfg.GitHubWebhookSecret)
	fcmTokenHandler := handler.NewFCMTokenHandler(fcmTokenSvc)
	reactionHandler := handler.NewReactionHandler(reactionSvc)
	commentHandler := handler.NewCommentHandler(commentSvc)
	githubHandler := handler.NewGitHubHandler(githubSvc)

	// ミドルウェアの初期化
	bearerAuth := handler.BearerAuth(userRepo, encryptor)
	groupMember := handler.GroupMember(groupRepo)

	// ルーティング設定（gin）
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	// パスは一致するがメソッドが異なる場合は 404 ではなく 405 を返す（従来挙動を維持）
	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusMethodNotAllowed, handler.ErrorResponse{Error: "method not allowed"})
	})

	// ヘルスチェック（疎通確認・warmup 用、常時有効）
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API ドキュメント（OpenAPI 3.1 仕様の配信 + Swagger UI）
	r.GET("/openapi.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", docs.SwaggerJSON)
	})
	r.GET("/openapi.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", docs.SwaggerYAML)
	})
	r.GET("/docs", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(docs.SwaggerUIHTML))
	})

	// 認証不要エンドポイント
	r.POST("/auth/github", authHandler.GitHub)
	r.POST("/webhook/github", webhookHandler.Receive)

	// dev 専用ログイン（DEV_MODE=true のときだけ登録。false なら未登録＝404）
	if cfg.DevMode {
		devAuthHandler := handler.NewDevAuthHandler(userRepo, encryptor)
		r.POST("/auth/dev", devAuthHandler.DevLogin)
	}

	// Bearer 認証が必要なエンドポイント
	r.GET("/groups", bearerAuth, groupHandler.List)
	r.POST("/groups", bearerAuth, groupHandler.Create)
	r.PUT("/me/fcm-token", bearerAuth, fcmTokenHandler.Upsert)
	r.POST("/auth/logout", bearerAuth, fcmTokenHandler.Logout)
	r.GET("/github/repos", bearerAuth, githubHandler.ListRepos)

	// グループメンバー確認が必要なエンドポイント
	r.GET("/groups/:id", bearerAuth, groupMember, groupHandler.Get)
	r.POST("/groups/:id/sync-members", bearerAuth, groupMember, groupHandler.SyncMembers)
	r.POST("/groups/:id/notifications", bearerAuth, groupMember, notifHandler.Send)
	r.GET("/groups/:id/notifications/:nid", bearerAuth, groupMember, notifHandler.GetStatus)
	r.POST("/groups/:id/posts", bearerAuth, groupMember, postHandler.Create)
	r.GET("/groups/:id/posts", bearerAuth, groupMember, postHandler.List)
	r.POST("/groups/:id/posts/:postId/reactions", bearerAuth, groupMember, reactionHandler.Create)
	r.DELETE("/groups/:id/posts/:postId/reactions/:reactionType", bearerAuth, groupMember, reactionHandler.Delete)
	r.GET("/groups/:id/posts/:postId/reactions", bearerAuth, groupMember, reactionHandler.List)
	r.POST("/groups/:id/posts/:postId/comments", bearerAuth, groupMember, commentHandler.Create)
	r.GET("/groups/:id/posts/:postId/comments", bearerAuth, groupMember, commentHandler.List)
	r.DELETE("/groups/:id/posts/:postId/comments/:commentId", bearerAuth, groupMember, commentHandler.Delete)
	r.GET("/groups/:id/commits", bearerAuth, groupMember, githubHandler.ListCommits)

	return r, nil
}
