package model

import (
	"testing"
	"time"
)

// TestModelFields は各モデルのフィールドが正しく定義されているかを確認する
func TestUserModel(t *testing.T) {
	u := User{
		ID:                   1,
		GitHubID:             12345,
		GitHubLogin:          "testuser",
		GitHubName:           "Test User",
		AvatarURL:            "https://example.com/avatar.png",
		EncryptedAccessToken: "encrypted_token",
		CreatedAt:            time.Now(),
	}

	if u.ID != 1 {
		t.Errorf("expected ID=1, got %d", u.ID)
	}
	if u.GitHubLogin != "testuser" {
		t.Errorf("expected GitHubLogin=testuser, got %s", u.GitHubLogin)
	}
}

func TestGroupModel(t *testing.T) {
	g := Group{
		ID:                 1,
		RepoFullName:       "owner/repo",
		Name:               "My Team",
		AvatarURL:          "https://example.com/avatar.png",
		OwnerUserID:        42,
		SprintDurationDays: 7,
		CreatedAt:          time.Now(),
	}

	if g.RepoFullName != "owner/repo" {
		t.Errorf("expected RepoFullName=owner/repo, got %s", g.RepoFullName)
	}
	if g.SprintDurationDays != 7 {
		t.Errorf("expected SprintDurationDays=7, got %d", g.SprintDurationDays)
	}
}

func TestGroupMemberModel(t *testing.T) {
	m := GroupMember{
		GroupID:    1,
		UserID:     2,
		Login:      "member1",
		AvatarURL:  "https://example.com/avatar.png",
		Role:       "member",
		AutoJoined: true,
	}

	if m.Role != "member" {
		t.Errorf("expected Role=member, got %s", m.Role)
	}
	if !m.AutoJoined {
		t.Error("expected AutoJoined=true")
	}
}

func TestSprintModel(t *testing.T) {
	now := time.Now()
	s := Sprint{
		ID:        1,
		GroupID:   2,
		IndexNum:  0,
		StartedAt: now,
		EndsAt:    now.AddDate(0, 0, 7),
	}

	if s.GroupID != 2 {
		t.Errorf("expected GroupID=2, got %d", s.GroupID)
	}
}

func TestNotificationModel(t *testing.T) {
	n := Notification{
		ID:       1,
		SprintID: 2,
		SentBy:   3,
		Message:  "今なに作ってる？",
		SentAt:   time.Now(),
	}

	if n.Message != "今なに作ってる？" {
		t.Errorf("expected default message, got %s", n.Message)
	}
}

func TestPostModel(t *testing.T) {
	body := "テスト投稿"
	repo := "owner/repo"
	msg := "Initial commit"

	p := Post{
		ID:                  1,
		NotificationID:      nil,
		UserID:              2,
		GroupID:             3,
		PostType:            "commit",
		Body:                &body,
		RepoFullName:        &repo,
		BranchName:          nil,
		CommitCount:         3,
		Additions:           100,
		Deletions:           50,
		LatestCommitMessage: &msg,
		Status:              nil,
		CreatedAt:           time.Now(),
	}

	if *p.Body != "テスト投稿" {
		t.Errorf("expected Body=テスト投稿, got %s", *p.Body)
	}
	if p.CommitCount != 3 {
		t.Errorf("expected CommitCount=3, got %d", p.CommitCount)
	}
}

func TestPostFeedModel(t *testing.T) {
	pf := PostFeed{
		Post: Post{
			ID:     1,
			UserID: 2,
		},
		Login:     "testuser",
		AvatarURL: "https://example.com/avatar.png",
		Blurred:   false,
	}

	if pf.Login != "testuser" {
		t.Errorf("expected Login=testuser, got %s", pf.Login)
	}
	if pf.Blurred {
		t.Error("expected Blurred=false")
	}
}
