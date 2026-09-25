package externalnotify

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
)

func testEvent() model.NotificationEvent {
	return model.NotificationEvent{
		Key: "begit_time:1", Type: model.EventBeGitTime, GroupID: 1, GroupName: "BeGit",
		Title: "⏰ BeGit Time!", Body: "今、なに作ってる？", AccentColor: "#39D353",
		Fields:    map[string]string{"発行者": "@ayaka", "残り時間": "60分"},
		ActionURL: "https://example.com/groups/1", OccurredAt: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC),
	}
}

func TestValidateWebhookURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		platform, raw string
		wantErr       bool
	}{
		{model.NotificationPlatformSlack, "https://hooks.slack.com/services/T/B/X", false},
		{model.NotificationPlatformDiscord, "https://discord.com/api/webhooks/1/token", false},
		{model.NotificationPlatformSlack, "http://hooks.slack.com/services/T/B/X", true},
		{model.NotificationPlatformSlack, "https://evil.example/services/T/B/X", true},
		{model.NotificationPlatformDiscord, "https://discord.com/invite/example", true},
		{"teams", "https://example.com/webhook", true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.platform+tt.raw, func(t *testing.T) {
			t.Parallel()
			if got := ValidateWebhookURL(tt.platform, tt.raw); (got != nil) != tt.wantErr {
				t.Fatalf("ValidateWebhookURL() error = %v, wantErr %v", got, tt.wantErr)
			}
		})
	}
}

func TestRenderSlackUsesBlocksAndFallbackText(t *testing.T) {
	t.Parallel()
	body, err := Render(model.NotificationPlatformSlack, testEvent())
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload["text"].(string), "BeGit Time") {
		t.Fatalf("missing fallback text: %s", body)
	}
	blocks, ok := payload["blocks"].([]interface{})
	if !ok || len(blocks) < 4 {
		t.Fatalf("expected polished Block Kit payload: %s", body)
	}
}

func TestRenderDiscordDisablesMentions(t *testing.T) {
	t.Parallel()
	body, err := Render(model.NotificationPlatformDiscord, testEvent())
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	mentions := payload["allowed_mentions"].(map[string]interface{})
	if parse := mentions["parse"].([]interface{}); len(parse) != 0 {
		t.Fatalf("mentions must be disabled: %s", body)
	}
	embeds := payload["embeds"].([]interface{})
	if len(embeds) != 1 {
		t.Fatalf("expected one embed: %s", body)
	}
}
