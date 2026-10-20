package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/irj0927/begit/pkg/d1"
)

type FinalMemberPoints struct {
	UserID       int64  `json:"user_id"`
	Login        string `json:"login"`
	AvatarURL    string `json:"avatar_url"`
	ReportPoints int    `json:"report_points"`
	BonusPoints  int    `json:"bonus_points"`
	TotalPoints  int    `json:"total_points"`
}

type FinalResult struct {
	GroupID     int64               `json:"group_id"`
	CompletedAt string              `json:"completed_at"`
	Members     []FinalMemberPoints `json:"members"`
	TeamTotal   int                 `json:"team_total"`
}

type PointsRepository struct{ db d1.Client }

func NewPointsRepository(db d1.Client) *PointsRepository { return &PointsRepository{db: db} }

// FinalizeBonuses is safe to repeat. Only a finished, unstopped challenge with
// every captured member's confirmed on-time post can insert the unique award.
func (r *PointsRepository) FinalizeBonuses(ctx context.Context, groupID int64) error {
	_, err := r.db.Exec(ctx, `INSERT OR IGNORE INTO point_awards(group_id, notification_id, user_id, kind, points)
 SELECT s.group_id, n.id, n.sent_by, 'all_on_time_bonus', 10
 FROM notifications n JOIN sprints s ON s.id = n.sprint_id
 JOIN groups g ON g.id = s.group_id
 WHERE (? = 0 OR s.group_id = ?) AND g.completed_at IS NULL
   AND n.stopped_at IS NULL
   AND datetime(COALESCE(n.ended_at, datetime(n.sent_at, '+1 hour'))) <= datetime('now')
   AND NOT EXISTS (
     SELECT 1 FROM point_awards pa
     WHERE pa.notification_id = n.id AND pa.user_id = n.sent_by
       AND pa.kind = 'all_on_time_bonus'
   )
   AND EXISTS (SELECT 1 FROM notification_members nm WHERE nm.notification_id = n.id)
   AND NOT EXISTS (
     SELECT 1 FROM notification_members nm
     WHERE nm.notification_id = n.id AND NOT EXISTS (
       SELECT 1 FROM posts p WHERE p.notification_id = n.id AND p.user_id = nm.user_id
         AND p.is_draft = 0 AND p.status = 'on_time'
         AND datetime(p.created_at) <= datetime(COALESCE(n.ended_at, datetime(n.sent_at, '+1 hour')))
     )
   )`, []interface{}{groupID, groupID})
	if err != nil {
		return fmt.Errorf("points: finalize bonuses: %w", err)
	}
	return nil
}

// Complete is one conditional UPDATE. The final-result trigger snapshots all
// members and balances in that same SQLite statement.
func (r *PointsRepository) Complete(ctx context.Context, groupID int64) error {
	changed, err := r.db.Exec(ctx, `UPDATE groups SET completed_at = datetime('now')
 WHERE id = ? AND completed_at IS NULL AND NOT EXISTS (
   SELECT 1 FROM notifications n JOIN sprints s ON s.id = n.sprint_id
   WHERE s.group_id = groups.id AND n.stopped_at IS NULL AND n.ended_at IS NULL
     AND datetime(n.sent_at, '+1 hour') > datetime('now')
 )`, []interface{}{groupID})
	if err != nil {
		return fmt.Errorf("points: complete: %w", err)
	}
	if changed == 0 {
		rows, err := r.db.Query(ctx, `SELECT completed_at FROM groups WHERE id = ?`, []interface{}{groupID})
		if errors.Is(err, d1.ErrNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if rows[0]["completed_at"] == nil {
			return ErrConflict
		}
	}
	return nil
}

func (r *PointsRepository) Result(ctx context.Context, groupID int64) (*FinalResult, error) {
	rows, err := r.db.Query(ctx, `SELECT completed_at FROM groups WHERE id = ?`, []interface{}{groupID})
	if errors.Is(err, d1.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	completedAt, ok := rows[0]["completed_at"].(string)
	if !ok || completedAt == "" {
		return nil, ErrNotFound
	}
	result := &FinalResult{GroupID: groupID, CompletedAt: completedAt, Members: []FinalMemberPoints{}}
	members, err := r.db.Query(ctx, `SELECT user_id, login, avatar_url, report_points, bonus_points
 FROM final_result_members WHERE group_id = ? ORDER BY (report_points + bonus_points) DESC, login COLLATE NOCASE`, []interface{}{groupID})
	if errors.Is(err, d1.ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for _, row := range members {
		m := FinalMemberPoints{UserID: int64(number(row["user_id"])), Login: stringValue(row["login"]), AvatarURL: stringValue(row["avatar_url"]), ReportPoints: int(number(row["report_points"])), BonusPoints: int(number(row["bonus_points"]))}
		m.TotalPoints = m.ReportPoints + m.BonusPoints
		result.TeamTotal += m.TotalPoints
		result.Members = append(result.Members, m)
	}
	return result, nil
}

func number(v interface{}) float64     { n, _ := v.(float64); return n }
func stringValue(v interface{}) string { s, _ := v.(string); return s }
