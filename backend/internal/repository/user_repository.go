package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// UserRepository は users テーブルへのアクセスインターフェース
type UserRepository interface {
	GetByEncryptedToken(ctx context.Context, encryptedToken string) (*model.User, error)
	UpsertUser(ctx context.Context, user *model.User) (*model.User, error)
	GetByGitHubLogin(ctx context.Context, login string) (*model.User, error)
	GetByID(ctx context.Context, id int64) (*model.User, error)
}

// userRepository は UserRepository インターフェースの実装
type userRepository struct {
	db d1.Client
}

// NewUserRepository は UserRepository を作成する
func NewUserRepository(db d1.Client) UserRepository {
	return &userRepository{db: db}
}

// scanUser は D1 クエリ結果を model.User に変換する
func scanUser(row map[string]interface{}) (*model.User, error) {
	user := &model.User{}

	if v, ok := row["id"].(float64); ok {
		user.ID = int64(v)
	}
	if v, ok := row["github_id"].(float64); ok {
		user.GitHubID = int64(v)
	}
	if v, ok := row["github_login"].(string); ok {
		user.GitHubLogin = v
	}
	if v, ok := row["github_name"].(string); ok {
		user.GitHubName = v
	}
	if v, ok := row["avatar_url"].(string); ok {
		user.AvatarURL = v
	}
	if v, ok := row["encrypted_access_token"].(string); ok {
		user.EncryptedAccessToken = v
	}
	if v, ok := row["created_at"].(string); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			// SQLite の datetime('now') は "2006-01-02 15:04:05" 形式の場合もある
			t, _ = time.Parse("2006-01-02 15:04:05", v)
		}
		user.CreatedAt = t
	}

	return user, nil
}

// GetByEncryptedToken は encrypted_access_token でユーザーを検索する
func (r *userRepository) GetByEncryptedToken(ctx context.Context, encryptedToken string) (*model.User, error) {
	rows, err := r.db.Query(ctx,
		"SELECT id, github_id, github_login, github_name, avatar_url, encrypted_access_token, created_at FROM users WHERE encrypted_access_token = ? LIMIT 1",
		[]interface{}{encryptedToken},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("user_repository: GetByEncryptedToken failed: %w", err)
	}

	return scanUser(rows[0])
}

// UpsertUser は users テーブルにユーザーを UPSERT する。
//
// INSERT OR REPLACE は使わない。REPLACE は UNIQUE 衝突時に既存行を
// DELETE+INSERT するため、(1) AUTOINCREMENT の id が振り直されて
// group_members / posts / notifications などの参照が孤立し、
// (2) それらの外部キー参照がある状態では暗黙 DELETE が
// FOREIGN KEY constraint failed で失敗する（=再ログインで 500）。
// github_id を識別子とした ON CONFLICT ... DO UPDATE で既存行を
// 更新し、id を保持する。
func (r *userRepository) UpsertUser(ctx context.Context, user *model.User) (*model.User, error) {
	_, err := r.db.Exec(ctx,
		`INSERT INTO users (github_id, github_login, github_name, avatar_url, encrypted_access_token)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(github_id) DO UPDATE SET
		   github_login           = excluded.github_login,
		   github_name            = excluded.github_name,
		   avatar_url             = excluded.avatar_url,
		   encrypted_access_token = excluded.encrypted_access_token`,
		[]interface{}{user.GitHubID, user.GitHubLogin, user.GitHubName, user.AvatarURL, user.EncryptedAccessToken},
	)
	if err != nil {
		return nil, fmt.Errorf("user_repository: UpsertUser failed: %w", err)
	}

	// 挿入後のレコードを取得して返す
	return r.GetByGitHubLogin(ctx, user.GitHubLogin)
}

// GetByID は id でユーザーを検索する
func (r *userRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	rows, err := r.db.Query(ctx,
		"SELECT id, github_id, github_login, github_name, avatar_url, encrypted_access_token, created_at FROM users WHERE id = ? LIMIT 1",
		[]interface{}{id},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("user_repository: GetByID failed: %w", err)
	}

	return scanUser(rows[0])
}

// GetByGitHubLogin は github_login でユーザーを検索する
func (r *userRepository) GetByGitHubLogin(ctx context.Context, login string) (*model.User, error) {
	rows, err := r.db.Query(ctx,
		"SELECT id, github_id, github_login, github_name, avatar_url, encrypted_access_token, created_at FROM users WHERE github_login = ? LIMIT 1",
		[]interface{}{login},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("user_repository: GetByGitHubLogin failed: %w", err)
	}

	return scanUser(rows[0])
}
