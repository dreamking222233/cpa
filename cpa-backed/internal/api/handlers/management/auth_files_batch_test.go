package management

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func TestUploadAuthFile_BatchMultipart(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	files := []struct {
		name    string
		content string
	}{
		{name: "alpha.json", content: `{"type":"codex","email":"alpha@example.com"}`},
		{name: "beta.json", content: `{"type":"claude","email":"beta@example.com"}`},
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, file := range files {
		part, err := writer.CreateFormFile("file", file.name)
		if err != nil {
			t.Fatalf("failed to create multipart file: %v", err)
		}
		if _, err = part.Write([]byte(file.content)); err != nil {
			t.Fatalf("failed to write multipart content: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	h.UploadAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected upload status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got, ok := payload["uploaded"].(float64); !ok || int(got) != len(files) {
		t.Fatalf("expected uploaded=%d, got %#v", len(files), payload["uploaded"])
	}

	for _, file := range files {
		fullPath := filepath.Join(authDir, file.name)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("expected uploaded file %s to exist: %v", file.name, err)
		}
		if string(data) != file.content {
			t.Fatalf("expected file %s content %q, got %q", file.name, file.content, string(data))
		}
	}

	auths := manager.List()
	if len(auths) != len(files) {
		t.Fatalf("expected %d auth entries, got %d", len(files), len(auths))
	}
}

func TestUploadAuthFile_BatchMultipart_InvalidJSONDoesNotOverwriteExistingFile(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	existingName := "alpha.json"
	existingContent := `{"type":"codex","email":"alpha@example.com"}`
	if err := os.WriteFile(filepath.Join(authDir, existingName), []byte(existingContent), 0o600); err != nil {
		t.Fatalf("failed to seed existing auth file: %v", err)
	}

	files := []struct {
		name    string
		content string
	}{
		{name: existingName, content: `{"type":"codex"`},
		{name: "beta.json", content: `{"type":"claude","email":"beta@example.com"}`},
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, file := range files {
		part, err := writer.CreateFormFile("file", file.name)
		if err != nil {
			t.Fatalf("failed to create multipart file: %v", err)
		}
		if _, err = part.Write([]byte(file.content)); err != nil {
			t.Fatalf("failed to write multipart content: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	h.UploadAuthFile(ctx)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("expected upload status %d, got %d with body %s", http.StatusMultiStatus, rec.Code, rec.Body.String())
	}

	data, err := os.ReadFile(filepath.Join(authDir, existingName))
	if err != nil {
		t.Fatalf("expected existing auth file to remain readable: %v", err)
	}
	if string(data) != existingContent {
		t.Fatalf("expected existing auth file to remain %q, got %q", existingContent, string(data))
	}

	betaData, err := os.ReadFile(filepath.Join(authDir, "beta.json"))
	if err != nil {
		t.Fatalf("expected valid auth file to be created: %v", err)
	}
	if string(betaData) != files[1].content {
		t.Fatalf("expected beta auth file content %q, got %q", files[1].content, string(betaData))
	}
}

func TestDeleteAuthFile_BatchQuery(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	files := []string{"alpha.json", "beta.json"}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(authDir, name), []byte(`{"type":"codex"}`), 0o600); err != nil {
			t.Fatalf("failed to write auth file %s: %v", name, err)
		}
	}

	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = &memoryAuthStore{}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(
		http.MethodDelete,
		"/v0/management/auth-files?name="+url.QueryEscape(files[0])+"&name="+url.QueryEscape(files[1]),
		nil,
	)
	ctx.Request = req

	h.DeleteAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected delete status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got, ok := payload["deleted"].(float64); !ok || int(got) != len(files) {
		t.Fatalf("expected deleted=%d, got %#v", len(files), payload["deleted"])
	}

	for _, name := range files {
		if _, err := os.Stat(filepath.Join(authDir, name)); !os.IsNotExist(err) {
			t.Fatalf("expected auth file %s to be removed, stat err: %v", name, err)
		}
	}
}

func TestPatchAuthFilesStatusBatch_DisablesSelectedAuths(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)
	ctx := contextForTest(t)
	if _, err := manager.Register(ctx, &coreauth.Auth{ID: "alpha-id", FileName: "alpha.json", Provider: "codex", Status: coreauth.StatusActive}); err != nil {
		t.Fatalf("failed to register alpha auth: %v", err)
	}
	if _, err := manager.Register(ctx, &coreauth.Auth{ID: "beta-id", FileName: "beta.json", Provider: "claude", Status: coreauth.StatusActive}); err != nil {
		t.Fatalf("failed to register beta auth: %v", err)
	}

	body := bytes.NewBufferString(`{"names":["alpha.json","beta-id"],"disabled":true}`)
	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	ginCtx.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/status/batch", body)
	ginCtx.Request.Header.Set("Content-Type", "application/json")

	h.PatchAuthFilesStatusBatch(ginCtx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	for _, id := range []string{"alpha-id", "beta-id"} {
		auth, ok := manager.GetByID(id)
		if !ok {
			t.Fatalf("expected auth %s to exist", id)
		}
		if !auth.Disabled || auth.Status != coreauth.StatusDisabled {
			t.Fatalf("expected auth %s disabled, got disabled=%t status=%s", id, auth.Disabled, auth.Status)
		}
	}
}

func TestPatchAuthFilesStatusBatch_EnablesSelectedAuths(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)
	ctx := contextForTest(t)
	if _, err := manager.Register(ctx, &coreauth.Auth{ID: "alpha-id", FileName: "alpha.json", Provider: "codex", Disabled: true, Status: coreauth.StatusDisabled, StatusMessage: "disabled via management API"}); err != nil {
		t.Fatalf("failed to register alpha auth: %v", err)
	}

	body := bytes.NewBufferString(`{"names":["alpha-id"],"disabled":false}`)
	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	ginCtx.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/status/batch", body)
	ginCtx.Request.Header.Set("Content-Type", "application/json")

	h.PatchAuthFilesStatusBatch(ginCtx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	auth, ok := manager.GetByID("alpha-id")
	if !ok {
		t.Fatal("expected alpha auth to exist")
	}
	if auth.Disabled || auth.Status != coreauth.StatusActive || auth.StatusMessage != "" {
		t.Fatalf("expected auth enabled, got disabled=%t status=%s message=%q", auth.Disabled, auth.Status, auth.StatusMessage)
	}
}

func TestPatchAuthFilesStatusBatch_PartialFailure(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)
	ctx := contextForTest(t)
	if _, err := manager.Register(ctx, &coreauth.Auth{ID: "alpha-id", FileName: "alpha.json", Provider: "codex", Status: coreauth.StatusActive}); err != nil {
		t.Fatalf("failed to register alpha auth: %v", err)
	}

	body := bytes.NewBufferString(`{"names":["alpha.json","missing.json"],"disabled":true}`)
	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	ginCtx.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/status/batch", body)
	ginCtx.Request.Header.Set("Content-Type", "application/json")

	h.PatchAuthFilesStatusBatch(ginCtx)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusMultiStatus, rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got, ok := payload["updated"].(float64); !ok || int(got) != 1 {
		t.Fatalf("expected updated=1, got %#v", payload["updated"])
	}
	failed, ok := payload["failed"].([]any)
	if !ok || len(failed) != 1 {
		t.Fatalf("expected one failed item, got %#v", payload["failed"])
	}
	auth, ok := manager.GetByID("alpha-id")
	if !ok || !auth.Disabled {
		t.Fatalf("expected existing auth to be disabled despite partial failure")
	}
}

func TestPatchAuthFilesStatusBatch_PersistFailureDoesNotDirtyRuntimeAuth(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)
	h.tokenStore = failingAuthStore{}
	ctx := contextForTest(t)
	if _, err := manager.Register(ctx, &coreauth.Auth{
		ID:       "alpha-id",
		FileName: "alpha.json",
		Provider: "codex",
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			"path": filepath.Join(t.TempDir(), "alpha.json"),
		},
		Metadata: map[string]any{"type": "codex"},
	}); err != nil {
		t.Fatalf("failed to register alpha auth: %v", err)
	}

	body := bytes.NewBufferString(`{"names":["alpha.json"],"disabled":true}`)
	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	ginCtx.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/status/batch", body)
	ginCtx.Request.Header.Set("Content-Type", "application/json")

	h.PatchAuthFilesStatusBatch(ginCtx)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusMultiStatus, rec.Code, rec.Body.String())
	}
	auth, ok := manager.GetByID("alpha-id")
	if !ok {
		t.Fatal("expected alpha auth to exist")
	}
	if auth.Disabled || auth.Status != coreauth.StatusActive {
		t.Fatalf("expected runtime auth to remain active after persist failure, got disabled=%t status=%s", auth.Disabled, auth.Status)
	}
}

func TestPatchAuthFilesStatusBatch_RequiresNamesAndDisabled(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)

	tests := []struct {
		name string
		body string
	}{
		{name: "missing disabled", body: `{"names":["alpha.json"]}`},
		{name: "missing names", body: `{"disabled":true}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ginCtx, _ := gin.CreateTestContext(rec)
			ginCtx.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/status/batch", bytes.NewBufferString(tt.body))
			ginCtx.Request.Header.Set("Content-Type", "application/json")

			h.PatchAuthFilesStatusBatch(ginCtx)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d with body %s", http.StatusBadRequest, rec.Code, rec.Body.String())
			}
		})
	}
}

func contextForTest(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}

type failingAuthStore struct{}

func (failingAuthStore) List(context.Context) ([]*coreauth.Auth, error) {
	return nil, nil
}

func (failingAuthStore) Save(context.Context, *coreauth.Auth) (string, error) {
	return "", errors.New("forced save failure")
}

func (failingAuthStore) Delete(context.Context, string) error {
	return nil
}
