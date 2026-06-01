package github

import (
	"context"
	"testing"
)

// stubClient が Client インターフェースを満たすことをコンパイル時にチェックする。
var _ Client = (*stubClient)(nil)

// TestStubClient_GetRecentCommits は固定のコミットサマリーを返すことを確認する。
func TestStubClient_GetRecentCommits(t *testing.T) {
	c := NewStubClient()
	summary, err := c.GetRecentCommits(context.Background(), "owner/repo", "alice", "any-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.RepoFullName != "owner/repo" {
		t.Errorf("expected repo full name to echo input, got %q", summary.RepoFullName)
	}
	if summary.CommitCount != 3 {
		t.Errorf("expected commit count 3, got %d", summary.CommitCount)
	}
}

// TestStubClient_GetCollaborators は dev シードユーザーを返すことを確認する。
func TestStubClient_GetCollaborators(t *testing.T) {
	c := NewStubClient()
	collabs, err := c.GetCollaborators(context.Background(), "owner/repo", "any-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(collabs) != 2 {
		t.Fatalf("expected 2 collaborators, got %d", len(collabs))
	}
	if collabs[0].Login != "alice" || collabs[1].Login != "bob" {
		t.Errorf("expected alice and bob, got %q and %q", collabs[0].Login, collabs[1].Login)
	}
}

// TestStubClient_RegisterWebhook はエラーを返さないことを確認する。
func TestStubClient_RegisterWebhook(t *testing.T) {
	c := NewStubClient()
	if err := c.RegisterWebhook(context.Background(), "owner/repo", "tok", "https://example.com/webhook", "secret"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
