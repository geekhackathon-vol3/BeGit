package service

import (
	"context"
	"errors"
	"testing"

	"github.com/irj0927/begit/internal/model"
)

func TestPointsServiceOnlyVerifiedRepositoryOwnerCanComplete(t *testing.T) {
	for _, tc := range []struct {
		name     string
		verified bool
		userID   int64
	}{
		{"different member", true, 2},
		{"unverified owner", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groups := &mockGroupRepository{getByIDFunc: func(context.Context, int64) (*model.Group, error) {
				return &model.Group{ID: 1, OwnerUserID: 1, OwnerVerified: tc.verified}, nil
			}}
			_, err := NewPointsService(groups, nil).Complete(context.Background(), 1, tc.userID)
			if !errors.Is(err, ErrForbidden) {
				t.Fatalf("got %v, want ErrForbidden", err)
			}
		})
	}
}

func TestManualTextCannotEarnReportPoints(t *testing.T) {
	notificationID := int64(3)
	body := "status update"
	_, err := NewPostService(nil, nil, nil, nil, nil, nil).CreatePost(context.Background(), CreatePostRequest{
		NotificationID: &notificationID, PostType: "memo", ContentSource: "manual", Body: &body,
	}, 1, 1)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("got %v, want ErrValidation", err)
	}
}
