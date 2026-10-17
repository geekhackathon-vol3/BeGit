package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
)

func TestBeGitTimeScheduleService_PosterCanManage(t *testing.T) {
	groups := &mockGroupRepository{
		getByIDFunc: func(context.Context, int64) (*model.Group, error) {
			return &model.Group{ID: 8}, nil
		},
		getMemberFunc: func(context.Context, int64, int64) (*model.GroupMember, error) {
			return &model.GroupMember{Role: model.GroupRolePoster}, nil
		},
	}
	service := &beGitTimeScheduleService{groups: groups}
	if err := service.requirePoster(context.Background(), 8, 42); err != nil {
		t.Fatalf("poster should be able to manage the schedule: %v", err)
	}

	groups.getMemberFunc = func(context.Context, int64, int64) (*model.GroupMember, error) {
		return &model.GroupMember{Role: model.GroupRoleViewer}, nil
	}
	if err := service.requirePoster(context.Background(), 8, 42); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer should be forbidden, got %v", err)
	}
}

func TestNextBeGitTimeRunUsesConfiguredTimezone(t *testing.T) {
	after := time.Date(2026, 10, 16, 23, 59, 0, 0, time.UTC) // Saturday 08:59 in Tokyo
	got, err := NextBeGitTimeRun(after, "09:00", "Asia/Tokyo", []int{6})
	if err != nil {
		t.Fatalf("NextBeGitTimeRun() failed: %v", err)
	}
	want := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestNextBeGitTimeRunSkipsPastOccurrence(t *testing.T) {
	after := time.Date(2026, 10, 19, 0, 1, 0, 0, time.UTC) // Monday 09:01 in Tokyo
	got, err := NextBeGitTimeRun(after, "09:00", "Asia/Tokyo", []int{1, 3})
	if err != nil {
		t.Fatalf("NextBeGitTimeRun() failed: %v", err)
	}
	want := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}
