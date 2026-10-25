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
	ModeResults []FinalModeResult   `json:"mode_results"`
}

type FinalModeResult struct {
	GameMode    string `json:"game_mode"`
	Attempts    int    `json:"attempts"`
	Successes   int    `json:"successes"`
	Failures    int    `json:"failures"`
	Cancelled   int    `json:"cancelled"`
	Pending     int    `json:"pending"`
	BonusPoints int    `json:"bonus_points"`
}

// PointsSummary is the current project scoreboard. CompletedAt is nil while
// the project is active; after completion its member values come from the
// immutable final-result snapshot.
type PointsSummary struct {
	GroupID     int64               `json:"group_id"`
	IsFinal     bool                `json:"is_final"`
	CompletedAt *string             `json:"completed_at"`
	Members     []FinalMemberPoints `json:"members"`
	TeamTotal   int                 `json:"team_total"`
	ModeResults []FinalModeResult   `json:"mode_results"`
}

type PointsRepository struct{ db d1.Client }

func NewPointsRepository(db d1.Client) *PointsRepository { return &PointsRepository{db: db} }

// FinalizeBonuses persists one immutable game result per due notification.
// The notification_game_result_award trigger atomically creates the unique
// bonus award for successful results, so retries cannot duplicate points.
func (r *PointsRepository) FinalizeBonuses(ctx context.Context, groupID int64) error {
	_, err := r.db.Exec(ctx, `WITH candidates AS (
 SELECT n.id AS notification_id, n.game_mode, n.sent_by,
   CASE
     WHEN n.stopped_at IS NOT NULL THEN 0
     WHEN n.game_mode = 'everyone' THEN
       CASE WHEN EXISTS (
         SELECT 1 FROM notification_members nm WHERE nm.notification_id = n.id
       ) AND NOT EXISTS (
         SELECT 1 FROM notification_members nm
         WHERE nm.notification_id = n.id AND NOT EXISTS (
           SELECT 1 FROM posts p
           WHERE p.notification_id = n.id AND p.user_id = nm.user_id
             AND p.is_draft = 0 AND p.status = 'on_time'
             AND datetime(p.created_at) <= datetime(COALESCE(n.ended_at, datetime(n.sent_at, '+1 hour')))
         )
       ) THEN 1 ELSE 0 END
     WHEN n.game_mode = 'spotlight' THEN
       CASE WHEN EXISTS (
         SELECT 1 FROM posts p
         WHERE p.notification_id = n.id AND p.user_id = n.sent_by
           AND p.is_draft = 0 AND p.status = 'on_time'
           AND datetime(p.created_at) <= datetime(COALESCE(n.ended_at, datetime(n.sent_at, '+1 hour')))
       ) AND NOT EXISTS (
         SELECT 1 FROM notification_members nm
         JOIN posts p ON p.notification_id = nm.notification_id AND p.user_id = nm.user_id
         WHERE nm.notification_id = n.id AND nm.user_id != n.sent_by
           AND p.is_draft = 0 AND p.status = 'on_time'
           AND datetime(p.created_at) <= datetime(COALESCE(n.ended_at, datetime(n.sent_at, '+1 hour')))
       ) THEN 1 ELSE 0 END
     ELSE 0
   END AS succeeded,
   CASE WHEN n.stopped_at IS NOT NULL THEN 1 ELSE 0 END AS cancelled
 FROM notifications n
 JOIN sprints s ON s.id = n.sprint_id
 JOIN groups g ON g.id = s.group_id
 WHERE (? = 0 OR s.group_id = ?) AND g.completed_at IS NULL
   AND (n.stopped_at IS NOT NULL OR datetime(COALESCE(n.ended_at, datetime(n.sent_at, '+1 hour'))) <= datetime('now'))
)
INSERT OR IGNORE INTO notification_game_results(
 notification_id, game_mode, outcome, bonus_user_id, bonus_points
)
SELECT notification_id, game_mode,
 CASE WHEN cancelled = 1 THEN 'cancelled' WHEN succeeded = 1 THEN 'succeeded' ELSE 'failed' END,
 CASE WHEN succeeded = 1 AND cancelled = 0 THEN sent_by ELSE NULL END,
 CASE WHEN succeeded = 1 AND cancelled = 0 THEN 10 ELSE 0 END
FROM candidates`, []interface{}{groupID, groupID})
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
	result := &FinalResult{GroupID: groupID, CompletedAt: completedAt, Members: []FinalMemberPoints{}, ModeResults: []FinalModeResult{}}
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
	modeRows, err := r.db.Query(ctx, `SELECT ngr.game_mode,
 SUM(CASE WHEN ngr.outcome != 'cancelled' THEN 1 ELSE 0 END) AS attempts,
 SUM(CASE WHEN ngr.outcome = 'succeeded' THEN 1 ELSE 0 END) AS successes,
 SUM(CASE WHEN ngr.outcome = 'failed' THEN 1 ELSE 0 END) AS failures,
 SUM(CASE WHEN ngr.outcome = 'cancelled' THEN 1 ELSE 0 END) AS cancelled,
 SUM(ngr.bonus_points) AS bonus_points
 FROM notification_game_results ngr
 JOIN notifications n ON n.id = ngr.notification_id
 JOIN sprints s ON s.id = n.sprint_id
	 WHERE s.group_id = ?
 GROUP BY ngr.game_mode ORDER BY ngr.game_mode`, []interface{}{groupID})
	if err != nil && !errors.Is(err, d1.ErrNotFound) {
		return nil, err
	}
	for _, row := range modeRows {
		result.ModeResults = append(result.ModeResults, FinalModeResult{
			GameMode: textValue(row["game_mode"]), BonusPoints: int(number(row["bonus_points"])),
			Attempts: int(number(row["attempts"])), Successes: int(number(row["successes"])),
			Failures: int(number(row["failures"])), Cancelled: int(number(row["cancelled"])),
		})
	}
	return result, nil
}

