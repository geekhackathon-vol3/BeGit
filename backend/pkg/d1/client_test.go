package d1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// d1ResponseMock は D1 REST API のレスポンスを生成するヘルパー
func mockD1Response(results []map[string]interface{}, success bool, errMsg string) []byte {
	type resultItem struct {
		Results []map[string]interface{} `json:"results"`
		Success bool                     `json:"success"`
		Meta    map[string]interface{}   `json:"meta"`
	}
	type d1Response struct {
		Result  []resultItem `json:"result"`
		Success bool         `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	resp := d1Response{
		Success: success,
		Result: []resultItem{
			{
				Results: results,
				Success: success,
				Meta:    map[string]interface{}{"rows_written": 0},
			},
		},
	}
	if errMsg != "" {
		resp.Errors = append(resp.Errors, struct {
			Message string `json:"message"`
		}{Message: errMsg})
	}
	data, _ := json.Marshal(resp)
	return data
}

// TestClientQuery はモック HTTP レスポンスに対して Query が正しく動作することを確認する
func TestClientQuery(t *testing.T) {
	expected := []map[string]interface{}{
		{"id": float64(1), "name": "test"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockD1Response(expected, true, ""))
	}))
	defer server.Close()

	client := &d1Client{
		httpClient:  server.Client(),
		accountID:   "test_account",
		databaseID:  "test_db",
		apiToken:    "test_token",
		apiEndpoint: server.URL + "/client/v4/accounts/{account_id}/d1/database/{database_id}/query",
	}

	rows, err := client.Query(context.Background(), "SELECT * FROM test", nil)
	if err != nil {
		t.Fatalf("Query() failed: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["name"] != "test" {
		t.Errorf("expected name=test, got %v", rows[0]["name"])
	}
}

// TestClientQuery_EmptyResult は空結果が ErrNotFound を返すことを確認する
func TestClientQuery_EmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockD1Response([]map[string]interface{}{}, true, ""))
	}))
	defer server.Close()

	client := &d1Client{
		httpClient:  server.Client(),
		accountID:   "test_account",
		databaseID:  "test_db",
		apiToken:    "test_token",
		apiEndpoint: server.URL + "/client/v4/accounts/{account_id}/d1/database/{database_id}/query",
	}

	_, err := client.Query(context.Background(), "SELECT * FROM test WHERE id = ?", []interface{}{999})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestClientQuery_ConstraintViolation は UNIQUE 制約違反が ErrConstraintViolation を返すことを確認する
func TestClientQuery_ConstraintViolation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		data, _ := json.Marshal(map[string]interface{}{
			"success": false,
			"errors": []map[string]interface{}{
				{"message": "UNIQUE constraint failed: users.github_id", "code": 1000},
			},
			"result": []interface{}{},
		})
		w.Write(data)
	}))
	defer server.Close()

	client := &d1Client{
		httpClient:  server.Client(),
		accountID:   "test_account",
		databaseID:  "test_db",
		apiToken:    "test_token",
		apiEndpoint: server.URL + "/client/v4/accounts/{account_id}/d1/database/{database_id}/query",
	}

	_, err := client.Exec(context.Background(), "INSERT INTO users VALUES (?)", []interface{}{"dup"})
	if !errors.Is(err, ErrConstraintViolation) {
		t.Errorf("expected ErrConstraintViolation, got %v", err)
	}
}
