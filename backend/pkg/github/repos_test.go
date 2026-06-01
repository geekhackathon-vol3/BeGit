package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestListUserRepos は push/admin 権限のあるリポジトリのみ返すことを確認する
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
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo (push/admin only), got %d", len(repos))
	}
	if repos[0].FullName != "alice/repo-push" || !repos[0].CanPush || repos[0].OwnerLogin != "alice" {
		t.Errorf("unexpected repo: %+v", repos[0])
	}
}
