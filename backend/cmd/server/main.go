package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

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
}

// server はすべての依存関係を保持するサーバー構造体
type server struct {
	cfg     *Config
	handler http.Handler
	mu      sync.RWMutex
}

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

	githubClient := githubpkg.NewClient()

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

	// Handler 層の初期化
	authHandler := handler.NewAuthHandler(authSvc)
	groupHandler := handler.NewGroupHandler(groupSvc)
	notifHandler := handler.NewNotificationHandler(notifSvc)
	postHandler := handler.NewPostHandler(postSvc)
	webhookHandler := handler.NewWebhookHandler(webhookSvc, webhookRepo, cfg.GitHubWebhookSecret)
	fcmTokenHandler := handler.NewFCMTokenHandler(fcmTokenSvc)

	// ミドルウェアの初期化
	bearerAuth := handler.BearerAuthMiddleware(userRepo, encryptor)
	groupMember := handler.GroupMemberMiddleware(groupRepo)

	// ルーティング設定（Go 1.22 ServeMux）
	mux := http.NewServeMux()

	// 認証不要エンドポイント
	mux.Handle("POST /auth/github", authHandler)
	mux.Handle("POST /webhook/github", webhookHandler)

	// Bearer 認証が必要なエンドポイント
	mux.Handle("GET /groups", bearerAuth(groupHandler))
	mux.Handle("POST /groups", bearerAuth(groupHandler))

	// グループメンバー確認が必要なエンドポイント
	mux.Handle("GET /groups/{id}", bearerAuth(groupMember(groupHandler)))
	mux.Handle("POST /groups/{id}/notifications", bearerAuth(groupMember(notifHandler)))
	mux.Handle("GET /groups/{id}/notifications/{nid}", bearerAuth(groupMember(notifHandler)))
	mux.Handle("POST /groups/{id}/posts", bearerAuth(groupMember(postHandler)))
	mux.Handle("GET /groups/{id}/posts", bearerAuth(groupMember(postHandler)))
	mux.Handle("PUT /me/fcm-token", bearerAuth(fcmTokenHandler))

	return mux, nil
}
