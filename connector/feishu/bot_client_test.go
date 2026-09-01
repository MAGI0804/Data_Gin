package feishu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type fakeBotTokenProvider struct {
	token        string
	refreshToken string
	refreshes    int
}

func (provider *fakeBotTokenProvider) Token(context.Context) (string, error) {
	return provider.token, nil
}

func (provider *fakeBotTokenProvider) Refresh(context.Context) (string, error) {
	provider.refreshes++
	return provider.refreshToken, nil
}

func TestBotClientUploadsAndSendsFile(t *testing.T) {
	t.Parallel()
	var uploadSeen, messageSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/open-apis/im/v1/files":
			uploadSeen = true
			if request.Header.Get("Authorization") != "Bearer tenant-token-value-123" {
				t.Fatalf("upload authorization = %q", request.Header.Get("Authorization"))
			}
			if err := request.ParseMultipartForm(maxBotFileBytes); err != nil {
				t.Fatalf("ParseMultipartForm() error = %v", err)
			}
			if got := request.FormValue("file_type"); got != "xls" {
				t.Fatalf("file_type = %q", got)
			}
			file, header, err := request.FormFile("file")
			if err != nil {
				t.Fatalf("FormFile() error = %v", err)
			}
			defer file.Close()
			body, _ := io.ReadAll(file)
			if header.Filename != "daily.xlsx" || string(body) != "xlsx-data" {
				t.Fatalf("upload = %q %q", header.Filename, body)
			}
			_, _ = response.Write([]byte(`{"code":0,"data":{"file_key":"file-key-123"}}`))
		case "/open-apis/im/v1/messages":
			messageSeen = true
			if request.URL.Query().Get("receive_id_type") != "chat_id" {
				t.Fatalf("receive_id_type = %q", request.URL.Query().Get("receive_id_type"))
			}
			var payload struct {
				ReceiveID string `json:"receive_id"`
				Message   string `json:"msg_type"`
				Content   string `json:"content"`
				UUID      string `json:"uuid"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload.ReceiveID != "oc_chat" || payload.Message != "file" || payload.UUID != "run-uuid-1" || payload.Content != `{"file_key":"file-key-123"}` {
				t.Fatalf("message payload = %#v", payload)
			}
			_, _ = response.Write([]byte(`{"code":0,"data":{"message_id":"message-id-123"}}`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	provider := &fakeBotTokenProvider{token: "tenant-token-value-123"}
	client, err := newBotClient(server.URL, provider, server.Client(), true)
	if err != nil {
		t.Fatalf("newBotClient() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "daily.xlsx")
	if err := os.WriteFile(path, []byte("xlsx-data"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	fileKey, err := client.UploadFile(t.Context(), path, "daily.xlsx")
	if err != nil || fileKey != "file-key-123" {
		t.Fatalf("UploadFile() = %q, %v", fileKey, err)
	}
	messageID, err := client.SendFile(t.Context(), "chat_id", "oc_chat", fileKey, "run-uuid-1")
	if err != nil || messageID != "message-id-123" {
		t.Fatalf("SendFile() = %q, %v", messageID, err)
	}
	if !uploadSeen || !messageSeen {
		t.Fatalf("uploadSeen=%t messageSeen=%t", uploadSeen, messageSeen)
	}
}

func TestBotClientRefreshesOnceAfterUnauthorized(t *testing.T) {
	t.Parallel()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests++
		if request.Header.Get("Authorization") == "Bearer stale-token-value" {
			response.WriteHeader(http.StatusUnauthorized)
			_, _ = response.Write([]byte(`{"code":99991663}`))
			return
		}
		_, _ = response.Write([]byte(`{"code":0,"data":{"message_id":"message-id-456"}}`))
	}))
	defer server.Close()
	provider := &fakeBotTokenProvider{token: "stale-token-value", refreshToken: "fresh-token-value"}
	client, err := newBotClient(server.URL, provider, server.Client(), true)
	if err != nil {
		t.Fatalf("newBotClient() error = %v", err)
	}
	messageID, err := client.SendText(t.Context(), "chat_id", "oc_chat", "日报已生成", "run-uuid-2")
	if err != nil || messageID != "message-id-456" || requests != 2 || provider.refreshes != 1 {
		t.Fatalf("SendText() = %q, %v requests=%d refreshes=%d", messageID, err, requests, provider.refreshes)
	}
}
