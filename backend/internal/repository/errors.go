// Package repository はドメインエラー型を定義する
package repository

import "errors"

// Repository 層で使用するエラー型
var (
	// ErrNotFound はレコードが見つからない場合に返す
	ErrNotFound = errors.New("not found")
	// ErrConflict は重複制約違反の場合に返す
	ErrConflict = errors.New("conflict")
	// ErrConstraintViolation は DB 制約違反の場合に返す
	ErrConstraintViolation = errors.New("constraint violation")
)
