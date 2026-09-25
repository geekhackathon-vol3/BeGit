package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/crypto"
	"github.com/irj0927/begit/pkg/externalnotify"
	"github.com/irj0927/begit/pkg/notificationqueue"
)

var AllExternalNotificationEventTypes = []string{
	model.EventBeGitTime,
	model.EventChallengeEnd,
	model.EventSprintReminder,
	model.EventSprintEnd,
	model.EventSprintStart,
}

type CreateNotificationChannelInput struct {
	Platform    string
	DisplayName string
	WebhookURL  string
	EventTypes  []string
}

type UpdateNotificationChannelInput struct {
	DisplayName *string
	WebhookURL  *string
	Enabled     *bool
	EventTypes  *[]string
}

type NotificationChannelService interface {
	List(ctx context.Context, groupID, userID int64) ([]model.NotificationChannel, error)
	Create(ctx context.Context, groupID, userID int64, input CreateNotificationChannelInput) (*model.NotificationChannel, error)
	Update(ctx context.Context, groupID, channelID, userID int64, input UpdateNotificationChannelInput) (*model.NotificationChannel, error)
	Delete(ctx context.Context, groupID, channelID, userID int64) error
	Test(ctx context.Context, groupID, channelID, userID int64) error
}

type notificationChannelService struct {
	channels  repository.NotificationChannelRepository
	groups    repository.GroupRepository
	encryptor crypto.Encryptor
	sender    externalnotify.Client
}

func NewNotificationChannelService(channels repository.NotificationChannelRepository, groups repository.GroupRepository, encryptor crypto.Encryptor, sender externalnotify.Client) NotificationChannelService {
	return &notificationChannelService{channels: channels, groups: groups, encryptor: encryptor, sender: sender}
}

func (s *notificationChannelService) requireOwner(ctx context.Context, groupID, userID int64) (*model.Group, error) {
	group, err := s.groups.GetByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if group.OwnerUserID != userID {
		return nil, ErrForbidden
	}
	return group, nil
}

func (s *notificationChannelService) List(ctx context.Context, groupID, userID int64) ([]model.NotificationChannel, error) {
	if _, err := s.requireOwner(ctx, groupID, userID); err != nil {
		return nil, err
	}
	return s.channels.ListByGroup(ctx, groupID)
}

func (s *notificationChannelService) Create(ctx context.Context, groupID, userID int64, input CreateNotificationChannelInput) (*model.NotificationChannel, error) {
	if _, err := s.requireOwner(ctx, groupID, userID); err != nil {
		return nil, err
	}
	input.Platform = strings.ToLower(strings.TrimSpace(input.Platform))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.DisplayName == "" || len([]rune(input.DisplayName)) > 60 {
		return nil, fmt.Errorf("%w: display_name is required and must be at most 60 characters", ErrValidation)
	}
	if err := externalnotify.ValidateWebhookURL(input.Platform, input.WebhookURL); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	events, err := validateEventTypes(input.EventTypes)
	if err != nil {
		return nil, err
	}
	encrypted, err := s.encryptor.Encrypt(input.WebhookURL)
	if err != nil {
		return nil, fmt.Errorf("encrypt webhook: %w", err)
	}
	return s.channels.Create(ctx, &model.NotificationChannel{
		GroupID: groupID, Platform: input.Platform, DisplayName: input.DisplayName,
		EncryptedWebhookURL: encrypted, Enabled: true, CreatedBy: userID, EventTypes: events,
	})
}

func (s *notificationChannelService) Update(ctx context.Context, groupID, channelID, userID int64, input UpdateNotificationChannelInput) (*model.NotificationChannel, error) {
	if _, err := s.requireOwner(ctx, groupID, userID); err != nil {
		return nil, err
	}
	channel, err := s.channels.GetByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if channel.GroupID != groupID {
		return nil, ErrNotFound
	}
	if input.DisplayName != nil {
		value := strings.TrimSpace(*input.DisplayName)
		if value == "" || len([]rune(value)) > 60 {
			return nil, fmt.Errorf("%w: invalid display_name", ErrValidation)
		}
		channel.DisplayName = value
	}
	if input.WebhookURL != nil {
		if err := externalnotify.ValidateWebhookURL(channel.Platform, *input.WebhookURL); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrValidation, err)
		}
		encrypted, err := s.encryptor.Encrypt(*input.WebhookURL)
		if err != nil {
			return nil, err
		}
		channel.EncryptedWebhookURL = encrypted
	}
	if input.Enabled != nil {
		channel.Enabled = *input.Enabled
	}
	if input.EventTypes != nil {
		events, err := validateEventTypes(*input.EventTypes)
		if err != nil {
			return nil, err
		}
		channel.EventTypes = events
	}
	return s.channels.Update(ctx, channel)
}

func (s *notificationChannelService) Delete(ctx context.Context, groupID, channelID, userID int64) error {
	if _, err := s.requireOwner(ctx, groupID, userID); err != nil {
		return err
	}
	channel, err := s.channels.GetByID(ctx, channelID)
	if err != nil {
		return err
	}
	if channel.GroupID != groupID {
		return ErrNotFound
	}
	return s.channels.Delete(ctx, channelID)
}

