package service

import (
	"context"
	"testing"
	"time"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
)

// mockUserRepoForNiceWork は nicework_service 用の最小 UserRepository モック
type mockUserRepoForNiceWork struct {
	getByLoginFunc func(ctx context.Context, login string) (*model.User, error)
}

func (m *mockUserRepoForNiceWork) GetByGitHubLogin(ctx context.Context, login string) (*model.User, error) {
	if m.getByLoginFunc != nil {
		return m.getByLoginFunc(ctx, login)
	}
	return nil, repository.ErrNotFound
}

// niceWorkDeps はテスト用の依存をまとめる
func niceWorkDeps() (*mockUserRepoForNiceWork, *mockGroupRepository, *mockSprintRepository, *mockNotificationRepository, *mockPostRepository, *mockFCMTokenRepository, *fakeFCMClient) {
	return &mockUserRepoForNiceWork{},
		&mockGroupRepository{},
		&mockSprintRepository{},
		&mockNotificationRepository{},
		&mockPostRepository{},
		&mockFCMTokenRepository{},
		&fakeFCMClient{}
}

func newNiceWorkSvc(u userByLoginRepo, g repository.GroupRepository, sp repository.SprintRepository, n repository.NotificationRepository, p repository.PostRepository, ft repository.FCMTokenRepository, fc *fakeFCMClient) NiceWorkService {
	return NewNiceWorkService(u, g, sp, n, p, ft, fc)
}

// TestNiceWork_NotMember_NoOp は送信者が非メンバーなら no-op（送信なし）であることを確認する
func TestNiceWork_NotMember_NoOp(t *testing.T) {
	u, g, sp, n, p, ft, fc := niceWorkDeps()
	u.getByLoginFunc = func(ctx context.Context, login string) (*model.User, error) {
		return &model.User{ID: 10, GitHubLogin: login}, nil
	}
	g.isMemberFunc = func(ctx context.Context, groupID, userID int64) (bool, error) {
		return false, nil
	}

	svc := newNiceWorkSvc(u, g, sp, n, p, ft, fc)
	err := svc.HandleActivity(context.Background(), 12, "octocat", "commit", ActivityData{})
	if err != nil {
		t.Fatalf("HandleActivity() should be no-op, got: %v", err)
	}
	if len(fc.withDataCalls) != 0 {
		t.Error("expected no FCM send for non-member")
	}
}

// TestNiceWork_NoAnchor_NoOp は anchor 無しなら no-op であることを確認する
func TestNiceWork_NoAnchor_NoOp(t *testing.T) {
	u, g, sp, n, p, ft, fc := niceWorkDeps()
	u.getByLoginFunc = func(ctx context.Context, login string) (*model.User, error) {
		return &model.User{ID: 10}, nil
	}
	g.isMemberFunc = func(ctx context.Context, groupID, userID int64) (bool, error) { return true, nil }
	sp.getCurrentFunc = func(ctx context.Context, groupID int64) (*model.Sprint, error) {
		return &model.Sprint{ID: 7, GroupID: groupID}, nil
	}
	n.getLatestInSprintBeforeFunc = func(ctx context.Context, sprintID int64, before time.Time) (*model.Notification, error) {
		return nil, repository.ErrNotFound
	}

	svc := newNiceWorkSvc(u, g, sp, n, p, ft, fc)
	if err := svc.HandleActivity(context.Background(), 12, "octocat", "commit", ActivityData{}); err != nil {
		t.Fatalf("HandleActivity() should be no-op, got: %v", err)
	}
	if len(fc.withDataCalls) != 0 {
		t.Error("expected no FCM send without anchor")
	}
}

// TestNiceWork_OnTime_SendsToAuthorOnly は anchor 内検知で on_time を確定し本人のみへ nice_work を送ることを確認する
func TestNiceWork_OnTime_SendsToAuthorOnly(t *testing.T) {
	u, g, sp, n, p, ft, fc := niceWorkDeps()
	u.getByLoginFunc = func(ctx context.Context, login string) (*model.User, error) {
		return &model.User{ID: 10}, nil
	}
	g.isMemberFunc = func(ctx context.Context, groupID, userID int64) (bool, error) { return true, nil }
	sp.getCurrentFunc = func(ctx context.Context, groupID int64) (*model.Sprint, error) {
		return &model.Sprint{ID: 7, GroupID: groupID}, nil
	}
	// anchor の sent_at は今から30分前 → 検知(now)は +1h 以内 → on_time
	n.getLatestInSprintBeforeFunc = func(ctx context.Context, sprintID int64, before time.Time) (*model.Notification, error) {
		return &model.Notification{ID: 345, SprintID: 7, SentAt: time.Now().Add(-30 * time.Minute)}, nil
	}
	var draftStatus string
	p.createDraftFunc = func(ctx context.Context, post *model.Post) (*model.Post, error) {
		if post.Status != nil {
			draftStatus = *post.Status
		}
		post.ID = 890
		return post, nil
	}
	ft.getTokensByUserIDFunc = func(ctx context.Context, userID int64) ([]string, error) {
		return []string{"author-token"}, nil
	}
	ft.getTokensByGroupIDFunc = func(ctx context.Context, groupID int64) ([]string, error) {
		return []string{"a", "b", "c"}, nil
	}

	svc := newNiceWorkSvc(u, g, sp, n, p, ft, fc)
	if err := svc.HandleActivity(context.Background(), 12, "octocat", "commit", ActivityData{CommitCount: 3}); err != nil {
		t.Fatalf("HandleActivity() failed: %v", err)
	}
	if draftStatus != "on_time" {
		t.Errorf("expected draft status on_time, got %q", draftStatus)
	}
	if len(fc.withDataCalls) != 1 {
		t.Fatalf("expected 1 FCM send (author only), got %d", len(fc.withDataCalls))
	}
	call := fc.withDataCalls[0]
	if len(call.tokens) != 1 || call.tokens[0] != "author-token" {
		t.Errorf("expected send to author tokens only, got %v", call.tokens)
	}
	if call.data["type"] != "nice_work" || call.data["status"] != "on_time" || call.data["draft_post_id"] != "890" || call.data["notification_id"] != "345" {
		t.Errorf("unexpected nice_work data: %v", call.data)
	}
}

