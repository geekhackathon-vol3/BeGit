package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
)

const (
	defaultBeGitTimeLocalTime = "09:00"
	defaultBeGitTimeTimezone  = "Asia/Tokyo"
)

var defaultBeGitTimeWeekdays = []int{1, 2, 3, 4, 5}

type UpdateBeGitTimeScheduleInput struct {
	Enabled   bool
	LocalTime string
	Timezone  string
	Weekdays  []int
}

type BeGitTimeScheduleService interface {
	Get(ctx context.Context, groupID, userID int64) (*model.BeGitTimeSchedule, error)
	Update(ctx context.Context, groupID, userID int64, input UpdateBeGitTimeScheduleInput) (*model.BeGitTimeSchedule, error)
}

type beGitTimeScheduleService struct {
	schedules repository.BeGitTimeScheduleRepository
	groups    repository.GroupRepository
}

func NewBeGitTimeScheduleService(schedules repository.BeGitTimeScheduleRepository, groups repository.GroupRepository) BeGitTimeScheduleService {
	return &beGitTimeScheduleService{schedules: schedules, groups: groups}
}

func (s *beGitTimeScheduleService) requireOwner(ctx context.Context, groupID, userID int64) error {
	group, err := s.groups.GetByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if !group.OwnerVerified {
		return ErrForbidden
	}
	member, err := s.groups.GetMember(ctx, groupID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrForbidden
		}
		return err
	}
	if model.NormalizeGroupRole(member.Role) != model.GroupRoleOwner {
		return ErrForbidden
	}
	return nil
}

func (s *beGitTimeScheduleService) Get(ctx context.Context, groupID, userID int64) (*model.BeGitTimeSchedule, error) {
	if err := s.requireOwner(ctx, groupID, userID); err != nil {
		return nil, err
	}
	schedule, err := s.schedules.GetByGroupID(ctx, groupID)
	if errors.Is(err, repository.ErrNotFound) {
		return &model.BeGitTimeSchedule{
			GroupID: groupID, LocalTime: defaultBeGitTimeLocalTime, Timezone: defaultBeGitTimeTimezone,
			Weekdays: append([]int(nil), defaultBeGitTimeWeekdays...),
		}, nil
	}
	return schedule, err
}

func (s *beGitTimeScheduleService) Update(ctx context.Context, groupID, userID int64, input UpdateBeGitTimeScheduleInput) (*model.BeGitTimeSchedule, error) {
	if err := s.requireOwner(ctx, groupID, userID); err != nil {
		return nil, err
	}
	localTime := strings.TrimSpace(input.LocalTime)
	timezone := strings.TrimSpace(input.Timezone)
	weekdays, err := validateBeGitTimeSchedule(localTime, timezone, input.Weekdays)
	if err != nil {
		return nil, err
	}

	schedule := &model.BeGitTimeSchedule{
		GroupID: groupID, Enabled: input.Enabled, LocalTime: localTime,
		Timezone: timezone, Weekdays: weekdays, UpdatedBy: userID,
	}
	if input.Enabled {
		next, err := NextBeGitTimeRun(time.Now().UTC(), localTime, timezone, weekdays)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrValidation, err)
		}
		schedule.NextRunAt = &next
	}
	return s.schedules.Upsert(ctx, schedule)
}

func validateBeGitTimeSchedule(localTime, timezone string, weekdays []int) ([]int, error) {
	if _, err := time.Parse("15:04", localTime); err != nil {
		return nil, fmt.Errorf("%w: local_time must use HH:mm", ErrValidation)
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, fmt.Errorf("%w: unknown timezone", ErrValidation)
	}
	if len(weekdays) == 0 {
		return nil, fmt.Errorf("%w: at least one weekday is required", ErrValidation)
	}
	unique := make(map[int]struct{}, len(weekdays))
	for _, weekday := range weekdays {
		if weekday < 1 || weekday > 7 {
			return nil, fmt.Errorf("%w: weekdays must be between 1 and 7", ErrValidation)
		}
		unique[weekday] = struct{}{}
	}
	normalized := make([]int, 0, len(unique))
	for weekday := range unique {
		normalized = append(normalized, weekday)
	}
	sort.Ints(normalized)
	return normalized, nil
}

// NextBeGitTimeRun returns the first configured wall-clock occurrence strictly after after.
// The returned value is UTC, while weekday/time evaluation is done in the configured IANA timezone.
func NextBeGitTimeRun(after time.Time, localTime, timezone string, weekdays []int) (time.Time, error) {
	parsedClock, err := time.Parse("15:04", localTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid local time")
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone")
	}
	allowed := make(map[int]bool, len(weekdays))
	for _, weekday := range weekdays {
		if weekday < 1 || weekday > 7 {
			return time.Time{}, fmt.Errorf("invalid weekday")
		}
		allowed[weekday] = true
	}
	if len(allowed) == 0 {
		return time.Time{}, fmt.Errorf("no weekdays configured")
	}

	localAfter := after.In(location)
	for dayOffset := 0; dayOffset <= 7; dayOffset++ {
		day := localAfter.AddDate(0, 0, dayOffset)
		isoWeekday := int(day.Weekday())
		if isoWeekday == 0 {
			isoWeekday = 7
		}
		if !allowed[isoWeekday] {
			continue
		}
		candidate := time.Date(day.Year(), day.Month(), day.Day(), parsedClock.Hour(), parsedClock.Minute(), 0, 0, location)
		if candidate.After(localAfter) {
			return candidate.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("could not calculate next run")
}
