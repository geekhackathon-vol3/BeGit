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

// mockGroupService はテスト用の GroupService モック（GetGroup のみ差し替え可能）
type mockGroupService struct {
	getGroupFunc func(ctx context.Context, groupID, userID int64) (*service.GroupDetail, error)
}

func (m *mockGroupService) ListGroups(ctx context.Context, userID int64) ([]service.GroupDetail, error) {
	return nil, nil
}

func (m *mockGroupService) CreateGroup(ctx context.Context, req service.CreateGroupRequest, userID int64) (*model.Group, error) {
	return nil, service.ErrValidation
}

func (m *mockGroupService) LeaveGroup(ctx context.Context, groupID, userID int64) error {
	return nil
}

func (m *mockGroupService) GetGroup(ctx context.Context, groupID, userID int64) (*service.GroupDetail, error) {
	if m.getGroupFunc != nil {
		return m.getGroupFunc(ctx, groupID, userID)
	}
	return &service.GroupDetail{
		Group: model.Group{ID: groupID, Name: "BeGit", RepoFullName: "hackathon/BeGit"},
		Members: []model.GroupMember{
			{GroupID: groupID, UserID: 23, Login: "riochin", AvatarURL: "https://github.com/riochin.png", Role: "owner"},
			{GroupID: groupID, UserID: 24, Login: "tochi", AvatarURL: "https://github.com/tochi.png", Role: "member"},
		},
	}, nil
}

func (m *mockGroupService) SyncMembers(ctx context.Context, groupID int64, accessToken string) ([]model.GroupMember, error) {
	return nil, nil
}

// newGroupRouter は userID を注入した上で GET /groups/:id を登録する
func newGroupRouter(groupSvc service.GroupService, notifSvc service.NotificationService, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/groups/:id", func(c *gin.Context) {
		if userID != 0 {
			c.Set(ctxUserID, userID)
		}
		NewGroupHandler(groupSvc, notifSvc).Get(c)
	})
	return r
}

func getGroupDetail(t *testing.T, r *gin.Engine) (GroupDetailJSON, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/groups/12", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var detail GroupDetailJSON
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	var raw map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &raw)
	return detail, raw
}

// TestGroupHandler_Get_NoActiveChallenge は進行中の通知が無ければ active_challenge が省略されることを確認する
func TestGroupHandler_Get_NoActiveChallenge(t *testing.T) {
	r := newGroupRouter(&mockGroupService{}, &mockNotificationService{}, 23)
	detail, raw := getGroupDetail(t, r)
	if detail.ID != 12 || len(detail.Members) != 2 {
		t.Errorf("unexpected detail: %+v", detail)
	}
	if _, ok := raw["active_challenge"]; ok {
		t.Errorf("active_challenge must be omitted when none: %v", raw)
	}
}

// TestGroupHandler_Get_ActiveChallenge_Issuer は発行者から見ると can_end=true、発行者情報がメンバー一覧から解決されることを確認する
func TestGroupHandler_Get_ActiveChallenge_Issuer(t *testing.T) {
	sentAt := time.Date(2026, 9, 18, 1, 25, 46, 0, time.UTC)
	notifSvc := &mockNotificationService{
		getActiveFunc: func(ctx context.Context, groupID, userID int64) (*service.ActiveChallenge, error) {
			if groupID != 12 || userID != 23 {
				t.Errorf("unexpected args: group=%d user=%d", groupID, userID)
			}
			return &service.ActiveChallenge{
				Notification: model.Notification{ID: 8, SprintID: 5, SentBy: 23, SentAt: sentAt},
				EndsAt:       sentAt.Add(time.Hour),
			}, nil
		},
	}
	r := newGroupRouter(&mockGroupService{}, notifSvc, 23)
	detail, raw := getGroupDetail(t, r)

	ac := detail.ActiveChallenge
	if ac == nil {
		t.Fatalf("expected active_challenge: %v", raw)
	}
	if ac.NotificationID != 8 || ac.SprintID != 5 || !ac.CanEnd {
		t.Errorf("unexpected active_challenge: %+v", ac)
	}
	if ac.SentBy.UserID != 23 || ac.SentBy.Login != "riochin" || ac.SentBy.AvatarURL != "https://github.com/riochin.png" {
		t.Errorf("issuer should be resolved from members: %+v", ac.SentBy)
	}
	if ac.SentAt != "2026-09-18T01:25:46Z" || ac.EndsAt != "2026-09-18T02:25:46Z" {
		t.Errorf("unexpected times: sent_at=%s ends_at=%s", ac.SentAt, ac.EndsAt)
	}
	if ac.MyPost != nil {
		t.Errorf("my_post must be nil when the user has no post: %+v", ac.MyPost)
	}
}