// TestNiceWork_Late は anchor から1h超の検知で late を確定することを確認する
func TestNiceWork_Late(t *testing.T) {
	u, g, sp, n, p, ft, fc := niceWorkDeps()
	u.getByLoginFunc = func(ctx context.Context, login string) (*model.User, error) { return &model.User{ID: 10}, nil }
	g.isMemberFunc = func(ctx context.Context, groupID, userID int64) (bool, error) { return true, nil }
	sp.getCurrentFunc = func(ctx context.Context, groupID int64) (*model.Sprint, error) {
		return &model.Sprint{ID: 7, GroupID: groupID}, nil
	}
	n.getLatestInSprintBeforeFunc = func(ctx context.Context, sprintID int64, before time.Time) (*model.Notification, error) {
		return &model.Notification{ID: 345, SprintID: 7, SentAt: time.Now().Add(-90 * time.Minute)}, nil
	}
	var draftStatus string
	p.createDraftFunc = func(ctx context.Context, post *model.Post) (*model.Post, error) {
		if post.Status != nil {
			draftStatus = *post.Status
		}
		post.ID = 891
		return post, nil
	}
	ft.getTokensByUserIDFunc = func(ctx context.Context, userID int64) ([]string, error) { return []string{"t"}, nil }

	svc := newNiceWorkSvc(u, g, sp, n, p, ft, fc)
	if err := svc.HandleActivity(context.Background(), 12, "octocat", "commit", ActivityData{}); err != nil {
		t.Fatalf("HandleActivity() failed: %v", err)
	}
	if draftStatus != "late" {
		t.Errorf("expected draft status late, got %q", draftStatus)
	}
	if fc.withDataCalls[0].data["status"] != "late" {
		t.Errorf("expected nice_work status late, got %v", fc.withDataCalls[0].data)
	}
}

// TestNiceWork_Idempotent_Skip は既 draft（UNIQUE 違反）で再発火せず skip することを確認する
func TestNiceWork_Idempotent_Skip(t *testing.T) {
	u, g, sp, n, p, ft, fc := niceWorkDeps()
	u.getByLoginFunc = func(ctx context.Context, login string) (*model.User, error) { return &model.User{ID: 10}, nil }
	g.isMemberFunc = func(ctx context.Context, groupID, userID int64) (bool, error) { return true, nil }
	sp.getCurrentFunc = func(ctx context.Context, groupID int64) (*model.Sprint, error) {
		return &model.Sprint{ID: 7, GroupID: groupID}, nil
	}
	n.getLatestInSprintBeforeFunc = func(ctx context.Context, sprintID int64, before time.Time) (*model.Notification, error) {
		return &model.Notification{ID: 345, SprintID: 7, SentAt: time.Now().Add(-10 * time.Minute)}, nil
	}
	p.createDraftFunc = func(ctx context.Context, post *model.Post) (*model.Post, error) {
		return nil, repository.ErrConstraintViolation // 既発火
	}

	svc := newNiceWorkSvc(u, g, sp, n, p, ft, fc)
	if err := svc.HandleActivity(context.Background(), 12, "octocat", "commit", ActivityData{}); err != nil {
		t.Fatalf("HandleActivity() should skip idempotently, got: %v", err)
	}
	if len(fc.withDataCalls) != 0 {
		t.Error("expected no FCM send when already fired (idempotent skip)")
	}
}

// TestNiceWork_FCMFailure_DoesNotFail は FCM 失敗でも ② 処理（draft 作成）が成功することを確認する
func TestNiceWork_FCMFailure_DoesNotFail(t *testing.T) {
	u, g, sp, n, p, ft, _ := niceWorkDeps()
	u.getByLoginFunc = func(ctx context.Context, login string) (*model.User, error) {
		return &model.User{ID: 10}, nil
	}
	g.isMemberFunc = func(ctx context.Context, groupID, userID int64) (bool, error) { return true, nil }
	sp.getCurrentFunc = func(ctx context.Context, groupID int64) (*model.Sprint, error) {
		return &model.Sprint{ID: 7, GroupID: groupID}, nil
	}
	n.getLatestInSprintBeforeFunc = func(ctx context.Context, sprintID int64, before time.Time) (*model.Notification, error) {
		return &model.Notification{ID: 345, SprintID: 7, SentAt: time.Now().Add(-30 * time.Minute)}, nil
	}
	p.createDraftFunc = func(ctx context.Context, post *model.Post) (*model.Post, error) {
		post.ID = 890
		return post, nil
	}
	ft.getTokensByUserIDFunc = func(ctx context.Context, userID int64) ([]string, error) {
		return []string{"author-token"}, nil
	}
	// failingFCMClient（interface）を直接渡すため実コンストラクタを使う（newNiceWorkSvc は *fakeFCMClient 固定）
	svc := NewNiceWorkService(u, g, sp, n, p, ft, &failingFCMClient{})
	if err := svc.HandleActivity(context.Background(), 12, "octocat", "commit", ActivityData{CommitCount: 3}); err != nil {
		t.Fatalf("HandleActivity() should succeed even if FCM fails, got: %v", err)
	}
}
