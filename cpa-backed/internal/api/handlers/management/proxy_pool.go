package management

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/proxyutil"
)

const defaultProxyPoolLogLimit = 100
const defaultProxyPoolTestURL = "https://api.openai.com/v1/models"
const defaultProxyPoolIPCheckURL = "https://api.ipify.org?format=json"

type proxyPoolPayload struct {
	Value []config.ProxyPoolEntry `json:"value"`
}

type proxyPoolSettingsPayload struct {
	Value config.ProxyPoolStrategy `json:"value"`
}

type proxyPoolTestPayload struct {
	URL        string `json:"url"`
	TestURL    string `json:"test-url,omitempty"`
	IPCheckURL string `json:"ip-check-url,omitempty"`
}

type proxyPoolTestResult struct {
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
	Scheme         string `json:"scheme,omitempty"`
	Proxy          string `json:"proxy"`
	TestURL        string `json:"test-url"`
	IPCheckURL     string `json:"ip-check-url,omitempty"`
	UpstreamStatus int    `json:"upstream-status,omitempty"`
	LatencyMS      int64  `json:"latency-ms,omitempty"`
	ExitIP         string `json:"exit-ip,omitempty"`
}

// GetProxyPool returns configured outbound proxy pool entries.
func (h *Handler) GetProxyPool(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "configuration unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"proxy-pool":          h.cfg.ProxyPool,
		"proxy-pool-strategy": h.cfg.ProxyPoolStrategy,
	})
}

// GetProxyPoolSettings returns proxy-pool selection settings.
func (h *Handler) GetProxyPoolSettings(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "configuration unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"proxy-pool-strategy": h.cfg.ProxyPoolStrategy})
}

// PutProxyPoolSettings replaces proxy-pool selection settings.
func (h *Handler) PutProxyPoolSettings(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "configuration unavailable"})
		return
	}

	var body proxyPoolSettingsPayload
	if errBindJSON := c.ShouldBindJSON(&body); errBindJSON != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	h.cfg.ProxyPoolStrategy = normalizeProxyPoolStrategy(body.Value)
	h.persist(c)
}

// PutProxyPool replaces the full outbound proxy pool.
func (h *Handler) PutProxyPool(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "configuration unavailable"})
		return
	}

	var body proxyPoolPayload
	if errBindJSON := c.ShouldBindJSON(&body); errBindJSON != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	entries, errNormalize := normalizeProxyPoolEntries(body.Value)
	if errNormalize != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errNormalize.Error()})
		return
	}

	h.cfg.ProxyPool = entries
	h.persist(c)
}

// PatchProxyPool applies add/update/delete operations by array index.
func (h *Handler) PatchProxyPool(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "configuration unavailable"})
		return
	}

	var body struct {
		Action string                `json:"action"`
		Index  *int                  `json:"index"`
		Value  config.ProxyPoolEntry `json:"value"`
	}
	if errBindJSON := c.ShouldBindJSON(&body); errBindJSON != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	action := strings.ToLower(strings.TrimSpace(body.Action))
	switch action {
	case "add":
		entry, errNormalize := normalizeProxyPoolEntry(body.Value)
		if errNormalize != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": errNormalize.Error()})
			return
		}
		h.cfg.ProxyPool = append(h.cfg.ProxyPool, entry)
	case "update":
		if body.Index == nil || *body.Index < 0 || *body.Index >= len(h.cfg.ProxyPool) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid index"})
			return
		}
		entry, errNormalize := normalizeProxyPoolEntry(body.Value)
		if errNormalize != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": errNormalize.Error()})
			return
		}
		h.cfg.ProxyPool[*body.Index] = entry
	case "delete":
		if body.Index == nil || *body.Index < 0 || *body.Index >= len(h.cfg.ProxyPool) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid index"})
			return
		}
		h.cfg.ProxyPool = append(h.cfg.ProxyPool[:*body.Index], h.cfg.ProxyPool[*body.Index+1:]...)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
		return
	}

	h.persist(c)
}

// DeleteProxyPool clears the outbound proxy pool.
func (h *Handler) DeleteProxyPool(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "configuration unavailable"})
		return
	}
	h.cfg.ProxyPool = nil
	h.persist(c)
}

