package util

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

func TestEffectiveProxyURLPrefersEnabledProxyPool(t *testing.T) {
	ResetProxyPoolRotationState()
	cfg := &config.SDKConfig{
		ProxyURL: "http://global.example.com:8080",
		ProxyPool: []config.ProxyPoolEntry{
			{URL: "http://disabled.example.com:8080", Disabled: true},
			{URL: "socks5://pool.example.com:1080"},
		},
	}

	got := EffectiveProxyURL(cfg)
	if got != "socks5://pool.example.com:1080" {
		t.Fatalf("EffectiveProxyURL = %q, want pool proxy", got)
	}
}

func TestEffectiveProxyURLFallsBackToProxyURL(t *testing.T) {
	ResetProxyPoolRotationState()
	cfg := &config.SDKConfig{
		ProxyURL: "http://global.example.com:8080",
		ProxyPool: []config.ProxyPoolEntry{
			{URL: "http://disabled.example.com:8080", Disabled: true},
		},
	}

	got := EffectiveProxyURL(cfg)
	if got != "http://global.example.com:8080" {
		t.Fatalf("EffectiveProxyURL = %q, want global proxy", got)
	}
}

func TestPickProxyFromPoolIgnoresEmptyAndDisabledEntries(t *testing.T) {
	got := PickProxyFromPool([]config.ProxyPoolEntry{
		{URL: " "},
		{URL: "direct"},
		{URL: "http://disabled.example.com:8080", Disabled: true},
		{URL: "http://enabled.example.com:8080"},
	}, config.ProxyPoolStrategy{})
	if got != "http://enabled.example.com:8080" {
		t.Fatalf("PickProxyFromPool = %q, want enabled proxy", got)
	}
}

func TestPickProxyFromPoolRotateRespectsRequestsPerProxy(t *testing.T) {
	ResetProxyPoolRotationState()
	pool := []config.ProxyPoolEntry{
		{URL: "http://proxy-a.example.com:8080"},
		{URL: "http://proxy-b.example.com:8080"},
	}
	strategy := config.ProxyPoolStrategy{
		Mode:             config.ProxyPoolModeRotate,
		RequestsPerProxy: 2,
	}

	sequence := []string{
		PickProxyFromPool(pool, strategy),
		PickProxyFromPool(pool, strategy),
		PickProxyFromPool(pool, strategy),
		PickProxyFromPool(pool, strategy),
		PickProxyFromPool(pool, strategy),
	}
	want := []string{
		"http://proxy-a.example.com:8080",
		"http://proxy-a.example.com:8080",
		"http://proxy-b.example.com:8080",
		"http://proxy-b.example.com:8080",
		"http://proxy-a.example.com:8080",
	}

	for i := range want {
		if sequence[i] != want[i] {
			t.Fatalf("sequence[%d] = %q, want %q", i, sequence[i], want[i])
		}
	}
}

func TestSanitizeProxyPoolDropsInvalidEntries(t *testing.T) {
	cfg := &config.Config{
		SDKConfig: config.SDKConfig{
			ProxyPool: []config.ProxyPoolEntry{
				{URL: "127.0.0.1:1080"},
				{URL: "none"},
				{URL: "socks5://valid.example.com:1080"},
			},
		},
	}

	cfg.SanitizeProxyPool()
	if len(cfg.ProxyPool) != 1 {
		t.Fatalf("ProxyPool length = %d, want 1", len(cfg.ProxyPool))
	}
	if cfg.ProxyPool[0].URL != "socks5://valid.example.com:1080" {
		t.Fatalf("remaining proxy = %q", cfg.ProxyPool[0].URL)
	}
}

func TestSanitizeProxyPoolStrategyDefaults(t *testing.T) {
	cfg := &config.Config{}

	cfg.SanitizeProxyPoolStrategy()

	if cfg.ProxyPoolStrategy.Mode != config.ProxyPoolModeRandom {
		t.Fatalf("mode = %q", cfg.ProxyPoolStrategy.Mode)
	}
	if cfg.ProxyPoolStrategy.RequestsPerProxy != config.DefaultProxyPoolRequestsPerProxy {
		t.Fatalf("requests-per-proxy = %d", cfg.ProxyPoolStrategy.RequestsPerProxy)
	}
}
