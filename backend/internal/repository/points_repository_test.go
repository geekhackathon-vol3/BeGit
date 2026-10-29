package repository

import (
	"context"
	"strings"
	"testing"
)

func TestPointsSummaryReturnsCurrentMemberAndModeTotals(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(_ context.Context, sql string, _ []interface{}) ([]map[string]interface{}, error) {
			switch {
			case strings.Contains(sql, "SELECT completed_at FROM groups"):
				return []map[string]interface{}{{"completed_at": nil}}, nil
			case strings.Contains(sql, "participants.user_id"):
				return []map[string]interface{}{
					{"user_id": float64(1), "login": "あやか", "avatar_url": "avatar", "report_points": float64(20), "bonus_points": float64(10)},
					{"user_id": float64(2), "login": "takumi", "avatar_url": "", "report_points": float64(10), "bonus_points": float64(0)},
				}, nil
			case strings.Contains(sql, "LEFT JOIN notification_game_results"):
				return []map[string]interface{}{{
					"game_mode": "everyone", "attempts": float64(2), "successes": float64(1),
					"failures": float64(1), "cancelled": float64(1), "pending": float64(1), "bonus_points": float64(10),
				}}, nil
			default:
				t.Fatalf("unexpected query: %s", sql)
				return nil, nil
			}
		},
	}

	result, err := NewPointsRepository(mock).Summary(context.Background(), 7)
	if err != nil {
		t.Fatalf("Summary() failed: %v", err)
	}
	if result.IsFinal || result.CompletedAt != nil || result.TeamTotal != 40 {
		t.Fatalf("unexpected summary header: %+v", result)
	}
	if len(result.Members) != 2 || result.Members[0].TotalPoints != 30 {
		t.Fatalf("unexpected members: %+v", result.Members)
	}
	if len(result.ModeResults) != 1 || result.ModeResults[0].Pending != 1 {
		t.Fatalf("unexpected mode results: %+v", result.ModeResults)
	}
}

func TestPointsSummaryUsesImmutableResultAfterCompletion(t *testing.T) {
	queryCount := 0
	mock := &mockD1Client{
		queryFunc: func(_ context.Context, sql string, _ []interface{}) ([]map[string]interface{}, error) {
			queryCount++
			switch {
			case strings.Contains(sql, "SELECT completed_at FROM groups"):
				return []map[string]interface{}{{"completed_at": "2026-10-26 00:50:00"}}, nil
			case strings.Contains(sql, "FROM final_result_members"):
				return []map[string]interface{}{{
					"user_id": float64(1), "login": "ayaka", "avatar_url": "", "report_points": float64(30), "bonus_points": float64(10),
				}}, nil
			case strings.Contains(sql, "FROM notification_game_results"):
				return []map[string]interface{}{}, nil
			default:
				t.Fatalf("unexpected query: %s", sql)
				return nil, nil
			}
		},
	}

	result, err := NewPointsRepository(mock).Summary(context.Background(), 7)
	if err != nil {
		t.Fatalf("Summary() failed: %v", err)
	}
	if !result.IsFinal || result.CompletedAt == nil || result.TeamTotal != 40 {
		t.Fatalf("unexpected final summary: %+v", result)
	}
	if queryCount < 3 {
		t.Fatalf("expected final snapshot queries, got %d", queryCount)
	}
}

// TestPointsRepository_Result_FormatsCompletedAtAsRFC3339 は completed_at を GET /groups/:id と同じ RFC3339 で返すことを確認する
func TestPointsRepository_Result_FormatsCompletedAtAsRFC3339(t *testing.T) {
	mock := &mockD1Client{
		queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
			if strings.Contains(sql, "FROM groups") {
				return []map[string]interface{}{{"completed_at": "2026-10-24 12:00:00"}}, nil
			}
			return []map[string]interface{}{}, nil
		},
	}

	result, err := NewPointsRepository(mock).Result(context.Background(), 1)
	if err != nil {
		t.Fatalf("Result() failed: %v", err)
}
if result.CompletedAt != "2026-10-24T12:00:00Z" {
	t.Errorf("expected RFC3339 completed_at, got %q", result.CompletedAt)
}
}
