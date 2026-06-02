package handler

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/irj0927/begit/internal/service"
)

// maxUploadBytes はリクエスト全体（main+front+フォームオーバーヘッド）の上限
const maxUploadBytes = 2*service.MaxPhotoBytes + 1*1024*1024

// PhotoJSON はフィード/アップロードレスポンスの写真型
type PhotoJSON struct {
	ID        int64  `json:"id"`
	PhotoType string `json:"photo_type"`
	URL       string `json:"url"`
}

// UploadPhotosResponse は写真アップロードのレスポンス
type UploadPhotosResponse struct {
	Photos []PhotoJSON `json:"photos"`
}

// PhotoHandler は写真アップロードエンドポイントのハンドラ
type PhotoHandler struct {
	photoService service.PhotoService
	r2Client     photoURLSigner
}

// photoURLSigner はアップロード直後のレスポンスに presigned URL を付与するための最小インターフェース。
// pkg/r2.Client がこれを満たす。
type photoURLSigner interface {
	PresignGetURL(key string, ttl time.Duration) (string, error)
}

// NewPhotoHandler は PhotoHandler を作成する
func NewPhotoHandler(photoService service.PhotoService, r2Client photoURLSigner) *PhotoHandler {
	return &PhotoHandler{photoService: photoService, r2Client: r2Client}
}

// Upload は投稿に写真（main/front）を紐付けてアップロードする。
//
//	@Summary		写真アップロード
//	@Description	投稿に main（背面・必須）/ front（前面・任意）の写真を multipart で紐付け、R2 に保存する
//	@Tags			photos
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int		true	"グループ ID"
//	@Param			postId	path		int		true	"投稿 ID"
//	@Param			main	formData	file	true	"背面写真（image/jpeg, image/png, image/heic）"
//	@Param			front	formData	file	false	"前面写真（任意）"
//	@Success		201		{object}	UploadPhotosResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		413		{object}	ErrorResponse
//	@Failure		422		{object}	ErrorResponse
//	@Failure		502		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/groups/{id}/posts/{postId}/photos [post]
func (h *PhotoHandler) Upload(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid group id")
		return
	}
	postID, err := strconv.ParseInt(c.Param("postId"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid post id")
		return
	}

	// リクエストサイズを制限してから multipart をパースする
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)
	if err := c.Request.ParseMultipartForm(maxUploadBytes); err != nil {
		respondError(c, http.StatusRequestEntityTooLarge, "request too large or invalid multipart form")
		return
	}

	main, err := readFormFile(c, "main")
	if err != nil {
		respondError(c, http.StatusBadRequest, "main photo is required")
		return
	}
	if main == nil {
		respondError(c, http.StatusBadRequest, "main photo is required")
		return
	}
	front, err := readFormFile(c, "front")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid front photo")
		return
	}

	req := service.UploadPhotosRequest{Main: main, Front: front}
	photos, err := h.photoService.UploadPhotos(c.Request.Context(), req, groupID, postID, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrValidation):
			respondError(c, http.StatusUnprocessableEntity, "invalid photo")
		case errors.Is(err, service.ErrForbidden):
			respondError(c, http.StatusForbidden, "forbidden")
		case errors.Is(err, service.ErrNotFound):
			respondError(c, http.StatusNotFound, "post not found")
		case errors.Is(err, service.ErrExternalAPI):
			respondError(c, http.StatusBadGateway, "photo storage error")
		default:
			respondError(c, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	result := make([]PhotoJSON, 0, len(photos))
	for _, p := range photos {
		url := ""
		if h.r2Client != nil {
			u, err := h.r2Client.PresignGetURL(p.R2Key, time.Hour)
			if err != nil {
				respondError(c, http.StatusInternalServerError, "failed to generate photo URL")
				return
			}
			url = u
		}
		result = append(result, PhotoJSON{ID: p.ID, PhotoType: p.PhotoType, URL: url})
	}

	c.JSON(http.StatusCreated, UploadPhotosResponse{Photos: result})
}

// readFormFile は multipart フォームから指定フィールドのファイルを読み取る。
// フィールドが存在しない場合は (nil, nil) を返す（任意フィールド用）。
func readFormFile(c *gin.Context, field string) (*service.UploadFile, error) {
	file, header, err := c.Request.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return &service.UploadFile{
		ContentType: contentTypeOf(header),
		Data:        data,
	}, nil
}

// contentTypeOf は multipart ヘッダから Content-Type を取り出す
func contentTypeOf(header *multipart.FileHeader) string {
	return header.Header.Get("Content-Type")
}
