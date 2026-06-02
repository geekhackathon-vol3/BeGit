// Package r2 は Cloudflare R2 への S3 互換 API クライアントを提供する。
// Workers Container から R2 バケット（begit-photos）への
// オブジェクトのアップロード（PUT）と署名付き取得 URL の生成を行う。
//
// 認証は AWS Signature Version 4（SigV4）で行う。R2 の S3 互換エンドポイント
//
//	https://<account_id>.r2.cloudflarestorage.com/<bucket>/<key>
//
// に対し、region="auto" / service="s3" で署名する。
package r2

import "errors"

// エラー型定義
var (
	// ErrUpload は R2 へのオブジェクト PUT が失敗した場合に返す
	ErrUpload = errors.New("r2 upload failed")
)
