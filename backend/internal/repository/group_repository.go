package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/d1"
)

// groupCreateInput は GroupRepository.Create の入力型（テスト用に公開）
type groupCreateInput struct {
	RepoFullName string
	Name         string
	AvatarURL    string
	OwnerUserID  int64
}

// GroupRepository は groups / group_members テーブルへのアクセスインターフェース
type GroupRepository interface {
	ListByUserID(ctx context.Context, userID int64) ([]model.Group, error)
	Create(ctx context.Context, input *groupCreateInput) (*model.Group, error)
	GetByID(ctx context.Context, groupID int64) (*model.Group, error)
	GetByRepoFullName(ctx context.Context, repoFullName string) (*model.Group, error)
	AddMember(ctx context.Context, groupID, userID int64, role string) error
	BatchAddMembers(ctx context.Context, groupID int64, userIDs []int64, role string) error
	IsMember(ctx context.Context, groupID, userID int64) (bool, error)
	GetMembers(ctx context.Context, groupID int64) ([]model.GroupMember, error)
}

// groupRepository は GroupRepository インターフェースの実装
type groupRepository struct {
	db d1.Client
}

// NewGroupRepository は GroupRepository を作成する
func NewGroupRepository(db d1.Client) GroupRepository {
	return &groupRepository{db: db}
}

// scanGroup は D1 クエリ結果を model.Group に変換する
func scanGroup(row map[string]interface{}) (*model.Group, error) {
	group := &model.Group{}

	if v, ok := row["id"].(float64); ok {
		group.ID = int64(v)
	}
	if v, ok := row["repo_full_name"].(string); ok {
		group.RepoFullName = v
	}
	if v, ok := row["name"].(string); ok {
		group.Name = v
	}
	if v, ok := row["avatar_url"].(string); ok {
		group.AvatarURL = v
	}
	if v, ok := row["owner_user_id"].(float64); ok {
		group.OwnerUserID = int64(v)
	}
	if v, ok := row["sprint_duration_days"].(float64); ok {
		group.SprintDurationDays = int(v)
	}
	if v, ok := row["created_at"].(string); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, _ = time.Parse("2006-01-02 15:04:05", v)
		}
		group.CreatedAt = t
	}

	return group, nil
}

// scanGroupMember は D1 クエリ結果を model.GroupMember に変換する
func scanGroupMember(row map[string]interface{}) model.GroupMember {
	m := model.GroupMember{}
	if v, ok := row["group_id"].(float64); ok {
		m.GroupID = int64(v)
	}
	if v, ok := row["user_id"].(float64); ok {
		m.UserID = int64(v)
	}
	if v, ok := row["login"].(string); ok {
		m.Login = v
	}
	if v, ok := row["avatar_url"].(string); ok {
		m.AvatarURL = v
	}
	if v, ok := row["role"].(string); ok {
		m.Role = v
	}
	if v, ok := row["auto_joined"].(float64); ok {
		m.AutoJoined = v != 0
	}
	return m
}

// ListByUserID は userID が所属する全グループを取得する
func (r *groupRepository) ListByUserID(ctx context.Context, userID int64) ([]model.Group, error) {
	rows, err := r.db.Query(ctx,
		`SELECT g.id, g.repo_full_name, g.name, g.avatar_url, g.owner_user_id, g.sprint_duration_days, g.created_at
		 FROM groups g
		 INNER JOIN group_members gm ON g.id = gm.group_id
		 WHERE gm.user_id = ?`,
		[]interface{}{userID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []model.Group{}, nil
		}
		return nil, fmt.Errorf("group_repository: ListByUserID failed: %w", err)
	}

	groups := make([]model.Group, 0, len(rows))
	for _, row := range rows {
		g, err := scanGroup(row)
		if err != nil {
			return nil, err
		}
		groups = append(groups, *g)
	}
	return groups, nil
}

// Create はグループを作成する
func (r *groupRepository) Create(ctx context.Context, input *groupCreateInput) (*model.Group, error) {
	_, err := r.db.Exec(ctx,
		`INSERT INTO groups (repo_full_name, name, avatar_url, owner_user_id)
		 VALUES (?, ?, ?, ?)`,
		[]interface{}{input.RepoFullName, input.Name, input.AvatarURL, input.OwnerUserID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrConstraintViolation) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("group_repository: Create failed: %w", err)
	}

	return r.GetByRepoFullName(ctx, input.RepoFullName)
}

// GetByID は groupID でグループを取得する
func (r *groupRepository) GetByID(ctx context.Context, groupID int64) (*model.Group, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, repo_full_name, name, avatar_url, owner_user_id, sprint_duration_days, created_at
		 FROM groups WHERE id = ? LIMIT 1`,
		[]interface{}{groupID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("group_repository: GetByID failed: %w", err)
	}

	return scanGroup(rows[0])
}

// GetByRepoFullName は repo_full_name でグループを取得する
func (r *groupRepository) GetByRepoFullName(ctx context.Context, repoFullName string) (*model.Group, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, repo_full_name, name, avatar_url, owner_user_id, sprint_duration_days, created_at
		 FROM groups WHERE repo_full_name = ? LIMIT 1`,
		[]interface{}{repoFullName},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("group_repository: GetByRepoFullName failed: %w", err)
	}

	return scanGroup(rows[0])
}

// AddMember はグループにメンバーを追加する
func (r *groupRepository) AddMember(ctx context.Context, groupID, userID int64, role string) error {
	_, err := r.db.Exec(ctx,
		`INSERT OR IGNORE INTO group_members (group_id, user_id, role) VALUES (?, ?, ?)`,
		[]interface{}{groupID, userID, role},
	)
	if err != nil {
		return fmt.Errorf("group_repository: AddMember failed: %w", err)
	}
	return nil
}

// BatchAddMembers は複数のメンバーをグループに追加する
func (r *groupRepository) BatchAddMembers(ctx context.Context, groupID int64, userIDs []int64, role string) error {
	for _, userID := range userIDs {
		if err := r.AddMember(ctx, groupID, userID, role); err != nil {
			return err
		}
	}
	return nil
}

// IsMember はユーザーがグループのメンバーかどうかを確認する
func (r *groupRepository) IsMember(ctx context.Context, groupID, userID int64) (bool, error) {
	rows, err := r.db.Query(ctx,
		`SELECT COUNT(*) as count FROM group_members WHERE group_id = ? AND user_id = ?`,
		[]interface{}{groupID, userID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("group_repository: IsMember failed: %w", err)
	}

	if len(rows) == 0 {
		return false, nil
	}

	count, _ := rows[0]["count"].(float64)
	return count > 0, nil
}

// GetMembers はグループのメンバー一覧を取得する
func (r *groupRepository) GetMembers(ctx context.Context, groupID int64) ([]model.GroupMember, error) {
	rows, err := r.db.Query(ctx,
		`SELECT gm.group_id, gm.user_id, u.github_login as login, u.avatar_url, gm.role, gm.auto_joined
		 FROM group_members gm
		 INNER JOIN users u ON gm.user_id = u.id
		 WHERE gm.group_id = ?`,
		[]interface{}{groupID},
	)
	if err != nil {
		if errors.Is(err, d1.ErrNotFound) {
			return []model.GroupMember{}, nil
		}
		return nil, fmt.Errorf("group_repository: GetMembers failed: %w", err)
	}

	members := make([]model.GroupMember, 0, len(rows))
	for _, row := range rows {
		members = append(members, scanGroupMember(row))
	}
	return members, nil
}
