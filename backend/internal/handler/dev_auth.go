package handler

import (
	"hash/fnv"
	"net/http"

	"github.com/gin-gonic/gin"

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

// DevLoginRequest は POST /auth/dev のリクエストボディ（省略可）
type DevLoginRequest struct {
	Login string `json:"login" example:"alice"`
}

// DevAuthHandler は dev 専用ログイン（DEV_MODE=true のときだけ登録）。
// GitHub OAuth を通さず、固定トークン dev_<login> を発行してユーザーを UPSERT する。
type DevAuthHandler struct {
	userRepo  repository.UserRepository
	encryptor crypto.Encryptor
}

// NewDevAuthHandler は DevAuthHandler を作成する。
func NewDevAuthHandler(userRepo repository.UserRepository, encryptor crypto.Encryptor) *DevAuthHandler {
	return &DevAuthHandler{userRepo: userRepo, encryptor: encryptor}
}

// DevLogin は dev 専用ログインを処理する。
//
//	@Summary		dev 専用ログイン（DEV_MODE 限定）
//	@Description	GitHub OAuth を通さず固定トークン dev_<login> を発行する。DEV_MODE=true のときのみ有効。body 省略時は alice。
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		DevLoginRequest	false	"ログイン名（省略時 alice）"
//	@Success		200		{object}	AuthResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/auth/dev [post]
func (h *DevAuthHandler) DevLogin(c *gin.Context) {
	var req DevLoginRequest
	// body は任意。デコード失敗・空 body でも alice にフォールバックする。
	_ = c.ShouldBindJSON(&req)

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
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	user := &model.User{
		GitHubID:             githubID,
		GitHubLogin:          login,
		GitHubName:           login + " (dev)",
		AvatarURL:            "https://avatars.githubusercontent.com/u/0?v=4",
		EncryptedAccessToken: encryptedToken,
	}

	saved, err := h.userRepo.UpsertUser(c.Request.Context(), user)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		User: UserJSON{
			ID:        saved.ID,
			Login:     saved.GitHubLogin,
			AvatarURL: saved.AvatarURL,
			Name:      saved.GitHubName,
		},
		Token: token,
	})
}

// deterministicDevID は未知の login に対し決定的な負の github_id を生成する。
func deterministicDevID(login string) int64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(login))
	return -100000 - int64(h.Sum32()%1000000)
}
