package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// Config はサーバー起動に必要な設定を保持する構造体
type Config struct {
	// GitHub OAuth
	GitHubClientID     string
	GitHubClientSecret string
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
		GitHubClientID:              os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret:          os.Getenv("GITHUB_CLIENT_SECRET"),
		GitHubWebhookSecret:         os.Getenv("GITHUB_WEBHOOK_SECRET"),
		FirebaseServiceAccountJSON:  os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON"),
		DBEncryptionKey:             os.Getenv("DB_ENCRYPTION_KEY"),
		CFAccountID:                 os.Getenv("CF_ACCOUNT_ID"),
		D1DatabaseID:                os.Getenv("D1_DATABASE_ID"),
		CFAPIToken:                  os.Getenv("CF_API_TOKEN"),
		AppBaseURL:                  os.Getenv("APP_BASE_URL"),
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

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Printf("Configuration error: %v", err)
		os.Exit(1)
	}

	_ = cfg // 後続タスクでルーティング・DI を組み込む

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "BeGit API")
	})

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
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
