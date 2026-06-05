package management

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOpenAIRequestLogTextExtractsSummary(t *testing.T) {
	content := []byte(`=== REQUEST INFO ===
URL: /v1/responses
Method: POST
Timestamp: 2026-05-29T01:02:03Z

=== REQUEST BODY ===
{"model":"gpt-5.4","input":"hello"}

=== API REQUEST ===
=== API REQUEST 1 ===
Timestamp: 2026-05-29T01:02:04Z
Auth: provider=codex, auth_id=codex-user.json, label=main, type=oauth

Body:
{"model":"gpt-5.4","input":"hello"}

=== API RESPONSE ===
=== API RESPONSE 1 ===
Timestamp: 2026-05-29T01:02:05Z
Status: 200
Body:
{"usage":{"input_tokens":1000,"output_tokens":100,"total_tokens":1100,"input_tokens_details":{"cached_tokens":200},"output_tokens_details":{"reasoning_tokens":30}}}
`)

	entry := openAIRequestLogEntry{Name: "v1-responses-2026-05-29T010203-req1.log", RequestID: "req1"}
	parseOpenAIRequestLogText(content, &entry)
	applyOpenAICost(&entry)

	if entry.TimeBeijing != "2026-05-29 09:02:03" {
		t.Fatalf("TimeBeijing = %q, want 2026-05-29 09:02:03", entry.TimeBeijing)
	}
	if entry.Model != "gpt-5.4" {
		t.Fatalf("Model = %q, want gpt-5.4", entry.Model)
	}
	if entry.AuthFile != "codex-user.json" || entry.AuthLabel != "main" || entry.AuthType != "oauth" {
		t.Fatalf("auth fields = file:%q label:%q type:%q", entry.AuthFile, entry.AuthLabel, entry.AuthType)
	}
	if entry.InputTokens != 1000 || entry.OutputTokens != 100 || entry.CachedTokens != 200 || entry.ReasoningTokens != 30 || entry.TotalTokens != 1100 {
		t.Fatalf("tokens = input:%d output:%d cached:%d reasoning:%d total:%d", entry.InputTokens, entry.OutputTokens, entry.CachedTokens, entry.ReasoningTokens, entry.TotalTokens)
	}
	if !entry.PriceAvailable || entry.PricingModel != "gpt-5.4" {
		t.Fatalf("pricing = available:%t model:%q", entry.PriceAvailable, entry.PricingModel)
	}
	if entry.CostUSD != 0.00355 {
		t.Fatalf("CostUSD = %v, want 0.00355", entry.CostUSD)
	}
}

func TestParseOpenAIRequestLogTextExtractsAPIKeyAuthType(t *testing.T) {
	content := []byte(`Timestamp: 2026-05-29T01:02:03Z
Auth: provider=openai, auth_id=openai-key.yaml, label=backup, type=api_key value=sk-***abcd
{"model":"gpt-5","usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}
`)

	var entry openAIRequestLogEntry
	parseOpenAIRequestLogText(content, &entry)

	if entry.AuthFile != "openai-key.yaml" || entry.AuthLabel != "backup" || entry.AuthType != "api_key" {
		t.Fatalf("auth fields = file:%q label:%q type:%q", entry.AuthFile, entry.AuthLabel, entry.AuthType)
	}
}

func TestParseOpenAIRequestLogTextDeduplicatesUsage(t *testing.T) {
	content := []byte(`Timestamp: 2026-05-29T01:02:03Z
{"model":"gpt-5","usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}
{"model":"gpt-5","usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}
`)

	var entry openAIRequestLogEntry
	parseOpenAIRequestLogText(content, &entry)

	if entry.InputTokens != 10 || entry.OutputTokens != 5 || entry.TotalTokens != 15 {
		t.Fatalf("deduped tokens = input:%d output:%d total:%d", entry.InputTokens, entry.OutputTokens, entry.TotalTokens)
	}
}

func TestApplyOpenAICostAppliesGPT54LongContextPricing(t *testing.T) {
	entry := openAIRequestLogEntry{
		Model:        "gpt-5.4",
		InputTokens:  300000,
		CachedTokens: 100000,
		OutputTokens: 1000,
		TotalTokens:  301000,
	}

	applyOpenAICost(&entry)

	if !entry.PriceAvailable || entry.PricingModel != "gpt-5.4" {
		t.Fatalf("pricing = available:%t model:%q", entry.PriceAvailable, entry.PricingModel)
	}
	if entry.CostUSD != 1.0725 {
		t.Fatalf("CostUSD = %v, want 1.0725", entry.CostUSD)
	}
	if entry.PricingPerMTokens["input"] != 5 || entry.PricingPerMTokens["cached_input"] != 0.5 || entry.PricingPerMTokens["output"] != 22.5 {
		t.Fatalf("pricing_per_m_tokens = %#v", entry.PricingPerMTokens)
	}
	if entry.PricingNote == "" {
		t.Fatal("expected long-context pricing note")
	}
}

func TestLookupOpenAIModelPriceMatchesVersionedModel(t *testing.T) {
	price, ok := lookupOpenAIModelPrice("openai/gpt-5.4-2026-03-05")
	if !ok {
		t.Fatal("expected versioned gpt-5.4 model to match pricing")
	}
	if price.Model != "gpt-5.4" {
		t.Fatalf("price model = %q, want gpt-5.4", price.Model)
	}
}

func TestLookupOpenAIModelPriceMatchesGPT54Pro(t *testing.T) {
	price, ok := lookupOpenAIModelPrice("openai/gpt-5.4-pro-2026-03-05")
	if !ok {
		t.Fatal("expected versioned gpt-5.4-pro model to match pricing")
	}
	if price.Model != "gpt-5.4-pro" {
		t.Fatalf("price model = %q, want gpt-5.4-pro", price.Model)
	}
}

func TestCollectOpenAIRequestLogCandidatesSkipsErrorLogs(t *testing.T) {
	dir := t.TempDir()

	requestLogPath := filepath.Join(dir, "v1-responses-2026-05-29T010203-req1.log")
	if errWriteRequest := os.WriteFile(requestLogPath, []byte("ok"), 0644); errWriteRequest != nil {
		t.Fatalf("failed to create request log: %v", errWriteRequest)
	}

	errorLogPath := filepath.Join(dir, "error-2026-05-29.log")
	if errWriteError := os.WriteFile(errorLogPath, []byte("error"), 0644); errWriteError != nil {
		t.Fatalf("failed to create error log: %v", errWriteError)
	}

	mainLogPath := filepath.Join(dir, "main.log")
	if errWriteMain := os.WriteFile(mainLogPath, []byte("main"), 0644); errWriteMain != nil {
		t.Fatalf("failed to create main log: %v", errWriteMain)
	}

	entries, errRead := os.ReadDir(dir)
	if errRead != nil {
		t.Fatalf("failed to read dir: %v", errRead)
	}

	candidates := collectOpenAIRequestLogCandidates(dir, entries)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 request log candidate, got %d", len(candidates))
	}
	if candidates[0].Name != filepath.Base(requestLogPath) {
		t.Fatalf("unexpected candidate name %q", candidates[0].Name)
	}
}
