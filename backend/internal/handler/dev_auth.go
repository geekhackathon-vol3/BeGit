package handler

import (
	"encoding/json"
	"hash/fnv"
	"net/http"

	"github.com/irj0927/begit/internal/model"
	"github.com/irj0927/begit/internal/repository"
	"github.com/irj0927/begit/pkg/crypto"
)

// devSeedUsers は既知の dev シードユーザーの固定 github_id（負値で実 GitHub ID と衝突回避）。
var devSeedUsers = map[string]int64{
	"alice": -1001,
	"bob":   -1002,
	"carol": -1003,
}

// devAuthHandler は POST /auth/dev を処理する dev 専用ログインハンドラ。
// DEV_MODE=true のときだけルート登録される（buildHandler 側で gate）。
// GitHub OAuth を通さず、固定トークン dev_<login> を発行してユーザーを UPSERT する。
type devAuthHandler struct {
	userRepo  repository.UserRepository
	encryptor crypto.Encryptor
}

// NewDevAuthHandler は devAuthHandler を作成する。
func NewDevAuthHandler(userRepo repository.UserRepository, encryptor crypto.Encryptor) http.Handler {
	return &devAuthHandler{userRepo: userRepo, encryptor: encryptor}
}

// ServeHTTP は POST /auth/dev を処理する。
// body: { "login": "alice" }（省略時は "alice"）。
// レスポンスは POST /auth/github と同形（{user, token}）。
func (h *devAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Login string `json:"login"`
	}
	// body は任意。デコード失敗・空 body でも alice にフォールバックする。
	_ = json.NewDecoder(r.Body).Decode(&req)

	login := req.Login
	if login == "" {
		login = "alice"
	}

	githubID, ok := devSeedUsers[login]
	if !ok {
		githubID = deterministicDevID(login)
	}

	// 固定トークン dev_<login> を決定的に暗号化して保存する。
	// ミドルウェアが同じ平文を Encrypt して照合するため Bearer 認証が通る。
	token := "dev_" + login
	encryptedToken, err := h.encryptor.Encrypt(token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	user := &model.User{
		GitHubID:             githubID,
		GitHubLogin:          login,
		GitHubName:           login + " (dev)",
		AvatarURL:            "https://avatars.githubusercontent.com/u/0?v=4",
		EncryptedAccessToken: encryptedToken,
	}

	saved, err := h.userRepo.UpsertUser(r.Context(), user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := AuthResponse{
		User: UserJSON{
			ID:        saved.ID,
			Login:     saved.GitHubLogin,
			AvatarURL: saved.AvatarURL,
			Name:      saved.GitHubName,
		},
		Token: token,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// deterministicDevID は未知の login に対し決定的な負の github_id を生成する。
func deterministicDevID(login string) int64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(login))
	return -100000 - int64(h.Sum32()%1000000)
}
