package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

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

// Config はサーバー起動に必要な設定を保持する構造体
type Config struct {
	// GitHub OAuth
	GitHubClientID      string
	GitHubClientSecret  string
	GitHubWebhookSecret string

	// Firebase FCM
	FirebaseServiceAccountJSON string

	// Cloudflare D1
	DBEncryptionKey string
	CFAccountID     string
	D1DatabaseID    string
	CFAPIToken      string

	// Application
	AppBaseURL string

	// DevMode が true のとき dev 認証バイパス（POST /auth/dev）と
	// スタブ GitHub クライアントを有効化する。本番では未設定＝false。
	DevMode bool
}

// loadConfig は環境変数から Config を読み込む
// 必須環境変数が欠けている場合はエラーを返す
func loadConfig() (*Config, error) {
	cfg := &Config{
		GitHubClientID:             os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret:         os.Getenv("GITHUB_CLIENT_SECRET"),
		GitHubWebhookSecret:        os.Getenv("GITHUB_WEBHOOK_SECRET"),
		FirebaseServiceAccountJSON: os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON"),
		DBEncryptionKey:            os.Getenv("DB_ENCRYPTION_KEY"),
		CFAccountID:                os.Getenv("CF_ACCOUNT_ID"),
		D1DatabaseID:               os.Getenv("D1_DATABASE_ID"),
		CFAPIToken:                 os.Getenv("CF_API_TOKEN"),
		AppBaseURL:                 os.Getenv("APP_BASE_URL"),
		DevMode:                    os.Getenv("DEV_MODE") == "true",
	}

	// 必須環境変数の検証
	required := map[string]string{
		"GITHUB_CLIENT_ID":              cfg.GitHubClientID,
		"GITHUB_CLIENT_SECRET":          cfg.GitHubClientSecret,
		"GITHUB_WEBHOOK_SECRET":         cfg.GitHubWebhookSecret,
		"FIREBASE_SERVICE_ACCOUNT_JSON": cfg.FirebaseServiceAccountJSON,
		"DB_ENCRYPTION_KEY":             cfg.DBEncryptionKey,
		"CF_ACCOUNT_ID":                 cfg.CFAccountID,
		"D1_DATABASE_ID":                cfg.D1DatabaseID,
		"CF_API_TOKEN":                  cfg.CFAPIToken,
		"APP_BASE_URL":                  cfg.AppBaseURL,
	}

	for name, value := range required {
		if value == "" {
			return nil, fmt.Errorf("required environment variable %s is not set", name)
		}
	}

	return cfg, nil
}

// configFromHeaders は X-Internal-* ヘッダーから Config を更新する（最初のリクエスト時に呼ぶ）
// Workers Secrets は src/index.ts から X-Internal-* ヘッダーとして転送される
func configFromHeaders(r *http.Request, cfg *Config) {
	if v := r.Header.Get("X-Internal-DB-Encryption-Key"); v != "" {
		cfg.DBEncryptionKey = v
	}
	if v := r.Header.Get("X-Internal-Github-Client-Id"); v != "" {
		cfg.GitHubClientID = v
	}
	if v := r.Header.Get("X-Internal-Github-Client-Secret"); v != "" {
		cfg.GitHubClientSecret = v
	}
	if v := r.Header.Get("X-Internal-Github-Webhook-Secret"); v != "" {
		cfg.GitHubWebhookSecret = v
	}
	if v := r.Header.Get("X-Internal-Firebase-Service-Account"); v != "" {
		cfg.FirebaseServiceAccountJSON = v
	}
	if v := r.Header.Get("X-Internal-CF-Account-Id"); v != "" {
		cfg.CFAccountID = v
	}
	if v := r.Header.Get("X-Internal-D1-Database-Id"); v != "" {
		cfg.D1DatabaseID = v
	}
	if v := r.Header.Get("X-Internal-CF-Api-Token"); v != "" {
		cfg.CFAPIToken = v
	}
	if v := r.Header.Get("X-Internal-App-Base-URL"); v != "" {
		cfg.AppBaseURL = v
	}
	if v := r.Header.Get("X-Internal-Dev-Mode"); v != "" {
		cfg.DevMode = v == "true"
	}
}

// server はすべての依存関係を保持するサーバー構造体
type server struct {
	cfg     *Config
	handler http.Handler
	mu      sync.RWMutex
}

//	@title						BeGit API
//	@version					1.0
//	@description				BeGit バックエンド API。GitHub と連携したリポジトリ単位のグループ・通知・投稿機能を提供する。
//	@BasePath					/
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				`Authorization: Bearer <token>` 形式でアクセストークンを付与する。
func main() {
	// 環境変数から設定を読み込む（X-Internal-* ヘッダーでの上書きも許可）
	cfg, err := loadConfig()
	if err != nil {
		// Workers Container 環境では env vars が X-Internal-* ヘッダー経由で来る可能性があるため
		// 起動時のエラーはログのみで続行（最初のリクエスト時に設定）
		log.Printf("Warning: Config load incomplete: %v (will retry from request headers)", err)
		cfg = &Config{}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &server{cfg: cfg}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      srv,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Server starting on port %s", port)
	if err := server.ListenAndServe(); err != nil {
		log.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}

// ServeHTTP はリクエストを処理する（初回は X-Internal-* ヘッダーから設定を補完してからハンドラーを初期化）
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	h := s.handler
	s.mu.RUnlock()

	if h == nil {
		// 初回リクエスト時にハンドラーを初期化
		s.mu.Lock()
		// ダブルチェック
		if s.handler == nil {
			configFromHeaders(r, s.cfg)

			handler, err := s.buildHandler()
			if err != nil {
				s.mu.Unlock()
				log.Printf("Failed to build handler: %v", err)
				http.Error(w, `{"error":"server not initialized"}`, http.StatusInternalServerError)
				return
			}
			s.handler = handler
			log.Printf("Handler initialized successfully")
		}
		h = s.handler
		s.mu.Unlock()
	}

	h.ServeHTTP(w, r)
}

// buildHandler は依存関係を DI で接続してルーターを構築する
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
