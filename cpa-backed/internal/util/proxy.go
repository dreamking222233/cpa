// Package util provides utility functions for the CLI Proxy API server.
// It includes helper functions for proxy configuration, HTTP client setup,
// log level management, and other common operations used across the application.
package util

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/proxyutil"
	log "github.com/sirupsen/logrus"
)

type proxyPoolRotationCursor struct {
	index int
	used  int
}

var proxyPoolRotationState = struct {
	mu      sync.Mutex
	cursors map[string]proxyPoolRotationCursor
}{
	cursors: make(map[string]proxyPoolRotationCursor),
}

// SetProxy configures the provided HTTP client with proxy settings from the configuration.
// It supports SOCKS5, HTTP, and HTTPS proxies. The function modifies the client's transport
// to route requests through the configured proxy server.
func SetProxy(cfg *config.SDKConfig, httpClient *http.Client) *http.Client {
	if cfg == nil || httpClient == nil {
		return httpClient
	}

	proxyURL := EffectiveProxyURL(cfg)
	transport, _, errBuild := proxyutil.BuildHTTPTransport(proxyURL)
	if errBuild != nil {
		log.Errorf("%v", errBuild)
	}
	if transport != nil {
		httpClient.Transport = transport
	}
	return httpClient
}

// EffectiveProxyURL returns the proxy URL to use for a standalone client setup.
// It prefers a randomly selected enabled proxy-pool entry and falls back to proxy-url.
func EffectiveProxyURL(cfg *config.SDKConfig) string {
	if cfg == nil {
		return ""
	}
	if proxyURL := PickProxyFromPool(cfg.ProxyPool, cfg.ProxyPoolStrategy); proxyURL != "" {
		return proxyURL
	}
	return strings.TrimSpace(cfg.ProxyURL)
}

// PickProxyFromPool returns one enabled proxy URL from the pool according to strategy.
func PickProxyFromPool(pool []config.ProxyPoolEntry, strategy config.ProxyPoolStrategy) string {
	enabled := enabledProxyPoolURLs(pool)
	if len(enabled) == 0 {
		return ""
	}
	if len(enabled) == 1 {
		return enabled[0]
	}

	mode := normalizeProxyPoolMode(strategy.Mode)
	if mode == config.ProxyPoolModeRotate {
		return pickProxyFromPoolRotate(enabled, normalizedRequestsPerProxy(strategy.RequestsPerProxy))
	}
	return pickProxyFromPoolRandom(enabled)
}

func enabledProxyPoolURLs(pool []config.ProxyPoolEntry) []string {
	enabled := make([]string, 0, len(pool))
	for i := range pool {
		entry := pool[i]
		proxyURL := strings.TrimSpace(entry.URL)
		if proxyURL == "" || entry.Disabled {
			continue
		}
		setting, errParse := proxyutil.Parse(proxyURL)
		if errParse != nil || setting.Mode != proxyutil.ModeProxy {
			continue
		}
		enabled = append(enabled, proxyURL)
	}
	return enabled
}

func pickProxyFromPoolRandom(enabled []string) string {
	idx, errRand := rand.Int(rand.Reader, big.NewInt(int64(len(enabled))))
	if errRand != nil {
		log.WithError(errRand).Warn("failed to select random proxy; using first enabled proxy")
		return enabled[0]
	}
	return enabled[idx.Int64()]
}

func pickProxyFromPoolRotate(enabled []string, requestsPerProxy int) string {
	key := proxyPoolRotationKey(enabled, requestsPerProxy)

	proxyPoolRotationState.mu.Lock()
	defer proxyPoolRotationState.mu.Unlock()

	cursor := proxyPoolRotationState.cursors[key]
	if cursor.index < 0 || cursor.index >= len(enabled) {
		cursor.index = 0
		cursor.used = 0
	}

	selected := enabled[cursor.index]
	cursor.used++
	if cursor.used >= requestsPerProxy {
		cursor.index = (cursor.index + 1) % len(enabled)
		cursor.used = 0
	}
	proxyPoolRotationState.cursors[key] = cursor

	return selected
}

func proxyPoolRotationKey(enabled []string, requestsPerProxy int) string {
	return strings.Join(enabled, "\n") + "|" + strconv.Itoa(requestsPerProxy)
}

func normalizeProxyPoolMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case config.ProxyPoolModeRotate:
		return config.ProxyPoolModeRotate
	case "", config.ProxyPoolModeRandom:
		return config.ProxyPoolModeRandom
	default:
		log.WithField("mode", mode).Warn("invalid proxy-pool mode; falling back to random")
		return config.ProxyPoolModeRandom
	}
}

func normalizedRequestsPerProxy(value int) int {
	if value > 0 {
		return value
	}
	return config.DefaultProxyPoolRequestsPerProxy
}

// DescribeEffectiveProxySelection returns the currently enabled pool and selection strategy summary.
func DescribeEffectiveProxySelection(cfg *config.SDKConfig) (urls []string, mode string, requestsPerProxy int) {
	if cfg == nil {
		return nil, config.ProxyPoolModeRandom, config.DefaultProxyPoolRequestsPerProxy
	}
	return enabledProxyPoolURLs(cfg.ProxyPool), normalizeProxyPoolMode(cfg.ProxyPoolStrategy.Mode), normalizedRequestsPerProxy(cfg.ProxyPoolStrategy.RequestsPerProxy)
}

// ResetProxyPoolRotationState clears in-memory round-robin counters. Tests use this helper.
func ResetProxyPoolRotationState() {
	proxyPoolRotationState.mu.Lock()
	defer proxyPoolRotationState.mu.Unlock()
	proxyPoolRotationState.cursors = make(map[string]proxyPoolRotationCursor)
}

// DebugProxyPoolRotationCursor returns rotation state for tests and diagnostics.
func DebugProxyPoolRotationCursor(pool []config.ProxyPoolEntry, strategy config.ProxyPoolStrategy) (string, error) {
	enabled := enabledProxyPoolURLs(pool)
	if len(enabled) == 0 {
		return "", fmt.Errorf("no enabled proxies")
	}
	key := proxyPoolRotationKey(enabled, normalizedRequestsPerProxy(strategy.RequestsPerProxy))
	proxyPoolRotationState.mu.Lock()
	defer proxyPoolRotationState.mu.Unlock()
	cursor := proxyPoolRotationState.cursors[key]
	return fmt.Sprintf("%d:%d", cursor.index, cursor.used), nil
}
