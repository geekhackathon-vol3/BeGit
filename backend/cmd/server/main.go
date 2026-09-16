package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

	// GitHub App credentials. GitHubAppPrivateKey is the decoded PEM value.
	GitHubAppID         string
	GitHubAppPrivateKey string

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
	AppBaseURL              string
	GitHubAppIOSRedirectURI string

	// CronSecret は内部 Cron エンドポイント（POST /internal/cron）の起動シークレット。
	// Workers scheduled() が X-Cron-Secret ヘッダーで付与する。未設定なら Cron 経路は常に 403。
	CronSecret string

	// BeGitTimeAllowMultiplePerSprint が true のとき BeGit Time! の「1スプリント1人1回」を適用しない。
	// dev での動作確認用。未設定（本番）は従来どおり制限あり。1時間の時間的非共存ルールは常に適用する。
	BeGitTimeAllowMultiplePerSprint bool

	// DevMode が true のとき dev 認証バイパス（POST /auth/dev）と
	// スタブ GitHub クライアントを有効化する。本番では未設定＝false。
	DevMode bool
}

// loadConfig は環境変数から Config を読み込む
// 必須環境変数が欠けている場合はエラーを返す
func loadConfig() (*Config, error) {
	cfg := &Config{
		GitHubClientID:                  os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret:              os.Getenv("GITHUB_CLIENT_SECRET"),
		GitHubWebhookSecret:             os.Getenv("GITHUB_WEBHOOK_SECRET"),
		GitHubAppID:                     os.Getenv("GITHUB_APP_ID"),
		GitHubAppPrivateKey:             os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		FirebaseServiceAccountJSON:      os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON"),
		DBEncryptionKey:                 os.Getenv("DB_ENCRYPTION_KEY"),
		CFAccountID:                     os.Getenv("CF_ACCOUNT_ID"),
		D1DatabaseID:                    os.Getenv("D1_DATABASE_ID"),
		CFAPIToken:                      os.Getenv("CF_API_TOKEN"),
		R2AccessKeyID:                   os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:               os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:                        os.Getenv("R2_BUCKET"),
		AppBaseURL:                      os.Getenv("APP_BASE_URL"),
		GitHubAppIOSRedirectURI:         os.Getenv("GITHUB_APP_IOS_REDIRECT_URI"),
		CronSecret:                      os.Getenv("CRON_SECRET"),
		DevMode:                         os.Getenv("DEV_MODE") == "true",
		BeGitTimeAllowMultiplePerSprint: os.Getenv("BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT") == "true",
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
	if v := r.Header.Get("X-Internal-Github-App-Id"); v != "" {
		cfg.GitHubAppID = v
	}
	if v := r.Header.Get("X-Internal-Github-App-Private-Key-B64"); v != "" {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			log.Printf("Warning: failed to decode GitHub App private key header: %v", err)
		} else {
			cfg.GitHubAppPrivateKey = string(decoded)
		}
	}
	if v := r.Header.Get("X-Internal-Firebase-Service-Account-B64"); v != "" {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			log.Printf("Warning: failed to decode Firebase service account header: %v", err)
		} else {
			cfg.FirebaseServiceAccountJSON = string(decoded)
		}
	} else if v := r.Header.Get("X-Internal-Firebase-Service-Account"); v != "" {
		// Backward compatibility for an older Worker deployment. New deployments
		// use the Base64 header above because JSON private keys may contain newlines.
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
	if v := r.Header.Get("X-Internal-Github-App-Ios-Redirect-Uri"); v != "" {
		cfg.GitHubAppIOSRedirectURI = v
	}
	if v := r.Header.Get("X-Internal-Cron-Secret"); v != "" {
		cfg.CronSecret = v
	}
	if v := r.Header.Get("X-Internal-Dev-Mode"); v != "" {
		cfg.DevMode = v == "true"
	}
	if v := r.Header.Get("X-Internal-Begit-Time-Allow-Multiple-Per-Sprint"); v != "" {
		cfg.BeGitTimeAllowMultiplePerSprint = v == "true"
	}
}

