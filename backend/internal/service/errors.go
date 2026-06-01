// Package service はサービス層のドメインエラー型を定義する
package service

import "errors"

// Service 層で使用するエラー型（ハンドラーがHTTPステータスコードにマップする）
var (
	// ErrNotFound はリソースが見つからない場合に返す → 404
	ErrNotFound = errors.New("not found")
	// ErrForbidden はアクセス権限がない場合に返す → 403
	ErrForbidden = errors.New("forbidden")
	// ErrUnauthorized は認証失敗の場合に返す → 401
	ErrUnauthorized = errors.New("unauthorized")
	// ErrConflict は重複・競合の場合に返す → 409
	ErrConflict = errors.New("conflict")
	// ErrValidation はバリデーションエラーの場合に返す → 422
	ErrValidation = errors.New("validation error")
	// ErrExternalAPI は外部 API エラーの場合に返す → 502
	ErrExternalAPI = errors.New("external api error")
	// ErrConstraintViolation は DB 制約違反の場合に返す
	ErrConstraintViolation = errors.New("constraint violation")
)
