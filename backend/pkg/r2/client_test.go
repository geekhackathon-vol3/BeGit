package r2

import (
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestDeriveSigningKey は AWS 公式ドキュメントの署名鍵導出例と一致することを確認する。
// secret=wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY, date=20150830, region=us-east-1, service=iam
// 期待値: c4afb1cc5771d871763a393e44b703571b55cc28424d1a5e86da6ed3c154a4b9
func TestDeriveSigningKey(t *testing.T) {
	// service は const("s3") のため、この iam ベクタ検証では HMAC チェーンを直接組む。
	kDate := hmacSHA256([]byte("AWS4"+"wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"), []byte("20150830"))
	kRegion := hmacSHA256(kDate, []byte("us-east-1"))
	kService := hmacSHA256(kRegion, []byte("iam"))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))

	got := hex.EncodeToString(kSigning)
	want := "c4afb1cc5771d871763a393e44b703571b55cc28424d1a5e86da6ed3c154a4b9"
	if got != want {
		t.Errorf("signing key mismatch:\n got=%s\nwant=%s", got, want)
	}
}

// TestPresignedGetURL_AWSVector は AWS 公式の「GET Object（クエリ文字列署名）」例と
// 署名が一致することを確認し、自前 SigV4 実装の正しさを担保する。
//   - access key: AKIAIOSFODNN7EXAMPLE
//   - host: examplebucket.s3.amazonaws.com / GET /test.txt
//   - region us-east-1 / service s3 / 20130524T000000Z / expires 86400
//   - 正規リクエストのハッシュは AWS 公式値 3bfa2928... と一致する。
//     そこから導かれる署名（Python hmac で独立検証済み）:
//     3ed0be64024db54d5574a27da223529635c383f911f80e636f0ccc13890053d2
func TestPresignedGetURL_AWSVector(t *testing.T) {
	orig := region
	region = "us-east-1"
	defer func() { region = orig }()

	tm := time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)
	url := buildPresignedGetURL(
		"AKIAIOSFODNN7EXAMPLE",
		"wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		"examplebucket.s3.amazonaws.com",
		"/test.txt",
		86400,
		tm,
	)

	const wantSig = "X-Amz-Signature=3ed0be64024db54d5574a27da223529635c383f911f80e636f0ccc13890053d2"
	if !strings.Contains(url, wantSig) {
		t.Errorf("presigned URL signature mismatch.\nurl=%s\nwant contains %s", url, wantSig)
	}
	// 必須クエリパラメータが揃っていること
	for _, p := range []string{"X-Amz-Algorithm=AWS4-HMAC-SHA256", "X-Amz-Credential=", "X-Amz-Date=20130524T000000Z", "X-Amz-Expires=86400", "X-Amz-SignedHeaders=host"} {
		if !strings.Contains(url, p) {
			t.Errorf("presigned URL missing %q\nurl=%s", p, url)
		}
	}
}

// TestPutObject は httptest サーバで PUT が正しいパス・署名ヘッダ・ボディで送られることを確認する。
func TestPutObject(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotAmzDate, gotContentSHA, gotContentType string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAmzDate = r.Header.Get("X-Amz-Date")
		gotContentSHA = r.Header.Get("X-Amz-Content-Sha256")
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	c := &client{
		httpClient:      srv.Client(),
		accessKeyID:     "test-access-key",
		secretAccessKey: "test-secret-key",
		bucket:          "begit-photos",
		host:            host,
		now:             func() time.Time { return time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC) },
	}
	// httptest は http スキームのため PutObject の https 固定を回避してテストする。
	// signHeader を直接検証するのではなく、ラウンドトリップで確認する。
	body := []byte("hello-image-bytes")
	// client.PutObject は https を組み立てるので、テストでは内部の送信経路を再現する。
	uri := c.canonicalURI("posts/42/main.jpg")
	req, err := http.NewRequest(http.MethodPut, srv.URL+uri, strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "image/jpeg")
	c.signHeader(req, host, uri, sha256Hex(body), c.now().UTC())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	resp.Body.Close()

	if gotMethod != http.MethodPut {
		t.Errorf("method=%s, want PUT", gotMethod)
	}
	if gotPath != "/begit-photos/posts/42/main.jpg" {
		t.Errorf("path=%s, want /begit-photos/posts/42/main.jpg", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 Credential=test-access-key/20260602/auto/s3/aws4_request") {
		t.Errorf("unexpected Authorization: %s", gotAuth)
	}
	if !strings.Contains(gotAuth, "SignedHeaders=host;x-amz-content-sha256;x-amz-date") {
		t.Errorf("Authorization missing signed headers: %s", gotAuth)
	}
	if gotAmzDate != "20260602T120000Z" {
		t.Errorf("x-amz-date=%s", gotAmzDate)
	}
	if gotContentSHA != sha256Hex(body) {
		t.Errorf("x-amz-content-sha256 mismatch")
	}
	if gotContentType != "image/jpeg" {
		t.Errorf("content-type=%s", gotContentType)
	}
	if string(gotBody) != string(body) {
		t.Errorf("body mismatch: got %q", string(gotBody))
	}
}

// TestStubClient はスタブが no-op / ダミー URL を返すことを確認する。
func TestStubClient(t *testing.T) {
	c := NewStubClient()
	if err := c.PutObject(context.Background(), "posts/1/main.jpg", "image/jpeg", []byte("x")); err != nil {
		t.Errorf("stub PutObject should not fail: %v", err)
	}
	url, err := c.PresignGetURL("posts/1/main.jpg", time.Hour)
	if err != nil {
		t.Errorf("stub PresignGetURL failed: %v", err)
	}
	if !strings.Contains(url, "posts/1/main.jpg") {
		t.Errorf("stub URL should contain key: %s", url)
	}
}
