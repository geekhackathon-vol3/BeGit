package r2

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	// service は S3 互換 API のサービス名
	service = "s3"
	// algorithm は SigV4 のアルゴリズム識別子
	algorithm = "AWS4-HMAC-SHA256"
	// unsignedPayload は body を署名対象にしない場合のペイロードハッシュ（presign で使用）
	unsignedPayload = "UNSIGNED-PAYLOAD"
)

// region は R2 が SigV4 で要求するリージョン。R2 は "auto" を受け付ける。
// テストで AWS 公式署名ベクタ（us-east-1）を検証するため var にしている。
var region = "auto"

// Client は R2 オブジェクトストレージの操作インターフェース
type Client interface {
	// PutObject は body を key で R2 にアップロードする。失敗時は ErrUpload を返す。
	PutObject(ctx context.Context, key, contentType string, body []byte) error
	// PresignGetURL は key を取得するための署名付き URL を生成する（ttl の間有効）。
	PresignGetURL(key string, ttl time.Duration) (string, error)
}

// client は Client インターフェースの実装
type client struct {
	httpClient      *http.Client
	accountID       string
	accessKeyID     string
	secretAccessKey string
	bucket          string
	// host はテスト用にオーバーライド可能。空なら accountID から導出する。
	host string
	// now は時刻取得関数。テストで固定値に差し替える。
	now func() time.Time
}

// NewClient は R2 S3 互換クライアントを作成する
func NewClient(accountID, accessKeyID, secretAccessKey, bucket string) Client {
	return &client{
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		accountID:       accountID,
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		bucket:          bucket,
		now:             time.Now,
	}
}

// endpointHost は R2 の S3 互換エンドポイントのホスト名を返す
func (c *client) endpointHost() string {
	if c.host != "" {
		return c.host
	}
	return c.accountID + ".r2.cloudflarestorage.com"
}

// canonicalURI は path-style の正規化済み URI（/<bucket>/<key>）を返す
func (c *client) canonicalURI(key string) string {
	return "/" + uriEncode(c.bucket, false) + "/" + uriEncode(key, false)
}

// PutObject は body を key で R2 にアップロードする
func (c *client) PutObject(ctx context.Context, key, contentType string, body []byte) error {
	host := c.endpointHost()
	uri := c.canonicalURI(key)
	urlStr := "https://" + host + uri

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, urlStr, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("r2: failed to create request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	payloadHash := sha256Hex(body)
	c.signHeader(req, host, uri, payloadHash, c.now().UTC())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpload, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		return fmt.Errorf("%w: status %d: %s", ErrUpload, resp.StatusCode, buf.String())
	}
	return nil
}

// signHeader は PUT/GET 等のリクエストに SigV4 のヘッダ署名を付与する。
// 署名対象ヘッダは host / x-amz-content-sha256 / x-amz-date の 3 つに固定する。
func (c *client) signHeader(req *http.Request, host, uri, payloadHash string, t time.Time) {
	amzDate := t.Format("20060102T150405Z")
	dateStamp := t.Format("20060102")

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := "host:" + host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"

	canonicalRequest := strings.Join([]string{
		req.Method,
		uri,
		"", // canonical query string（PUT/GET では空）
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := credentialScope(dateStamp)
	signature := sign(c.secretAccessKey, dateStamp, amzDate, scope, canonicalRequest)

	req.Header.Set("Authorization", fmt.Sprintf(
		"%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, c.accessKeyID, scope, signedHeaders, signature,
	))
}

// PresignGetURL は key を取得するための署名付き URL（クエリ文字列署名）を生成する
func (c *client) PresignGetURL(key string, ttl time.Duration) (string, error) {
	host := c.endpointHost()
	uri := c.canonicalURI(key)
	expires := int(ttl.Seconds())
	if expires <= 0 {
		expires = 3600
	}
	return buildPresignedGetURL(c.accessKeyID, c.secretAccessKey, host, uri, expires, c.now().UTC()), nil
}

// buildPresignedGetURL は SigV4 のクエリ文字列署名で GET 用の署名付き URL を構築する。
// region / accessKeyID / host / uri / time を引数で受け取り、テストから AWS 公式ベクタを検証できるようにする。
func buildPresignedGetURL(accessKeyID, secretAccessKey, host, uri string, expires int, t time.Time) string {
	amzDate := t.Format("20060102T150405Z")
	dateStamp := t.Format("20060102")
	scope := credentialScope(dateStamp)

	// 署名対象クエリ（キー昇順。値・キーとも URI エンコード。Credential の "/" は %2F に）
	query := strings.Join([]string{
		"X-Amz-Algorithm=" + algorithm,
		"X-Amz-Credential=" + uriEncode(accessKeyID+"/"+scope, true),
		"X-Amz-Date=" + amzDate,
		"X-Amz-Expires=" + fmt.Sprintf("%d", expires),
		"X-Amz-SignedHeaders=host",
	}, "&")

	canonicalHeaders := "host:" + host + "\n"
	canonicalRequest := strings.Join([]string{
		http.MethodGet,
		uri,
		query,
		canonicalHeaders,
		"host",
		unsignedPayload,
	}, "\n")

	signature := sign(secretAccessKey, dateStamp, amzDate, scope, canonicalRequest)

	return "https://" + host + uri + "?" + query + "&X-Amz-Signature=" + signature
}

// credentialScope は <date>/<region>/<service>/aws4_request を返す
func credentialScope(dateStamp string) string {
	return dateStamp + "/" + region + "/" + service + "/aws4_request"
}

// sign は canonical request から SigV4 署名（16進文字列）を計算する
func sign(secretAccessKey, dateStamp, amzDate, scope, canonicalRequest string) string {
	stringToSign := strings.Join([]string{
		algorithm,
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := deriveSigningKey(secretAccessKey, dateStamp)
	return hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
}

// deriveSigningKey は SigV4 の署名鍵を導出する
func deriveSigningKey(secretAccessKey, dateStamp string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretAccessKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// uriEncode は RFC 3986 の unreserved 文字以外をパーセントエンコードする。
// encodeSlash=false のとき '/' はエンコードしない（パス用）。true のときエンコードする（クエリ値用）。
func uriEncode(s string, encodeSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') ||
			(ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.' || ch == '~':
			b.WriteByte(ch)
		case ch == '/' && !encodeSlash:
			b.WriteByte(ch)
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}
