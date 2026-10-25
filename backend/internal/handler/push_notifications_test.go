package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/service"
)

// mockPushNotificationService はテスト用の PushNotificationService モック。呼ばれた type と引数を記録する
type mockPushNotificationService struct {
	calls []string
	err   error
}

func (m *mockPushNotificationService) SendBeGitTime(ctx context.Context, groupID, notificationID, sprintID int64) error {
	m.calls = append(m.calls, fmt.Sprintf("begit_time group=%d notification=%d sprint=%d", groupID, notificationID, sprintID))
	return m.err
}

func (m *mockPushNotificationService) SendReaction(ctx context.Context, groupID, postID, actorUserID, authorUserID int64) error {
	m.calls = append(m.calls, fmt.Sprintf("reaction group=%d post=%d actor=%d author=%d", groupID, postID, actorUserID, authorUserID))
	return m.err
}

func postPush(t *testing.T, svc service.PushNotificationService, secret, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/internal/push-notifications", NewPushNotificationHandler(svc, "topsecret").Send)
	req := httptest.NewRequest(http.MethodPost, "/internal/push-notifications", strings.NewReader(body))
	req.Header.Set("X-Notification-Queue-Secret", secret)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

const reactionBody = `{"type":"reaction","group_id":12,"post_id":890,"actor_user_id":2,"author_user_id":99}`

// TestPush_Dispatch は type ごとにサービスへ振り分け、メッセージの ID をそのまま渡して 204 を返すことを確認する
func TestPush_Dispatch(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{reactionBody, "reaction group=12 post=890 actor=2 author=99"},
		{`{"type":"begit_time","group_id":12,"notification_id":345,"sprint_id":7}`, "begit_time group=12 notification=345 sprint=7"},
	} {
		svc := &mockPushNotificationService{}
		w := postPush(t, svc, "topsecret", tc.body)
		if w.Code != http.StatusNoContent {
			t.Errorf("%s: expected 204, got %d", tc.want, w.Code)
		}
		if len(svc.calls) != 1 || svc.calls[0] != tc.want {
			t.Errorf("%s: unexpected calls %v", tc.want, svc.calls)
		}
	}
}

// TestPush_Rejects は secret の不一致・不正なメッセージを、サービスを呼ばずに 4xx（Queue は再送しない）にすることを確認する
func TestPush_Rejects(t *testing.T) {
	cases := []struct {
		name, secret, body string
		want               int
	}{
		{"wrong secret", "wrong", reactionBody, http.StatusForbidden},
		{"no secret", "", reactionBody, http.StatusForbidden},
		{"broken json", "topsecret", `{"type":`, http.StatusBadRequest},
		{"unknown type", "topsecret", `{"type":"comment","group_id":12}`, http.StatusBadRequest},
		{"job_id message", "topsecret", `{"job_id":1}`, http.StatusBadRequest},
		{"reaction missing author", "topsecret", `{"type":"reaction","group_id":12,"post_id":890,"actor_user_id":2}`, http.StatusBadRequest},
		{"begit_time zero sprint", "topsecret", `{"type":"begit_time","group_id":12,"notification_id":345,"sprint_id":0}`, http.StatusBadRequest},
		{"string id", "topsecret", `{"type":"reaction","group_id":"12","post_id":890,"actor_user_id":2,"author_user_id":99}`, http.StatusBadRequest},
		{"too large", "topsecret", `{"type":"reaction","pad":"` + strings.Repeat("x", pushMessageMaxBytes) + `"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockPushNotificationService{}
			w := postPush(t, svc, tc.secret, tc.body)
			if w.Code != tc.want {
				t.Errorf("expected %d, got %d", tc.want, w.Code)
			}
			if len(svc.calls) != 0 {
				t.Errorf("service should not be called, got %v", svc.calls)
			}
		})
	}
}

// TestPush_ServiceErrors は送り先がない・届かないなら 4xx（捨てる）、それ以外の失敗は 503（Queue が再送する）にすることを確認する
func TestPush_ServiceErrors(t *testing.T) {
	if w := postPush(t, &mockPushNotificationService{err: service.ErrNotFound}, "topsecret", reactionBody); w.Code != http.StatusNotFound {
		t.Errorf("ErrNotFound: expected 404, got %d", w.Code)
	}
	undeliverable := fmt.Errorf("%w: fcm 404", service.ErrPushUndeliverable)
	if w := postPush(t, &mockPushNotificationService{err: undeliverable}, "topsecret", reactionBody); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("ErrPushUndeliverable: expected 422, got %d", w.Code)
	}
	if w := postPush(t, &mockPushNotificationService{err: errors.New("fcm down")}, "topsecret", reactionBody); w.Code != http.StatusServiceUnavailable {
		t.Errorf("transient: expected 503, got %d", w.Code)
	}
}
