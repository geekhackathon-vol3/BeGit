package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/service"
)

// mockNotificationService はテスト用の NotificationService モック
type mockNotificationService struct {
	sendFunc              func(ctx context.Context, groupID, userID int64) (*model.Notification, error)
	getActiveNotification func(ctx context.Context, groupID int64) (*service.ActiveNotification, error)
	getStatusFunc         func(ctx context.Context, notifID, groupID int64) (*service.NotificationStatus, error)
	getActiveFunc         func(ctx context.Context, groupID, userID int64) (*service.ActiveChallenge, error)
	endFunc               func(ctx context.Context, groupID, notifID, userID int64) (*model.Notification, error)
}

func (m *mockNotificationService) SendNotification(ctx context.Context, groupID, userID int64) (*model.Notification, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, groupID, userID)
	}
	return &model.Notification{ID: 1, SprintID: 1, SentBy: userID, SentAt: time.Now()}, nil
}

func (m *mockNotificationService) GetActiveNotification(ctx context.Context, groupID int64) (*service.ActiveNotification, error) {
	if m.getActiveNotification != nil {
		return m.getActiveNotification(ctx, groupID)
	}
	return nil, nil
}

func (m *mockNotificationService) GetNotificationStatus(ctx context.Context, notifID, groupID int64) (*service.NotificationStatus, error) {
	if m.getStatusFunc != nil {
		return m.getStatusFunc(ctx, notifID, groupID)
	}
	return &service.NotificationStatus{NotificationID: notifID}, nil
}

func (m *mockNotificationService) GetActiveChallenge(ctx context.Context, groupID, userID int64) (*service.ActiveChallenge, error) {
	if m.getActiveFunc != nil {
		return m.getActiveFunc(ctx, groupID, userID)
	}
	return nil, nil
}

func (m *mockNotificationService) EndChallenge(ctx context.Context, groupID, notifID, userID int64) (*model.Notification, error) {
	if m.endFunc != nil {
		return m.endFunc(ctx, groupID, notifID, userID)
	}
	return nil, service.ErrNotFound
}

func (m *mockNotificationService) StopNotification(ctx context.Context, groupID, notifID, userID int64) error {
	return nil
}

// newNotificationRouter は userID を注入した上で通知エンドポイントを登録する
func newNotificationRouter(svc service.NotificationService, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	inject := func(c *gin.Context) {
		if userID != 0 {
			c.Set(ctxUserID, userID)
		}
	}
	h := NewNotificationHandler(svc)
	r.POST("/groups/:id/notifications", inject, h.Send)
	r.POST("/groups/:id/notifications/:nid/end", inject, h.End)
	return r
}

// TestNotificationHandler_End_OK は発行者の中断で 200 と ended_at を返すことを確認する
func TestNotificationHandler_End_OK(t *testing.T) {
	endedAt := time.Date(2026, 9, 18, 1, 40, 0, 0, time.UTC)
	svc := &mockNotificationService{
		endFunc: func(ctx context.Context, groupID, notifID, userID int64) (*model.Notification, error) {
			if groupID != 12 || notifID != 8 || userID != 23 {
				t.Errorf("unexpected args: group=%d notif=%d user=%d", groupID, notifID, userID)
			}
			return &model.Notification{ID: 8, SprintID: 5, SentBy: 23, SentAt: endedAt.Add(-15 * time.Minute), EndedAt: &endedAt}, nil
		},
	}
	r := newNotificationRouter(svc, 23)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/groups/12/notifications/8/end", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body NotificationJSON
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.ID != 8 || body.SprintID != 5 || body.EndedAt == nil || *body.EndedAt != "2026-09-18T01:40:00Z" {
		t.Errorf("unexpected body: %+v", body)
	}
}

// TestNotificationHandler_End_ErrorMapping はサービスエラーが 403 / 404 / 409 / 500 にマップされることを確認する
func TestNotificationHandler_End_ErrorMapping(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"not issuer":  {service.ErrForbidden, http.StatusForbidden},
		"wrong group": {service.ErrNotFound, http.StatusNotFound},
		"not active":  {service.ErrConflict, http.StatusConflict},
		"internal":    {context.DeadlineExceeded, http.StatusInternalServerError},
	} {
		svc := &mockNotificationService{
			endFunc: func(ctx context.Context, groupID, notifID, userID int64) (*model.Notification, error) {
				return nil, tc.err
			},
		}
		r := newNotificationRouter(svc, 24)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/groups/12/notifications/8/end", nil))
		if w.Code != tc.want {
			t.Errorf("[%s] expected %d, got %d: %s", name, tc.want, w.Code, w.Body.String())
		}
	}
}

// TestNotificationHandler_End_BadRequest は未認証 / 不正な ID で 401 / 400 を返すことを確認する
func TestNotificationHandler_End_BadRequest(t *testing.T) {
	r := newNotificationRouter(&mockNotificationService{}, 0)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/groups/12/notifications/8/end", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without user, got %d", w.Code)
	}

	r = newNotificationRouter(&mockNotificationService{}, 23)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/groups/12/notifications/abc/end", nil))
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid notification id, got %d", w.Code)
	}
}

// TestNotificationHandler_Send_NoEndedAt は発行直後のレスポンスに ended_at が含まれないことを確認する
func TestNotificationHandler_Send_NoEndedAt(t *testing.T) {
	r := newNotificationRouter(&mockNotificationService{}, 23)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/groups/12/notifications", nil))
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	var raw map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &raw)
	if _, ok := raw["ended_at"]; ok {
		t.Errorf("ended_at must be omitted when not ended: %s", w.Body.String())
	}
}