// internalConfigHeaders は configFromHeaders が読む X-Internal-* ヘッダー名の一覧。
// ハンドラー再構築の要否判定（fingerprint）はこの一覧だけを対象にし、
// 任意の X-Internal-* を付けたリクエストで再構築を連発させられないようにする。
var internalConfigHeaders = []string{
	"X-Internal-DB-Encryption-Key",
	"X-Internal-Github-Client-Id",
	"X-Internal-Github-Client-Secret",
	"X-Internal-Github-Webhook-Secret",
	"X-Internal-Github-App-Id",
	"X-Internal-Github-App-Private-Key-B64",
	"X-Internal-Firebase-Service-Account-B64",
	"X-Internal-Firebase-Service-Account",
	"X-Internal-CF-Account-Id",
	"X-Internal-D1-Database-Id",
	"X-Internal-CF-Api-Token",
	"X-Internal-R2-Access-Key-Id",
	"X-Internal-R2-Secret-Access-Key",
	"X-Internal-R2-Bucket",
	"X-Internal-App-Base-URL",
	"X-Internal-Github-App-Ios-Redirect-Uri",
	"X-Internal-Cron-Secret",
	"X-Internal-Dev-Mode",
	"X-Internal-Begit-Time-Allow-Multiple-Per-Sprint",
}

// configFingerprint は X-Internal-* ヘッダーの内容から設定の同一性を表すハッシュを返す。
// 該当ヘッダーが1つも無い（Worker を経由しないローカル起動など）場合は空文字を返す。
func configFingerprint(r *http.Request) string {
	h := sha256.New()
	found := false
	for _, name := range internalConfigHeaders {
		v := r.Header.Get(name)
		if v != "" {
			found = true
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write([]byte(v))
		h.Write([]byte{0})
	}
	if !found {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// server はすべての依存関係を保持するサーバー構造体
type server struct {
	// baseCfg は起動時に環境変数から読んだ設定。再構築のたびにここからヘッダーを重ねる。
	baseCfg Config
	cfg     *Config
	handler http.Handler
	// fingerprint は現在の handler を構築したときの configFingerprint。
	fingerprint string
	// newHandler はテスト用の差し替え口。nil なら buildHandler を使う。
	newHandler func() (http.Handler, error)
	mu         sync.RWMutex
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

	srv := &server{baseCfg: *cfg, cfg: cfg}

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

// ServeHTTP はリクエストを処理する。
// X-Internal-* ヘッダーから設定を補完してハンドラーを初期化し、以降はヘッダーの内容
// （Workers Secrets / vars）が変わったときだけ再構築する。コンテナは Cron で常時起動し続けるため、
// 再構築しないと `wrangler secret put` した値が再起動まで反映されない（FCM が送られない等）。
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fp := configFingerprint(r)

	s.mu.RLock()
	h := s.handler
	upToDate := h != nil && (fp == "" || fp == s.fingerprint)
	s.mu.RUnlock()

	if !upToDate {
		s.mu.Lock()
		// ダブルチェック
		if s.handler == nil || (fp != "" && fp != s.fingerprint) {
			if err := s.rebuildLocked(r, fp); err != nil && s.handler == nil {
				s.mu.Unlock()
				log.Printf("Failed to build handler: %v", err)
				http.Error(w, `{"error":"server not initialized"}`, http.StatusInternalServerError)
				return
			}
		}
		h = s.handler
		s.mu.Unlock()
	}

	h.ServeHTTP(w, r)
}

// rebuildLocked は起動時設定にヘッダーを重ねた Config でハンドラーを構築し直す（s.mu を保持して呼ぶ）。
// 既存ハンドラーがある状態で構築に失敗した場合は既存を使い続け、同じ設定での再試行を毎リクエスト繰り返さない。
func (s *server) rebuildLocked(r *http.Request, fp string) error {
	cfg := s.baseCfg
	configFromHeaders(r, &cfg)

	prevCfg := s.cfg
	s.cfg = &cfg
	build := s.newHandler
	if build == nil {
		build = s.buildHandler
	}
	handler, err := build()
	if err != nil {
		s.cfg = prevCfg
		if s.handler != nil {
			s.fingerprint = fp
			log.Printf("Warning: failed to rebuild handler with updated config, keeping previous one: %v", err)
		}
		return err
	}

	if s.handler == nil {
		log.Printf("Handler initialized successfully")
	} else {
		log.Printf("Handler rebuilt: internal config changed")
	}
	s.handler = handler
	s.fingerprint = fp
	return nil
}
