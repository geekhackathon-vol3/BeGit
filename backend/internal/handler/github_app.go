package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/service"
)

// GitHubAppHandler はGitHub Appのインストール完了コールバックを処理する。
type GitHubAppHandler struct {
	installationService service.GitHubAppInstallationService
	// iosRedirectURI is optional so browser-only installations can keep receiving JSON.
	iosRedirectURI string
}

func NewGitHubAppHandler(installationService service.GitHubAppInstallationService, iosRedirectURI ...string) *GitHubAppHandler {
	redirectURI := ""
	if len(iosRedirectURI) > 0 {
		redirectURI = iosRedirectURI[0]
	}
	return &GitHubAppHandler{
		installationService: installationService,
		iosRedirectURI:      redirectURI,
	}
}

// Setup はGitHub AppのSetup URLからinstallation_idを受け取り、D1へ保存する。
// GitHubがブラウザをリダイレクトする公開エンドポイントのためBearer認証は要求しない。
//
// @Summary GitHub Appインストール完了
// @Description GitHub Appのインストール情報を検証して保存する
// @Tags github-app
// @Produce json
// @Param installation_id query int true "GitHub App installation ID"
// @Param setup_action query string true "install or update"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Router /github/app/setup [get]
func (h *GitHubAppHandler) Setup(c *gin.Context) {
	installationID, err := strconv.ParseInt(c.Query("installation_id"), 10, 64)
	if err != nil || installationID <= 0 {
		respondError(c, http.StatusBadRequest, "installation_id: invalid")
		return
	}

	setupAction := c.Query("setup_action")
	installation, err := h.installationService.CompleteSetup(c.Request.Context(), installationID, setupAction, nil)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrValidation):
			respondError(c, http.StatusBadRequest, "invalid setup action")
		case errors.Is(err, service.ErrForbidden):
			respondError(c, http.StatusForbidden, "GitHub App installation is not accessible")
		case errors.Is(err, service.ErrExternalAPI):
			respondError(c, http.StatusBadGateway, "failed to complete GitHub App installation")
		default:
			respondError(c, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response := gin.H{
		"status":          "installed",
		"installation_id": installation.InstallationID,
		"account_login":   installation.AccountLogin,
		"account_type":    installation.AccountType,
	}

	// iOSから開始したインストールは、完了後にカスタムURLスキームへ戻す。
	// 未設定時は従来どおりJSONを返し、ブラウザからの手動確認も壊さない。
	if redirectURL, ok := buildIOSRedirectURL(
		h.iosRedirectURI,
		installation.InstallationID,
		setupAction,
		installation.AccountLogin,
		installation.AccountType,
	); ok {
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	c.JSON(http.StatusOK, response)
}

func buildIOSRedirectURL(base string, installationID int64, setupAction, accountLogin, accountType string) (string, bool) {
	if base == "" || installationID <= 0 {
		return "", false
	}

	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "begit" || parsed.Host != "github-app-setup" {
		return "", false
	}

	query := parsed.Query()
	query.Set("installation_id", strconv.FormatInt(installationID, 10))
	query.Set("setup_action", setupAction)
	query.Set("account_login", accountLogin)
	query.Set("account_type", accountType)
	parsed.RawQuery = query.Encode()
	return parsed.String(), true
}