// Summary returns both the in-progress scoreboard and the immutable completed
// result through one contract. The current member list is unioned with past
// point recipients so a member removal never hides points already earned.
func (r *PointsRepository) Summary(ctx context.Context, groupID int64) (*PointsSummary, error) {
	rows, err := r.db.Query(ctx, `SELECT completed_at FROM groups WHERE id = ?`, []interface{}{groupID})
	if errors.Is(err, d1.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if completedAt, ok := rows[0]["completed_at"].(string); ok && completedAt != "" {
		final, err := r.Result(ctx, groupID)
		if err != nil {
			return nil, err
		}
		return &PointsSummary{
			GroupID: groupID, IsFinal: true, CompletedAt: &completedAt,
			Members: final.Members, TeamTotal: final.TeamTotal, ModeResults: final.ModeResults,
		}, nil
	}

	summary := &PointsSummary{
		GroupID: groupID, Members: []FinalMemberPoints{}, ModeResults: []FinalModeResult{},
	}
	members, err := r.db.Query(ctx, `SELECT participants.user_id, u.github_login AS login,
 COALESCE(u.avatar_url, '') AS avatar_url,
 COALESCE(SUM(CASE WHEN pa.kind = 'report' THEN pa.points ELSE 0 END), 0) AS report_points,
 COALESCE(SUM(CASE WHEN pa.kind = 'all_on_time_bonus' THEN pa.points ELSE 0 END), 0) AS bonus_points
 FROM (
   SELECT user_id FROM group_members WHERE group_id = ?
   UNION
   SELECT user_id FROM point_awards WHERE group_id = ?
 ) participants
 JOIN users u ON u.id = participants.user_id
 LEFT JOIN point_awards pa ON pa.group_id = ? AND pa.user_id = participants.user_id
 GROUP BY participants.user_id, u.github_login, u.avatar_url
 ORDER BY (report_points + bonus_points) DESC, login COLLATE NOCASE`, []interface{}{groupID, groupID, groupID})
	if err != nil && !errors.Is(err, d1.ErrNotFound) {
		return nil, err
	}
	for _, row := range members {
		member := FinalMemberPoints{
			UserID: int64(number(row["user_id"])), Login: stringValue(row["login"]),
			AvatarURL: stringValue(row["avatar_url"]), ReportPoints: int(number(row["report_points"])),
			BonusPoints: int(number(row["bonus_points"])),
		}
		member.TotalPoints = member.ReportPoints + member.BonusPoints
		summary.TeamTotal += member.TotalPoints
		summary.Members = append(summary.Members, member)
	}

	modeRows, err := r.db.Query(ctx, `SELECT n.game_mode,
 SUM(CASE WHEN ngr.outcome != 'cancelled' THEN 1 ELSE 0 END) AS attempts,
 SUM(CASE WHEN ngr.outcome = 'succeeded' THEN 1 ELSE 0 END) AS successes,
 SUM(CASE WHEN ngr.outcome = 'failed' THEN 1 ELSE 0 END) AS failures,
 SUM(CASE WHEN ngr.outcome = 'cancelled' THEN 1 ELSE 0 END) AS cancelled,
 SUM(CASE WHEN ngr.notification_id IS NULL THEN 1 ELSE 0 END) AS pending,
 COALESCE(SUM(ngr.bonus_points), 0) AS bonus_points
 FROM notifications n
 JOIN sprints s ON s.id = n.sprint_id
 LEFT JOIN notification_game_results ngr ON ngr.notification_id = n.id
 WHERE s.group_id = ?
 GROUP BY n.game_mode ORDER BY n.game_mode`, []interface{}{groupID})
	if err != nil && !errors.Is(err, d1.ErrNotFound) {
		return nil, err
	}
	for _, row := range modeRows {
		summary.ModeResults = append(summary.ModeResults, FinalModeResult{
			GameMode: textValue(row["game_mode"]), Attempts: int(number(row["attempts"])),
			Successes: int(number(row["successes"])), Failures: int(number(row["failures"])),
			Cancelled: int(number(row["cancelled"])), Pending: int(number(row["pending"])),
			BonusPoints: int(number(row["bonus_points"])),
		})
	}
	return summary, nil
}

func number(v interface{}) float64     { n, _ := v.(float64); return n }
func stringValue(v interface{}) string { s, _ := v.(string); return s }
