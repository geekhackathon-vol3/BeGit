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
	"github.com/irj0927/begit/pkg/externalnotify"
	"github.com/irj0927/begit/pkg/fcm"
	githubpkg "github.com/irj0927/begit/pkg/github"
	"github.com/irj0927/begit/pkg/notificationqueue"
	"github.com/irj0927/begit/pkg/r2"
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

	// GitHub クライアント。
	// GitHub OAuth 認証情報（GITHUB_CLIENT_ID / GITHUB_CLIENT_SECRET）が設定されていれば、
	// DEV_MODE であっても実 GitHub API を使う（dev-stub-user ではなく実ユーザーが返る）。
	// DEV_MODE かつ認証情報が無い場合のみスタブ（GitHub 設定なしでローカル/dev が起動できる）。
	var githubClient githubpkg.Client
	switch {
	case cfg.GitHubClientID != "" && cfg.GitHubClientSecret != "":
		githubClient = githubpkg.NewClient()
		log.Printf("GitHub client configured (real GitHub API)")
	case cfg.DevMode:
		githubClient = githubpkg.NewStubClient()
		log.Printf("DEV_MODE enabled without GitHub credentials: using stub GitHub client")
	default:
		githubClient = githubpkg.NewClient()
	}

	var fcmClient fcm.Client
	if cfg.FirebaseServiceAccountJSON != "" {
		fcmClient, err = fcm.NewClient(context.Background(), cfg.FirebaseServiceAccountJSON)
		if err != nil {
			log.Printf("Warning: failed to create FCM client: %v", err)
		}
	}

	// R2 クライアント。
	// R2 認証情報（R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY）が設定されていれば実 R2 を使う。
	// DEV_MODE かつ認証情報が無い場合のみスタブ（実 R2 なしでローカル/dev が起動できる）。
	// begit-dev で実 R2(begit-photos) を検証する場合は secret を登録する:
	//   cd backend && npx wrangler secret put R2_ACCESS_KEY_ID --env dev
	//   cd backend && npx wrangler secret put R2_SECRET_ACCESS_KEY --env dev
	r2Bucket := cfg.R2Bucket
	if r2Bucket == "" {
		r2Bucket = "begit-photos"
	}
	var r2Client r2.Client
	switch {
	case cfg.R2AccessKeyID != "" && cfg.R2SecretAccessKey != "":
		r2Client = r2.NewClient(cfg.CFAccountID, cfg.R2AccessKeyID, cfg.R2SecretAccessKey, r2Bucket)
		log.Printf("R2 client configured (bucket=%s)", r2Bucket)
	default:
		r2Client = r2.NewStubClient()
		log.Printf("Warning: R2 credentials not set, using stub R2 client (photo upload is disabled)")
	}

	// Repository 層の初期化
	userRepo := repository.NewUserRepository(d1Client)
	groupRepo := repository.NewGroupRepository(d1Client)
	sprintRepo := repository.NewSprintRepository(d1Client)
	notifRepo := repository.NewNotificationRepository(d1Client)
	postRepo := repository.NewPostRepository(d1Client)
	webhookRepo := repository.NewWebhookRepository(d1Client)
	githubAppInstallationRepo := repository.NewGitHubAppInstallationRepository(d1Client)
	fcmTokenRepo := repository.NewFCMTokenRepository(d1Client)
	reactionRepo := repository.NewReactionRepository(d1Client)
	commentRepo := repository.NewCommentRepository(d1Client)
	photoRepo := repository.NewPhotoRepository(d1Client)
	deliveryRepo := repository.NewNotificationDeliveryRepository(d1Client)
	notificationChannelRepo := repository.NewNotificationChannelRepository(d1Client)
	notificationDeliveryJobRepo := repository.NewNotificationDeliveryJobRepository(d1Client)

	externalSender := externalnotify.NewClient()
	queueClient := notificationqueue.NewClient(cfg.AppBaseURL, cfg.NotificationQueueSecret)
	externalPublisher := service.NewExternalNotificationPublisher(notificationChannelRepo, notificationDeliveryJobRepo, queueClient)
	notificationChannelSvc := service.NewNotificationChannelService(notificationChannelRepo, groupRepo, encryptor, externalSender)
	externalDeliverySvc := service.NewExternalNotificationDeliveryService(notificationChannelRepo, notificationDeliveryJobRepo, encryptor, externalSender)

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

	notifSvc := service.NewNotificationServiceFullWithPublisher(
		sprintRepo,
		notifRepo,
		fcmTokenRepo,
		fcmClient,
		groupRepo,
		postRepo,
		cfg.BeGitTimeAllowMultiplePerSprint,
		externalPublisher,
	)

	postSvc := service.NewPostService(githubClient, sprintRepo, postRepo, groupRepo, photoRepo, r2Client, notifRepo)

	photoSvc := service.NewPhotoService(r2Client, photoRepo, postRepo)

	// ② Nice Work! 発火サービス（webhook_service から委譲される）
	niceWorkSvc := service.NewNiceWorkServiceWithPublisher(userRepo, groupRepo, sprintRepo, notifRepo, postRepo, fcmTokenRepo, fcmClient, externalPublisher)

	webhookSvc := service.NewWebhookServiceWithNiceWork(groupRepo, sprintRepo, niceWorkSvc)

	fcmTokenSvc := service.NewFCMTokenService(fcmTokenRepo)

	// ⑦ ソーシャル通知付き（fcm 依存注入）
	reactionSvc := service.NewReactionServiceWithExternalNotifications(reactionRepo, postRepo, userRepo, fcmTokenRepo, fcmClient, groupRepo, externalPublisher)

	commentSvc := service.NewCommentServiceWithExternalNotifications(commentRepo, postRepo, userRepo, fcmTokenRepo, fcmClient, groupRepo, externalPublisher)

	// ③④⑤⑥ Cron サービス
	cronSvc := service.NewCronServiceWithPublisher(notifRepo, sprintRepo, groupRepo, postRepo, deliveryRepo, fcmTokenRepo, fcmClient, externalPublisher)

	appInstallationClient, ok := githubClient.(githubpkg.AppInstallationClient)
	if !ok {
		return nil, fmt.Errorf("GitHub client does not support App installations")
	}
	appInstallationTokenClient, ok := githubClient.(githubpkg.AppInstallationTokenClient)
	if !ok {
		return nil, fmt.Errorf("GitHub client does not support Installation Tokens")
	}
	githubSvc := service.NewGitHubServiceWithAppToken(
		service.GitHubServiceConfig{
			GitHubAppID:         cfg.GitHubAppID,
			GitHubAppPrivateKey: cfg.GitHubAppPrivateKey,
		},
		githubClient,
		appInstallationTokenClient,
		groupRepo,
	)
	groupSvc := service.NewGroupServiceWithAppToken(
		service.GroupServiceConfig{
			AppBaseURL:          cfg.AppBaseURL,
			GitHubWebhookSecret: cfg.GitHubWebhookSecret,
			GitHubAppID:         cfg.GitHubAppID,
			GitHubAppPrivateKey: cfg.GitHubAppPrivateKey,
		},
		githubClient,
		appInstallationTokenClient,
		groupRepo,
		userRepo,
	)
	githubAppInstallationSvc := service.NewGitHubAppInstallationService(
		service.GitHubAppInstallationServiceConfig{
			AppID:         cfg.GitHubAppID,
			PrivateKeyPEM: cfg.GitHubAppPrivateKey,
		},
		appInstallationClient,
		githubAppInstallationRepo,
	)

	// Handler 層の初期化
	authHandler := handler.NewAuthHandler(authSvc)
	groupHandler := handler.NewGroupHandler(groupSvc, notifSvc)
	notifHandler := handler.NewNotificationHandler(notifSvc)
	postHandler := handler.NewPostHandler(postSvc)
	photoHandler := handler.NewPhotoHandler(photoSvc, r2Client)
	webhookHandler := handler.NewWebhookHandler(webhookSvc, webhookRepo, cfg.GitHubWebhookSecret)
	fcmTokenHandler := handler.NewFCMTokenHandler(fcmTokenSvc, authSvc)
	reactionHandler := handler.NewReactionHandler(reactionSvc)
	commentHandler := handler.NewCommentHandler(commentSvc)
	githubHandler := handler.NewGitHubHandler(githubSvc)
	githubAppHandler := handler.NewGitHubAppHandler(githubAppInstallationSvc, cfg.GitHubAppIOSRedirectURI)
	cronHandler := handler.NewCronHandler(cronSvc, cfg.CronSecret)
	notificationChannelHandler := handler.NewNotificationChannelHandler(notificationChannelSvc)
	externalDeliveryHandler := handler.NewNotificationDeliveryHandler(externalDeliverySvc, cfg.NotificationQueueSecret)

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
	r.GET("/github/app/setup", githubAppHandler.Setup)

	// 内部 Cron エンドポイント（bearer 不要。X-Cron-Secret 一致時のみ受理。
	// Workers scheduled() 経由でのみ到達する想定で公開はしない）。
	r.POST("/internal/cron", cronHandler.Run)
	r.POST("/internal/notification-deliveries/:jobId", externalDeliveryHandler.Deliver)

	// dev 専用ログイン（DEV_MODE=true のときだけ登録。false なら未登録＝404）
	if cfg.DevMode {
		devAuthHandler := handler.NewDevAuthHandler(userRepo, encryptor)
		r.POST("/auth/dev", devAuthHandler.DevLogin)
	}

	// Bearer 認証が必要なエンドポイント
	r.GET("/me", bearerAuth, authHandler.Me)
	r.GET("/groups", bearerAuth, groupHandler.List)
	r.POST("/groups", bearerAuth, groupHandler.Create)
	r.DELETE("/groups/:id", bearerAuth, groupHandler.Delete)
	r.PUT("/me/fcm-token", bearerAuth, fcmTokenHandler.Upsert)
	r.POST("/auth/logout", bearerAuth, fcmTokenHandler.Logout)
	r.GET("/github/repos", bearerAuth, githubHandler.ListRepos)

	// グループメンバー確認が必要なエンドポイント
	r.GET("/groups/:id", bearerAuth, groupMember, groupHandler.Get)
	r.POST("/groups/:id/sync-members", bearerAuth, groupMember, groupHandler.SyncMembers)
	r.POST("/groups/:id/notifications", bearerAuth, groupMember, notifHandler.Send)
	r.GET("/groups/:id/notifications/active", bearerAuth, groupMember, notifHandler.GetActive)
	r.GET("/groups/:id/notifications/:nid", bearerAuth, groupMember, notifHandler.GetStatus)
	r.POST("/groups/:id/notifications/:nid/stop", bearerAuth, groupMember, notifHandler.Stop)
	r.POST("/groups/:id/notifications/:nid/end", bearerAuth, groupMember, notifHandler.End)
	r.POST("/groups/:id/posts", bearerAuth, groupMember, postHandler.Create)
	r.GET("/groups/:id/posts", bearerAuth, groupMember, postHandler.List)
	r.DELETE("/groups/:id/posts/:postId", bearerAuth, groupMember, postHandler.Delete)
	r.GET("/groups/:id/posts/:postId/draft", bearerAuth, groupMember, postHandler.GetDraft)
	r.POST("/groups/:id/posts/:postId/confirm", bearerAuth, groupMember, postHandler.Confirm)
	r.POST("/groups/:id/posts/:postId/photos", bearerAuth, groupMember, photoHandler.Upload)
	r.POST("/groups/:id/posts/:postId/reactions", bearerAuth, groupMember, reactionHandler.Create)
	r.DELETE("/groups/:id/posts/:postId/reactions/:reactionType", bearerAuth, groupMember, reactionHandler.Delete)
	r.GET("/groups/:id/posts/:postId/reactions", bearerAuth, groupMember, reactionHandler.List)
	r.POST("/groups/:id/posts/:postId/comments", bearerAuth, groupMember, commentHandler.Create)
	r.GET("/groups/:id/posts/:postId/comments", bearerAuth, groupMember, commentHandler.List)
	r.DELETE("/groups/:id/posts/:postId/comments/:commentId", bearerAuth, groupMember, commentHandler.Delete)
	r.GET("/groups/:id/commits", bearerAuth, groupMember, githubHandler.ListCommits)
	r.GET("/groups/:id/pull-requests", bearerAuth, groupMember, githubHandler.ListPullRequests)
	r.GET("/groups/:id/notification-channels", bearerAuth, groupMember, notificationChannelHandler.List)
	r.POST("/groups/:id/notification-channels", bearerAuth, groupMember, notificationChannelHandler.Create)
	r.PATCH("/groups/:id/notification-channels/:channelId", bearerAuth, groupMember, notificationChannelHandler.Update)
	r.DELETE("/groups/:id/notification-channels/:channelId", bearerAuth, groupMember, notificationChannelHandler.Delete)
	r.POST("/groups/:id/notification-channels/:channelId/test", bearerAuth, groupMember, notificationChannelHandler.Test)

	return r, nil
}
