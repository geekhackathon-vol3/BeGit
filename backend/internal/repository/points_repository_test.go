package repository

import (
	"context"
	"strings"
	"testing"
)

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
