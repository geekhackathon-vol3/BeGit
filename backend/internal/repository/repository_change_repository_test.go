package repository

import (
	"context"
	"strings"
	"testing"
)

func TestRepositoryChangeRepositoryListAfter(t *testing.T) {
	queries := 0
	mock := &mockD1Client{queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
		queries++
		if strings.Contains(sql, "MIN(sequence)") {
			return []map[string]interface{}{{"min_sequence": float64(10), "max_sequence": float64(15)}}, nil
		}
		return []map[string]interface{}{
			{"sequence": float64(13), "group_id": float64(7), "entity_type": "reaction", "entity_id": float64(9), "post_id": float64(42), "operation": "upsert", "changed_at": "2026-10-26 00:00:00"},
		}, nil
	}}
	repo := NewRepositoryChangeRepository(mock)
	changes, cursor, reset, err := repo.ListAfter(context.Background(), 7, 12, 100)
	if err != nil {
		t.Fatalf("ListAfter failed: %v", err)
	}
	if reset || cursor != 13 || len(changes) != 1 || changes[0].PostID == nil || *changes[0].PostID != 42 {
		t.Fatalf("unexpected result cursor=%d reset=%v changes=%+v", cursor, reset, changes)
	}
	if queries != 2 {
		t.Fatalf("expected bounds and list queries, got %d", queries)
	}
}

func TestRepositoryChangeRepositoryRequiresResetForExpiredCursor(t *testing.T) {
	mock := &mockD1Client{queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
		return []map[string]interface{}{{"min_sequence": float64(100), "max_sequence": float64(120)}}, nil
	}}
	repo := NewRepositoryChangeRepository(mock)
	changes, cursor, reset, err := repo.ListAfter(context.Background(), 7, 10, 100)
	if err != nil {
		t.Fatalf("ListAfter failed: %v", err)
	}
	if !reset || cursor != 120 || len(changes) != 0 {
		t.Fatalf("expected reset cursor=120, got cursor=%d reset=%v changes=%+v", cursor, reset, changes)
	}
}

func TestRepositoryChangeRepositoryAllowsLookaheadAtMaximumPageSize(t *testing.T) {
	mock := &mockD1Client{queryFunc: func(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
		if strings.Contains(sql, "MIN(sequence)") {
			return []map[string]interface{}{{"min_sequence": float64(1), "max_sequence": float64(201)}}, nil
		}
		if got := params[2]; got != 201 {
			t.Fatalf("expected lookahead limit 201, got %v", got)
		}
		return []map[string]interface{}{}, nil
	}}
	repo := NewRepositoryChangeRepository(mock)
	if _, _, _, err := repo.ListAfter(context.Background(), 7, 0, 201); err != nil {
		t.Fatalf("ListAfter failed: %v", err)
	}
}
