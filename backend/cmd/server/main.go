package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
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

	// Cloudflare R2（S3 互換 API 認証情報。CF_API_TOKEN とは別物で R2 ダッシュボードで発行する）
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string

	// Application
	AppBaseURL string

	// CronSecret は内部 Cron エンドポイント（POST /internal/cron）の起動シークレット。
	// Workers scheduled() が X-Cron-Secret ヘッダーで付与する。未設定なら Cron 経路は常に 403。
	CronSecret string

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
		R2AccessKeyID:              os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:          os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:                   os.Getenv("R2_BUCKET"),
		AppBaseURL:                 os.Getenv("APP_BASE_URL"),
		CronSecret:                 os.Getenv("CRON_SECRET"),
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
	if v := r.Header.Get("X-Internal-R2-Access-Key-Id"); v != "" {
		cfg.R2AccessKeyID = v
	}
	if v := r.Header.Get("X-Internal-R2-Secret-Access-Key"); v != "" {
		cfg.R2SecretAccessKey = v
	}
	if v := r.Header.Get("X-Internal-R2-Bucket"); v != "" {
		cfg.R2Bucket = v
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

// @title						BeGit API
// @version					1.0
// @description				BeGit バックエンド API。GitHub と連携したリポジトリ単位のグループ・通知・投稿機能を提供する。
// @BasePath					/
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				`Authorization: Bearer <token>` 形式でアクセストークンを付与する。
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
