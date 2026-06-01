package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/pkg/crypto"
)

// fakeUserRepo は dev_auth テスト用の UserRepository モック。
type fakeUserRepo struct {
	upserted *model.User
}

func (r *fakeUserRepo) GetByEncryptedToken(ctx context.Context, encryptedToken string) (*model.User, error) {
	return nil, nil
}

func (r *fakeUserRepo) UpsertUser(ctx context.Context, user *model.User) (*model.User, error) {
	r.upserted = user
	saved := *user
	saved.ID = 42 // DB 採番を模す
	return &saved, nil
}

func (r *fakeUserRepo) GetByGitHubLogin(ctx context.Context, login string) (*model.User, error) {
	return nil, nil
}

const testEncKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

// TestDevAuthHandler_DefaultLogin は body 省略時に alice として token=dev_alice を返すことを確認する。
func TestDevAuthHandler_DefaultLogin(t *testing.T) {
	enc, err := crypto.NewEncryptor(testEncKey)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}
	repo := &fakeUserRepo{}
	h := NewDevAuthHandler(repo, enc)

	req := httptest.NewRequest(http.MethodPost, "/auth/dev", bytes.NewReader([]byte(`{}`)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. body: %s", rr.Code, rr.Body.String())
	}

	var resp AuthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.Token != "dev_alice" {
		t.Errorf("expected token=dev_alice, got %q", resp.Token)
	}
	if resp.User.Login != "alice" {
		t.Errorf("expected login=alice, got %q", resp.User.Login)
	}
	if resp.User.ID != 42 {
		t.Errorf("expected saved user ID=42, got %d", resp.User.ID)
	}
}

// TestDevAuthHandler_StoresMatchingEncryptedToken は保存される encrypted_access_token が
// ミドルウェアが Bearer トークンを Encrypt した値と一致することを確認する（バイパス成立の核心）。
func TestDevAuthHandler_StoresMatchingEncryptedToken(t *testing.T) {
	enc, err := crypto.NewEncryptor(testEncKey)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}
	repo := &fakeUserRepo{}
	h := NewDevAuthHandler(repo, enc)

	body, _ := json.Marshal(map[string]string{"login": "bob"})
	req := httptest.NewRequest(http.MethodPost, "/auth/dev", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if repo.upserted == nil {
		t.Fatal("expected UpsertUser to be called")
	}

	// ミドルウェアは "dev_bob" を Encrypt して DB 照合する。決定的暗号化なので一致するはず。
	want, err := enc.Encrypt("dev_bob")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if repo.upserted.EncryptedAccessToken != want {
		t.Errorf("stored encrypted token does not match middleware lookup value")
	}
	if repo.upserted.GitHubID != -1002 {
		t.Errorf("expected bob github_id=-1002, got %d", repo.upserted.GitHubID)
	}
}

// TestDevAuthHandler_MethodNotAllowed は GET で 405 を返すことを確認する。
func TestDevAuthHandler_MethodNotAllowed(t *testing.T) {
	enc, _ := crypto.NewEncryptor(testEncKey)
	h := NewDevAuthHandler(&fakeUserRepo{}, enc)

	req := httptest.NewRequest(http.MethodGet, "/auth/dev", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}
