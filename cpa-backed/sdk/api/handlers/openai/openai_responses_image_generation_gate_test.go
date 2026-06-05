package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/api/handlers"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
	"github.com/tidwall/gjson"
)

func performResponsesEndpointRequest(t *testing.T, endpointPath string, body string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(endpointPath, handler)

	req := httptest.NewRequest(http.MethodPost, endpointPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestV1ResponsesRejectsImageGenerationTool(t *testing.T) {
	base := handlers.NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, nil)
	handler := NewOpenAIResponsesAPIHandler(base)

	resp := performResponsesEndpointRequest(t, "/v1/responses", `{"model":"gpt-5.4-mini","input":"draw","tools":[{"type":"image_generation"}]}`, handler.Responses)

	assertImageGenerationRequiresV2Response(t, resp)
}

func TestV1ResponsesRejectsGPTImage2Model(t *testing.T) {
	base := handlers.NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, nil)
	handler := NewOpenAIResponsesAPIHandler(base)

	resp := performResponsesEndpointRequest(t, "/v1/responses", `{"model":"gpt-image-2","input":"draw"}`, handler.Responses)

	assertImageGenerationRequiresV2Response(t, resp)
}

func TestV1ResponsesCompactRejectsImageGenerationTool(t *testing.T) {
	base := handlers.NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, nil)
	handler := NewOpenAIResponsesAPIHandler(base)

	resp := performResponsesEndpointRequest(t, "/v1/responses/compact", `{"model":"gpt-5.4-mini","input":"draw","tool_choice":{"type":"image_generation"}}`, handler.Compact)

	assertImageGenerationRequiresV2Response(t, resp)
}

func TestResponsesPayloadRequestsImageGeneration(t *testing.T) {
	testCases := []struct {
		name string
		body string
		want bool
	}{
		{name: "plain responses", body: `{"model":"gpt-5.4-mini","input":"hello"}`, want: false},
		{name: "gpt image model", body: `{"model":"gpt-image-2","input":"draw"}`, want: true},
		{name: "prefixed gpt image model", body: `{"model":"codex/gpt-image-2","input":"draw"}`, want: true},
		{name: "image tool", body: `{"model":"gpt-5.4-mini","tools":[{"type":"image_generation"}]}`, want: true},
		{name: "image tool choice type", body: `{"model":"gpt-5.4-mini","tool_choice":{"type":"image_generation"}}`, want: true},
		{name: "image tool choice name", body: `{"model":"gpt-5.4-mini","tool_choice":{"name":"image_generation"}}`, want: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := responsesPayloadRequestsImageGeneration([]byte(tc.body)); got != tc.want {
				t.Fatalf("responsesPayloadRequestsImageGeneration = %t, want %t", got, tc.want)
			}
		})
	}
}

func assertImageGenerationRequiresV2Response(t *testing.T, resp *httptest.ResponseRecorder) {
	t.Helper()

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", resp.Code, http.StatusBadRequest, resp.Body.String())
	}
	if got := gjson.GetBytes(resp.Body.Bytes(), "error.code").String(); got != "image_generation_requires_v2_responses" {
		t.Fatalf("error.code = %q, want image_generation_requires_v2_responses: %s", got, resp.Body.String())
	}
	if message := gjson.GetBytes(resp.Body.Bytes(), "error.message").String(); message != imageGenerationRequiresV2Message {
		t.Fatalf("error.message = %q, want %q", message, imageGenerationRequiresV2Message)
	}
}
