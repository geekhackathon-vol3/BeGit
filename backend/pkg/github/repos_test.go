package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestListUserRepos は権限に関わらずアクセス可能なリポジトリをすべて返すことを確認する
func TestListUserRepos(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"full_name":   "alice/repo-push",
				"name":        "repo-push",
				"private":     false,
				"owner":       map[string]interface{}{"login": "alice", "avatar_url": "a.png"},
				"permissions": map[string]interface{}{"admin": false, "push": true},
			},
			{
				"full_name":   "alice/repo-readonly",
				"name":        "repo-readonly",
				"private":     true,
				"owner":       map[string]interface{}{"login": "alice", "avatar_url": "a.png"},
				"permissions": map[string]interface{}{"admin": false, "push": false},
			},
		})
	}))
	defer server.Close()

	client := &githubClient{
		httpClient:    server.Client(),
		oauthEndpoint: server.URL,
		apiEndpoint:   server.URL,
	}

	repos, err := client.ListUserRepos(context.Background(), "token")
	if err != nil {
		t.Fatalf("ListUserRepos() failed: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos (all accessible), got %d", len(repos))
	}
	if repos[0].FullName != "alice/repo-push" || !repos[0].CanPush {
		t.Errorf("unexpected repo[0]: %+v", repos[0])
	}
	if repos[1].FullName != "alice/repo-readonly" || repos[1].CanPush {
		t.Errorf("unexpected repo[1]: %+v", repos[1])
	}
}

// TestListCommits は一覧 + 各コミットの差分統計を返すことを確認する
func TestListCommits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// /repos/{owner}/{repo}/commits/{sha} は詳細、それ以外は一覧
		if r.URL.Path == "/repos/alice/repo/commits" {
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"sha": "sha1",
					"commit": map[string]interface{}{
						"message": "first",
						"author":  map[string]interface{}{"name": "Alice", "date": "2026-06-01T10:00:00Z"},
					},
					"author": map[string]interface{}{"login": "alice"},
				},
			})
			return
		}
		// 詳細（stats 付き）
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stats": map[string]interface{}{"additions": 42, "deletions": 7},
		})
	}))
	defer server.Close()

	client := &githubClient{
		httpClient:    server.Client(),
		oauthEndpoint: server.URL,
		apiEndpoint:   server.URL,
	}

	commits, err := client.ListCommits(context.Background(), "alice/repo", "token", CommitListOptions{PerPage: 5})
	if err != nil {
		t.Fatalf("ListCommits() failed: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	c := commits[0]
	if c.SHA != "sha1" || c.Message != "first" || c.AuthorLogin != "alice" || c.Additions != 42 || c.Deletions != 7 {
		t.Errorf("unexpected commit: %+v", c)
	}
}