// TestProxyPoolEntry tests whether a concrete proxy URL can reach the upstream target.
func (h *Handler) TestProxyPoolEntry(c *gin.Context) {
	var body proxyPoolTestPayload
	if errBindJSON := c.ShouldBindJSON(&body); errBindJSON != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	entry, errNormalize := normalizeProxyPoolEntry(config.ProxyPoolEntry{URL: body.URL})
	if errNormalize != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errNormalize.Error()})
		return
	}

	result := runProxyPoolConnectivityTest(c.Request.Context(), entry.URL, body.TestURL, body.IPCheckURL)
	c.JSON(http.StatusOK, result)
}

type proxyPoolRequestLogEntry struct {
	Name      string   `json:"name"`
	RequestID string   `json:"request-id"`
	Proxy     string   `json:"proxy"`
	Proxies   []string `json:"proxies"`
	Modified  int64    `json:"modified"`
	Size      int64    `json:"size"`
}

// GetProxyPoolRequestLogs returns recent request log files with extracted proxy metadata.
func (h *Handler) GetProxyPoolRequestLogs(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "configuration unavailable"})
		return
	}

	dir := h.logDirectory()
	if strings.TrimSpace(dir) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "log directory not configured"})
		return
	}

	limit := parseProxyPoolLogLimit(c.Query("limit"))
	entries, errRead := os.ReadDir(dir)
	if errRead != nil {
		if os.IsNotExist(errRead) {
			c.JSON(http.StatusOK, gin.H{"logs": []proxyPoolRequestLogEntry{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to list log directory: %v", errRead)})
		return
	}

	candidates := make([]proxyPoolRequestLogEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isRequestLogFileName(name) {
			continue
		}
		info, errInfo := entry.Info()
		if errInfo != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to read log info for %s: %v", name, errInfo)})
			return
		}
		candidates = append(candidates, proxyPoolRequestLogEntry{
			Name:      name,
			RequestID: requestIDFromLogName(name),
			Modified:  info.ModTime().Unix(),
			Size:      info.Size(),
		})
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Modified > candidates[j].Modified })
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	for i := range candidates {
		proxies := extractProxiesFromRequestLog(filepath.Join(dir, candidates[i].Name))
		candidates[i].Proxies = proxies
		if len(proxies) > 0 {
			candidates[i].Proxy = proxies[len(proxies)-1]
		}
	}

	c.JSON(http.StatusOK, gin.H{"logs": candidates})
}

func normalizeProxyPoolEntries(entries []config.ProxyPoolEntry) ([]config.ProxyPoolEntry, error) {
	out := make([]config.ProxyPoolEntry, 0, len(entries))
	for i := range entries {
		entry, errNormalize := normalizeProxyPoolEntry(entries[i])
		if errNormalize != nil {
			return nil, errNormalize
		}
		out = append(out, entry)
	}
	return out, nil
}

func normalizeProxyPoolEntry(entry config.ProxyPoolEntry) (config.ProxyPoolEntry, error) {
	entry.Name = strings.TrimSpace(entry.Name)
	entry.URL = strings.TrimSpace(entry.URL)
	if entry.URL == "" {
		return config.ProxyPoolEntry{}, fmt.Errorf("proxy URL is required")
	}
	setting, errParse := proxyutil.Parse(entry.URL)
	if errParse != nil {
		return config.ProxyPoolEntry{}, fmt.Errorf("invalid proxy URL: %v", errParse)
	}
	if setting.Mode != proxyutil.ModeProxy {
		return config.ProxyPoolEntry{}, fmt.Errorf("proxy pool entries must be concrete proxy URLs")
	}
	return entry, nil
}

func normalizeProxyPoolStrategy(strategy config.ProxyPoolStrategy) config.ProxyPoolStrategy {
	strategy.Mode = strings.ToLower(strings.TrimSpace(strategy.Mode))
	switch strategy.Mode {
	case "", config.ProxyPoolModeRandom:
		strategy.Mode = config.ProxyPoolModeRandom
	case config.ProxyPoolModeRotate:
		// keep as-is
	default:
		strategy.Mode = config.ProxyPoolModeRandom
	}
	if strategy.RequestsPerProxy <= 0 {
		strategy.RequestsPerProxy = config.DefaultProxyPoolRequestsPerProxy
	}
	return strategy
}

func parseProxyPoolLogLimit(raw string) int {
	limit := defaultProxyPoolLogLimit
	if parsed, errParse := strconv.Atoi(strings.TrimSpace(raw)); errParse == nil && parsed > 0 {
		limit = parsed
	}
	if limit > 500 {
		limit = 500
	}
	return limit
}

func requestIDFromLogName(name string) string {
	trimmed := strings.TrimSuffix(name, ".log")
	idx := strings.LastIndex(trimmed, "-")
	if idx == -1 || idx == len(trimmed)-1 {
		return ""
	}
	return trimmed[idx+1:]
}

func extractProxyFromRequestLog(path string) string {
	proxies := extractProxiesFromRequestLog(path)
	if len(proxies) == 0 {
		return ""
	}
	return proxies[len(proxies)-1]
}

func extractProxiesFromRequestLog(path string) []string {
	file, errOpen := os.Open(path)
	if errOpen != nil {
		return nil
	}
	defer func() { _ = file.Close() }()

	var proxies []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, logScannerInitialBuffer), logScannerMaxBuffer)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Proxy:") {
			proxy := strings.TrimSpace(strings.TrimPrefix(line, "Proxy:"))
			if proxy != "" {
				proxies = append(proxies, proxy)
			}
		}
	}
	return proxies
}

