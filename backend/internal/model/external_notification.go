package model

import "time"

const (
	NotificationPlatformSlack   = "slack"
	NotificationPlatformDiscord = "discord"

	EventBeGitTime      = "begit_time"
	EventChallengeEnd   = "challenge_end"
	EventSprintReminder = "sprint_reminder"
	EventSprintEnd      = "sprint_end"
	EventSprintStart    = "sprint_start"
)

// NotificationChannel はグループに接続された外部通知先。
// EncryptedWebhookURL は API レスポンスへ露出させない。
type NotificationChannel struct {
	ID                  int64
	GroupID             int64
	Platform            string
	DisplayName         string
	EncryptedWebhookURL string
	Enabled             bool
	CreatedBy           int64
	EventTypes          []string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// NotificationEvent はプラットフォームに依存しない通知表現。
type NotificationEvent struct {
	Key         string            `json:"key"`
	Type        string            `json:"type"`
	GroupID     int64             `json:"group_id"`
	GroupName   string            `json:"group_name"`
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	AccentColor string            `json:"accent_color,omitempty"`
	ActionURL   string            `json:"action_url,omitempty"`
	Fields      map[string]string `json:"fields,omitempty"`
	OccurredAt  time.Time         `json:"occurred_at"`
}

// NotificationDeliveryJob は Queue へ投入される永続ジョブ。
type NotificationDeliveryJob struct {
	ID           int64
	EventKey     string
	ChannelID    int64
	EventType    string
	PayloadJSON  string
	Status       string
	AttemptCount int
	LastError    string
}
