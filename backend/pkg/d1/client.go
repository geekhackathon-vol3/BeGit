// Package d1 は Cloudflare D1 REST API クライアントを提供する。
// Workers Container から D1 SQLite データベースへのアクセスに使用する。
package d1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// エラー型定義
var (
	// ErrNotFound はクエリ結果が0行の場合に返す
	ErrNotFound = errors.New("not found")
	// ErrConstraintViolation は UNIQUE 制約違反の場合に返す
	ErrConstraintViolation = errors.New("constraint violation")
	// ErrD1API は D1 REST API の HTTP エラーの場合に返す
	ErrD1API = errors.New("d1 api error")
)

// Client は D1 REST API の操作インターフェース
type Client interface {
	// Query は SELECT 等の結果セットを返す SQL を実行する。
	// 結果が0行の場合は ErrNotFound を返す。
	Query(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error)
	// Exec は INSERT/UPDATE/DELETE 等の結果セットを返さない SQL を実行する。
	// 影響行数と UNIQUE 制約違反などのエラーを返す。
	Exec(ctx context.Context, sql string, params []interface{}) (rowsAffected int64, err error)
}

// d1Client は Client インターフェースの実装
type d1Client struct {
	httpClient  *http.Client
	accountID   string
	databaseID  string
	apiToken    string
	apiEndpoint string // テスト用にオーバーライド可能
}

// d1QueryRequest は D1 REST API のリクエストボディ
type d1QueryRequest struct {
	SQL    string        `json:"sql"`
	Params []interface{} `json:"params"`
}

// d1APIResponse は D1 REST API のレスポンスボディ
type d1APIResponse struct {
	Result []struct {
		Results  []map[string]interface{} `json:"results"`
		Success  bool                     `json:"success"`
		Meta     map[string]interface{}   `json:"meta"`
		Error    *string                  `json:"error,omitempty"`
	} `json:"result"`
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

// NewClient は D1 REST API クライアントを作成する
func NewClient(accountID, databaseID, apiToken string) Client {
	return &d1Client{
		httpClient: &http.Client{},
		accountID:  accountID,
		databaseID: databaseID,
		apiToken:   apiToken,
	}
}

// getEndpoint はクエリ先の URL を返す
func (c *d1Client) getEndpoint() string {
	if c.apiEndpoint != "" {
		endpoint := c.apiEndpoint
		endpoint = strings.Replace(endpoint, "{account_id}", c.accountID, 1)
		endpoint = strings.Replace(endpoint, "{database_id}", c.databaseID, 1)
		return endpoint
	}
	return fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query",
		c.accountID, c.databaseID,
	)
}

// doRequest は D1 REST API へ POST リクエストを送信し、レスポンスを返す
func (c *d1Client) doRequest(ctx context.Context, sql string, params []interface{}) (*d1APIResponse, error) {
	if params == nil {
		params = []interface{}{}
	}

	body, err := json.Marshal(d1QueryRequest{SQL: sql, Params: params})
	if err != nil {
		return nil, fmt.Errorf("d1: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.getEndpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("d1: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("d1: request failed: %w", err)
	}
	defer resp.Body.Close()

	var apiResp d1APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("d1: failed to decode response: %w", err)
	}

	// エラーチェック
	if !apiResp.Success || len(apiResp.Errors) > 0 {
		for _, e := range apiResp.Errors {
			if strings.Contains(e.Message, "UNIQUE constraint failed") {
				return nil, ErrConstraintViolation
			}
		}
		errMsg := "unknown error"
		if len(apiResp.Errors) > 0 {
			errMsg = apiResp.Errors[0].Message
		}
		return nil, fmt.Errorf("%w: %s", ErrD1API, errMsg)
	}

	// result 内の個別クエリエラーもチェック
	for _, r := range apiResp.Result {
		if r.Error != nil && *r.Error != "" {
			if strings.Contains(*r.Error, "UNIQUE constraint failed") {
				return nil, ErrConstraintViolation
			}
			return nil, fmt.Errorf("%w: %s", ErrD1API, *r.Error)
		}
	}

	return &apiResp, nil
}

// Query は SELECT 等の結果セットを返す SQL を実行する
func (c *d1Client) Query(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
	apiResp, err := c.doRequest(ctx, sql, params)
	if err != nil {
		return nil, err
	}

	if len(apiResp.Result) == 0 || len(apiResp.Result[0].Results) == 0 {
		return nil, ErrNotFound
	}

	return apiResp.Result[0].Results, nil
}

// Exec は INSERT/UPDATE/DELETE 等の結果セットを返さない SQL を実行する
func (c *d1Client) Exec(ctx context.Context, sql string, params []interface{}) (int64, error) {
	apiResp, err := c.doRequest(ctx, sql, params)
	if err != nil {
		return 0, err
	}

	var rowsAffected int64
	if len(apiResp.Result) > 0 && apiResp.Result[0].Meta != nil {
		if v, ok := apiResp.Result[0].Meta["rows_written"]; ok {
			if f, ok := v.(float64); ok {
				rowsAffected = int64(f)
			}
		}
	}

	return rowsAffected, nil
}
