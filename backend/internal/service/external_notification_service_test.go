package service

import (
	"context"
	"errors"
	"testing"

	"github.com/irj0927/begit/internal/model"
)

func TestNotificationChannelService_PosterCanManage(t *testing.T) {
	groups := &mockGroupRepository{
		getByIDFunc: func(context.Context, int64) (*model.Group, error) {
			return &model.Group{ID: 8}, nil
		},
		getMemberFunc: func(context.Context, int64, int64) (*model.GroupMember, error) {
			return &model.GroupMember{Role: model.GroupRolePoster}, nil
		},
	}
	service := &notificationChannelService{groups: groups}
	if _, err := service.requirePoster(context.Background(), 8, 42); err != nil {
		t.Fatalf("poster should be able to manage notification channels: %v", err)
	}

	groups.getMemberFunc = func(context.Context, int64, int64) (*model.GroupMember, error) {
		return &model.GroupMember{Role: model.GroupRoleViewer}, nil
	}
	if _, err := service.requirePoster(context.Background(), 8, 42); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer should be forbidden, got %v", err)
	}
}
