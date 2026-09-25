// Package externalnotify は Slack / Discord Incoming Webhook への描画と送信を提供する。
package externalnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/irj0927/begit/internal/model"
)

const maxResponseBytes = 16 * 1024

type Client interface {
	Send(ctx context.Context, platform, webhookURL string, event model.NotificationEvent) error
}

type DeliveryError struct {
	StatusCode int
	Retryable  bool
	Message    string
}

func (e *DeliveryError) Error() string {
	if e.StatusCode == 0 {
		return "external notification failed: " + e.Message
	}
	return fmt.Sprintf("external notification failed (%d): %s", e.StatusCode, e.Message)
}

type HTTPClient struct{ client *http.Client }

func NewClient() Client {
	return &HTTPClient{client: &http.Client{
		Timeout: 10 * time.Second,
		// Webhook の資格情報を別ホストへ転送しない。
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func ValidateWebhookURL(platform, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" {
		return errors.New("webhook URL must be a valid HTTPS URL")
	}
	host := strings.ToLower(u.Hostname())
	switch platform {
	case model.NotificationPlatformSlack:
		if host != "hooks.slack.com" && host != "hooks.slack-gov.com" {
			return errors.New("Slack webhook must use hooks.slack.com")
		}
		if !strings.HasPrefix(u.EscapedPath(), "/services/") {
			return errors.New("invalid Slack webhook path")
		}
	case model.NotificationPlatformDiscord:
		if host != "discord.com" && host != "discordapp.com" {
			return errors.New("Discord webhook must use discord.com")
		}
		if !strings.HasPrefix(u.EscapedPath(), "/api/webhooks/") {
			return errors.New("invalid Discord webhook path")
		}
	default:
		return errors.New("unsupported notification platform")
	}
	return nil
}

func (c *HTTPClient) Send(ctx context.Context, platform, webhookURL string, event model.NotificationEvent) error {
	if err := ValidateWebhookURL(platform, webhookURL); err != nil {
		return &DeliveryError{Message: err.Error()}
	}
	body, err := Render(platform, event)
	if err != nil {
		return &DeliveryError{Message: err.Error()}
	}
	endpoint := webhookURL
	if platform == model.NotificationPlatformDiscord {
		u, _ := url.Parse(endpoint)
		q := u.Query()
		q.Set("wait", "true")
		u.RawQuery = q.Encode()
		endpoint = u.String()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return &DeliveryError{Message: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return &DeliveryError{Retryable: true, Message: err.Error()}
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	message := strings.TrimSpace(string(responseBody))
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	return &DeliveryError{
		StatusCode: resp.StatusCode,
		Retryable:  resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500,
		Message:    message,
	}
}

func Render(platform string, event model.NotificationEvent) ([]byte, error) {
	switch platform {
	case model.NotificationPlatformSlack:
		return renderSlack(event)
	case model.NotificationPlatformDiscord:
		return renderDiscord(event)
	default:
		return nil, errors.New("unsupported notification platform")
	}
}

func renderSlack(event model.NotificationEvent) ([]byte, error) {
	blocks := []map[string]interface{}{
		{"type": "header", "text": map[string]string{"type": "plain_text", "text": event.Title, "emoji": "true"}},
		{"type": "section", "text": map[string]string{"type": "mrkdwn", "text": event.Body}},
	}
	if len(event.Fields) > 0 {
		fields := make([]map[string]string, 0, len(event.Fields))
		for _, pair := range orderedFields(event.Fields) {
			fields = append(fields, map[string]string{"type": "mrkdwn", "text": "*" + pair[0] + "*\n" + pair[1]})
		}
		blocks = append(blocks, map[string]interface{}{"type": "section", "fields": fields})
	}
	if isHTTPURL(event.ActionURL) {
		blocks = append(blocks, map[string]interface{}{"type": "actions", "elements": []map[string]interface{}{
			{"type": "button", "text": map[string]string{"type": "plain_text", "text": "BeGitで見る", "emoji": "true"}, "style": "primary", "url": event.ActionURL},
		}})
	}
	blocks = append(blocks, map[string]interface{}{"type": "context", "elements": []map[string]string{
		{"type": "mrkdwn", "text": "🌱 *BeGit*  •  " + event.GroupName},
	}})
	return json.Marshal(map[string]interface{}{"text": event.Title + " — " + event.Body, "blocks": blocks})
}

func renderDiscord(event model.NotificationEvent) ([]byte, error) {
	fields := make([]map[string]interface{}, 0, len(event.Fields))
	for _, pair := range orderedFields(event.Fields) {
		fields = append(fields, map[string]interface{}{"name": pair[0], "value": pair[1], "inline": true})
	}
	embed := map[string]interface{}{
		"title":       event.Title,
		"description": event.Body,
		"color":       parseColor(event.AccentColor),
		"fields":      fields,
		"footer":      map[string]string{"text": "🌱 BeGit  •  " + event.GroupName},
		"timestamp":   event.OccurredAt.UTC().Format(time.RFC3339),
	}
	if isHTTPURL(event.ActionURL) {
		embed["url"] = event.ActionURL
	}
	return json.Marshal(map[string]interface{}{
		"username":         "BeGit",
		"content":          "",
		"embeds":           []map[string]interface{}{embed},
		"allowed_mentions": map[string]interface{}{"parse": []string{}},
	})
}

func orderedFields(fields map[string]string) [][2]string {
	preferred := []string{"発行者", "残り時間", "達成", "期間"}
	result := make([][2]string, 0, len(fields))
	seen := make(map[string]bool)
	for _, key := range preferred {
		if value, ok := fields[key]; ok {
			result = append(result, [2]string{key, value})
			seen[key] = true
		}
	}
	remaining := make([]string, 0, len(fields))
	for key := range fields {
		if !seen[key] {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)
	for _, key := range remaining {
		value := fields[key]
		if !seen[key] {
			result = append(result, [2]string{key, value})
		}
	}
	return result
}

func parseColor(value string) int64 {
	value = strings.TrimPrefix(value, "#")
	if parsed, err := strconv.ParseInt(value, 16, 64); err == nil && len(value) == 6 {
		return parsed
	}
	return 0x39D353
}

func isHTTPURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}
