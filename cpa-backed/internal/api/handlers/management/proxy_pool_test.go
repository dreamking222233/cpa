package management

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestNormalizeProxyPoolEntryRejectsInvalidURL(t *testing.T) {
	_, err := normalizeProxyPoolEntry(config.ProxyPoolEntry{URL: "127.0.0.1:1080"})
	if err == nil {
		t.Fatal("expected invalid proxy URL error")
	}
}

func TestNormalizeProxyPoolEntryRejectsDirectMode(t *testing.T) {
	_, err := normalizeProxyPoolEntry(config.ProxyPoolEntry{URL: "direct"})
	if err == nil {
		t.Fatal("expected direct proxy pool entry to be rejected")
	}
}

func TestNormalizeProxyPoolEntryAcceptsHTTPProxy(t *testing.T) {
	entry, err := normalizeProxyPoolEntry(config.ProxyPoolEntry{URL: "http://127.0.0.1:8080"})
	if err != nil {
		t.Fatalf("expected HTTP proxy URL to be accepted: %v", err)
	}
	if entry.URL != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected normalized URL %q", entry.URL)
	}
}

func TestNormalizeProxyPoolStrategyDefaults(t *testing.T) {
	got := normalizeProxyPoolStrategy(config.ProxyPoolStrategy{})
	if got.Mode != config.ProxyPoolModeRandom {
		t.Fatalf("mode = %q", got.Mode)
	}
	if got.RequestsPerProxy != config.DefaultProxyPoolRequestsPerProxy {
		t.Fatalf("requests-per-proxy = %d", got.RequestsPerProxy)
	}
}

func TestExtractProxyFromRequestLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v1-responses-2026-05-28T100000-req1.log")
	content := "=== API REQUEST 1 ===\nProxy: socks5://redacted@127.0.0.1:1080\nHeaders:\n=== API REQUEST 2 ===\nProxy: socks5://redacted@127.0.0.2:1080\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write log fixture: %v", err)
	}

	got := extractProxyFromRequestLog(path)
	if got != "socks5://redacted@127.0.0.2:1080" {
		t.Fatalf("extractProxyFromRequestLog = %q", got)
	}
	proxies := extractProxiesFromRequestLog(path)
	if len(proxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(proxies))
	}
}

func TestRequestIDFromLogName(t *testing.T) {
	got := requestIDFromLogName("v1-responses-2026-05-28T100000-abc123.log")
	if got != "abc123" {
		t.Fatalf("requestIDFromLogName = %q", got)
	}
}

func TestIsRequestLogFileName(t *testing.T) {
	if !isRequestLogFileName("v1-responses-2026-05-28T100000-abc123.log") {
		t.Fatal("expected request log filename to be accepted")
	}
	if isRequestLogFileName("error-2026-05-28.log") {
		t.Fatal("expected error log filename to be rejected")
	}
	if isRequestLogFileName("main.log") {
		t.Fatal("expected main log filename to be rejected")
	}
}