func (s *notificationChannelService) Test(ctx context.Context, groupID, channelID, userID int64) error {
	group, err := s.requireOwner(ctx, groupID, userID)
	if err != nil {
		return err
	}
	channel, err := s.channels.GetByID(ctx, channelID)
	if err != nil {
		return err
	}
	if channel.GroupID != groupID {
		return ErrNotFound
	}
	webhookURL, err := s.encryptor.Decrypt(channel.EncryptedWebhookURL)
	if err != nil {
		return fmt.Errorf("decrypt webhook: %w", err)
	}
	return s.sender.Send(ctx, channel.Platform, webhookURL, model.NotificationEvent{
		Key: fmt.Sprintf("test:%d:%d", channel.ID, time.Now().UnixNano()), Type: "test", GroupID: group.ID,
		GroupName: group.Name, Title: "🌱 BeGitとつながりました！", Body: "これからチームの進捗を、ここへ可愛くお届けします。",
		AccentColor: "#39D353", Fields: map[string]string{"通知先": channel.DisplayName}, OccurredAt: time.Now().UTC(),
	})
}

func validateEventTypes(values []string) ([]string, error) {
	if len(values) == 0 {
		values = AllExternalNotificationEventTypes
	}
	allowed := make(map[string]bool, len(AllExternalNotificationEventTypes))
	for _, value := range AllExternalNotificationEventTypes {
		allowed[value] = true
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !allowed[value] {
			return nil, fmt.Errorf("%w: unsupported event_type %q", ErrValidation, value)
		}
		if !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	return result, nil
}

// ExternalNotificationPublisher はドメインイベントを購読中チャネルの永続ジョブへ変換する。
type ExternalNotificationPublisher interface {
	Publish(ctx context.Context, event model.NotificationEvent) error
}

type externalNotificationPublisher struct {
	channels repository.NotificationChannelRepository
	jobs     repository.NotificationDeliveryJobRepository
	queue    notificationqueue.Client
}

func NewExternalNotificationPublisher(channels repository.NotificationChannelRepository, jobs repository.NotificationDeliveryJobRepository, queue notificationqueue.Client) ExternalNotificationPublisher {
	return &externalNotificationPublisher{channels: channels, jobs: jobs, queue: queue}
}

func (p *externalNotificationPublisher) Publish(ctx context.Context, event model.NotificationEvent) error {
	channels, err := p.channels.ListEnabledForEvent(ctx, event.GroupID, event.Type)
	if err != nil || len(channels) == 0 {
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	var publishErrors []error
	for _, channel := range channels {
		jobID, created, err := p.jobs.Create(ctx, event.Key, channel.ID, event.Type, string(payload))
		if err != nil {
			publishErrors = append(publishErrors, err)
			continue
		}
		if !created {
			continue
		}
		if err := p.queue.Enqueue(ctx, jobID); err != nil {
			publishErrors = append(publishErrors, err)
		}
	}
	return errors.Join(publishErrors...)
}

type ExternalNotificationDeliveryService interface {
	Deliver(ctx context.Context, jobID int64) error
}

type externalNotificationDeliveryService struct {
	channels  repository.NotificationChannelRepository
	jobs      repository.NotificationDeliveryJobRepository
	encryptor crypto.Encryptor
	sender    externalnotify.Client
}

func NewExternalNotificationDeliveryService(channels repository.NotificationChannelRepository, jobs repository.NotificationDeliveryJobRepository, encryptor crypto.Encryptor, sender externalnotify.Client) ExternalNotificationDeliveryService {
	return &externalNotificationDeliveryService{channels: channels, jobs: jobs, encryptor: encryptor, sender: sender}
}

func (s *externalNotificationDeliveryService) Deliver(ctx context.Context, jobID int64) error {
	job, err := s.jobs.GetByID(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status == "delivered" || job.Status == "failed" {
		return nil
	}
	channel, err := s.channels.GetByID(ctx, job.ChannelID)
	if err != nil {
		return err
	}
	if !channel.Enabled {
		return s.jobs.MarkFailed(ctx, jobID, "channel is disabled")
	}
	webhookURL, err := s.encryptor.Decrypt(channel.EncryptedWebhookURL)
	if err != nil {
		return err
	}
	var event model.NotificationEvent
	if err := json.Unmarshal([]byte(job.PayloadJSON), &event); err != nil {
		_ = s.jobs.MarkFailed(ctx, jobID, "invalid event payload")
		return nil
	}
	err = s.sender.Send(ctx, channel.Platform, webhookURL, event)
	if err == nil {
		return s.jobs.MarkDelivered(ctx, jobID)
	}
	message := truncateDeliveryError(err.Error())
	_ = s.jobs.MarkAttempt(ctx, jobID, message)
	var deliveryErr *externalnotify.DeliveryError
	if errors.As(err, &deliveryErr) && !deliveryErr.Retryable {
		_ = s.jobs.MarkFailed(ctx, jobID, message)
		return nil
	}
	return err
}

func truncateDeliveryError(value string) string {
	const max = 500
	if len(value) <= max {
		return value
	}
	return value[:max]
}