func runProxyPoolConnectivityTest(ctx context.Context, proxyURL string, testURL string, ipCheckURL string) proxyPoolTestResult {
	testURL = normalizeProxyPoolTestURL(testURL)
	ipCheckURL = normalizeProxyPoolIPCheckURL(ipCheckURL)

	result := proxyPoolTestResult{
		Proxy:      proxyURL,
		TestURL:    testURL,
		IPCheckURL: ipCheckURL,
	}

	transport, _, errBuild := proxyutil.BuildHTTPTransport(proxyURL)
	if errBuild != nil {
		result.Error = errBuild.Error()
		return result
	}
	client := &http.Client{Transport: transport}

	req, errRequest := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
	if errRequest != nil {
		result.Error = errRequest.Error()
		return result
	}
	req.Header.Set("User-Agent", "CLIProxyAPI-ProxyPool-Test/1.0")

	start := time.Now()
	resp, errDo := client.Do(req)
	if errDo != nil {
		result.Error = errDo.Error()
		return result
	}
	result.LatencyMS = time.Since(start).Milliseconds()
	result.UpstreamStatus = resp.StatusCode
	if resp.Request != nil && resp.Request.URL != nil {
		result.Scheme = strings.ToLower(strings.TrimSpace(resp.Request.URL.Scheme))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	result.Success = isSuccessfulProxyPoolTestStatus(resp.StatusCode)
	if !result.Success {
		result.Error = fmt.Sprintf("upstream returned status %d", resp.StatusCode)
		return result
	}

	if exitIP := fetchProxyExitIP(ctx, client, ipCheckURL); exitIP != "" {
		result.ExitIP = exitIP
	}

	return result
}

func normalizeProxyPoolTestURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultProxyPoolTestURL
	}
	return raw
}

func normalizeProxyPoolIPCheckURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultProxyPoolIPCheckURL
	}
	return raw
}

func isSuccessfulProxyPoolTestStatus(status int) bool {
	if status == http.StatusProxyAuthRequired {
		return false
	}
	return status >= 200 && status < 500
}

func fetchProxyExitIP(ctx context.Context, client *http.Client, ipCheckURL string) string {
	req, errRequest := http.NewRequestWithContext(ctx, http.MethodGet, ipCheckURL, nil)
	if errRequest != nil {
		return ""
	}
	resp, errDo := client.Do(req)
	if errDo != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	body, errRead := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if errRead != nil || len(body) == 0 {
		return ""
	}

	var payload struct {
		IP string `json:"ip"`
	}
	if errUnmarshal := json.Unmarshal(body, &payload); errUnmarshal == nil {
		return strings.TrimSpace(payload.IP)
	}

	return strings.TrimSpace(string(body))
}