// TestGroupHandler_Get_ActiveChallenge_MemberWithDraft は発行者以外から見ると can_end=false で、自分の draft が my_post に入ることを確認する
func TestGroupHandler_Get_ActiveChallenge_MemberWithDraft(t *testing.T) {
	status := "on_time"
	notifSvc := &mockNotificationService{
		getActiveFunc: func(ctx context.Context, groupID, userID int64) (*service.ActiveChallenge, error) {
			return &service.ActiveChallenge{
				Notification: model.Notification{ID: 8, SprintID: 5, SentBy: 23, SentAt: time.Now()},
				EndsAt:       time.Now().Add(time.Hour),
				MyPost:       &model.Post{ID: 41, UserID: userID, IsDraft: true, Status: &status},
			}, nil
		},
	}
	r := newGroupRouter(&mockGroupService{}, notifSvc, 24)
	detail, _ := getGroupDetail(t, r)

	ac := detail.ActiveChallenge
	if ac == nil || ac.CanEnd {
		t.Fatalf("expected active_challenge with can_end=false: %+v", ac)
	}
	if ac.MyPost == nil || ac.MyPost.PostID != 41 || !ac.MyPost.IsDraft || ac.MyPost.Status == nil || *ac.MyPost.Status != "on_time" {
		t.Errorf("unexpected my_post: %+v", ac.MyPost)
	}
}

// TestGroupHandler_Get_ActiveChallenge_IssuerLeft は発行者がメンバー一覧に居なくても login 空で返すことを確認する
func TestGroupHandler_Get_ActiveChallenge_IssuerLeft(t *testing.T) {
	notifSvc := &mockNotificationService{
		getActiveFunc: func(ctx context.Context, groupID, userID int64) (*service.ActiveChallenge, error) {
			return &service.ActiveChallenge{
				Notification: model.Notification{ID: 8, SprintID: 5, SentBy: 99, SentAt: time.Now()},
				EndsAt:       time.Now().Add(time.Hour),
			}, nil
		},
	}
	r := newGroupRouter(&mockGroupService{}, notifSvc, 24)
	detail, _ := getGroupDetail(t, r)
	if detail.ActiveChallenge == nil || detail.ActiveChallenge.SentBy.UserID != 99 || detail.ActiveChallenge.SentBy.Login != "" {
		t.Errorf("unexpected issuer: %+v", detail.ActiveChallenge)
	}
}

// TestGroupHandler_Get_ActiveChallenge_FailSoft は active_challenge の取得失敗時もグループ詳細を 200 で返すことを確認する
func TestGroupHandler_Get_ActiveChallenge_FailSoft(t *testing.T) {
	notifSvc := &mockNotificationService{
		getActiveFunc: func(ctx context.Context, groupID, userID int64) (*service.ActiveChallenge, error) {
			return nil, context.DeadlineExceeded
		},
	}
	r := newGroupRouter(&mockGroupService{}, notifSvc, 23)
	detail, raw := getGroupDetail(t, r)
	if detail.ID != 12 {
		t.Errorf("unexpected detail: %+v", detail)
	}
	if _, ok := raw["active_challenge"]; ok {
		t.Errorf("active_challenge must be omitted on failure: %v", raw)
	}
}
