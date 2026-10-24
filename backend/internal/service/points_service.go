package service

import (
	"context"
	"errors"

	"github.com/irj0927/begit/internal/repository"
)

type PointsService struct {
	groups repository.GroupRepository
	points *repository.PointsRepository
}

func NewPointsService(groups repository.GroupRepository, points *repository.PointsRepository) *PointsService {
	return &PointsService{groups: groups, points: points}
}

func (s *PointsService) Complete(ctx context.Context, groupID, userID int64) (*repository.FinalResult, error) {
	group, err := s.groups.GetByID(ctx, groupID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// Only the verified repository administrator can finalize a work.
	if !group.OwnerVerified || group.OwnerUserID != userID {
		return nil, ErrForbidden
	}
	if group.CompletedAt == nil {
		if err := s.points.FinalizeBonuses(ctx, groupID); err != nil {
			return nil, err
		}
		if err := s.points.Complete(ctx, groupID); err != nil {
			if errors.Is(err, repository.ErrConflict) {
				return nil, ErrConflict
			}
			return nil, err
		}
	}
	return s.points.Result(ctx, groupID)
}

func (s *PointsService) Result(ctx context.Context, groupID int64) (*repository.FinalResult, error) {
	result, err := s.points.Result(ctx, groupID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrNotFound
	}
	return result, err
}
