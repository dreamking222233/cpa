package management

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	defaultOpenAIRequestLogLimit = 100
	maxOpenAIRequestLogLimit     = 500
	maxRequestLogSummaryBytes    = 8 * 1024 * 1024
	requestLogSummaryHeadBytes   = 256 * 1024
)

var beijingLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type openAIRequestLogEntry struct {
	Name              string             `json:"name"`
	RequestID         string             `json:"request_id"`
	Time              string             `json:"time"`
	TimeUnix          int64              `json:"time_unix"`
	TimeBeijing       string             `json:"time_beijing"`
	Model             string             `json:"model"`
	AuthFile          string             `json:"auth_file"`
	AuthID            string             `json:"auth_id"`
	AuthLabel         string             `json:"auth_label"`
	AuthType          string             `json:"auth_type"`
	Status            int                `json:"status"`
	InputTokens       int64              `json:"input_tokens"`
	OutputTokens      int64              `json:"output_tokens"`
	CachedTokens      int64              `json:"cached_tokens"`
	ReasoningTokens   int64              `json:"reasoning_tokens"`
	TotalTokens       int64              `json:"total_tokens"`
	CostUSD           float64            `json:"cost_usd"`
	PriceAvailable    bool               `json:"price_available"`
	PricingModel      string             `json:"pricing_model"`
	PricingNote       string             `json:"pricing_note,omitempty"`
	PricingPerMTokens map[string]float64 `json:"pricing_per_m_tokens,omitempty"`
	Modified          int64              `json:"modified"`
	Size              int64              `json:"size"`
	Truncated         bool               `json:"truncated"`
}

type openAIRequestLogTotals struct {
	Requests     int     `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CachedTokens int64   `json:"cached_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

type openAIModelPrice struct {
	Model                       string
	InputPerMillion             float64
	CachedInputPerMillion       float64
	OutputPerMillion            float64
	LongContextInputThreshold   int64
	LongContextInputMultiplier  float64
	LongContextOutputMultiplier float64
}

// Prices are USD per 1M tokens from OpenAI API pricing, checked 2026-05-29.
// Source: https://openai.com/api/pricing/
var openAIModelPrices = map[string]openAIModelPrice{
	"gpt-5.5":             {Model: "gpt-5.5", InputPerMillion: 5.00, CachedInputPerMillion: 0.50, OutputPerMillion: 30.00, LongContextInputThreshold: 270000, LongContextInputMultiplier: 2, LongContextOutputMultiplier: 1.5},
	"gpt-5.5-pro":         {Model: "gpt-5.5-pro", InputPerMillion: 30.00, CachedInputPerMillion: 30.00, OutputPerMillion: 180.00, LongContextInputThreshold: 270000, LongContextInputMultiplier: 2, LongContextOutputMultiplier: 1.5},
	"gpt-5.5-chat-latest": {Model: "gpt-5.5", InputPerMillion: 5.00, CachedInputPerMillion: 0.50, OutputPerMillion: 30.00, LongContextInputThreshold: 270000, LongContextInputMultiplier: 2, LongContextOutputMultiplier: 1.5},
	"chat-latest":         {Model: "chat-latest", InputPerMillion: 5.00, CachedInputPerMillion: 0.50, OutputPerMillion: 30.00},
	"gpt-5.4":             {Model: "gpt-5.4", InputPerMillion: 2.50, CachedInputPerMillion: 0.25, OutputPerMillion: 15.00, LongContextInputThreshold: 270000, LongContextInputMultiplier: 2, LongContextOutputMultiplier: 1.5},
	"gpt-5.4-pro":         {Model: "gpt-5.4-pro", InputPerMillion: 30.00, CachedInputPerMillion: 30.00, OutputPerMillion: 180.00, LongContextInputThreshold: 270000, LongContextInputMultiplier: 2, LongContextOutputMultiplier: 1.5},
	"gpt-5.4-mini":        {Model: "gpt-5.4-mini", InputPerMillion: 0.75, CachedInputPerMillion: 0.075, OutputPerMillion: 4.50},
	"gpt-5.4-nano":        {Model: "gpt-5.4-nano", InputPerMillion: 0.20, CachedInputPerMillion: 0.02, OutputPerMillion: 1.25},
	"gpt-5":               {Model: "gpt-5", InputPerMillion: 1.25, CachedInputPerMillion: 0.125, OutputPerMillion: 10.00},
	"gpt-5-mini":          {Model: "gpt-5-mini", InputPerMillion: 0.25, CachedInputPerMillion: 0.025, OutputPerMillion: 2.00},
	"gpt-5-nano":          {Model: "gpt-5-nano", InputPerMillion: 0.05, CachedInputPerMillion: 0.005, OutputPerMillion: 0.40},
	"gpt-5.3-codex":       {Model: "gpt-5.3-codex", InputPerMillion: 1.75, CachedInputPerMillion: 0.175, OutputPerMillion: 14.00},
	"gpt-5-codex":         {Model: "gpt-5-codex", InputPerMillion: 1.25, CachedInputPerMillion: 0.125, OutputPerMillion: 10.00},
	"gpt-4.1":             {Model: "gpt-4.1", InputPerMillion: 2.00, CachedInputPerMillion: 0.50, OutputPerMillion: 8.00},
	"gpt-4.1-mini":        {Model: "gpt-4.1-mini", InputPerMillion: 0.40, CachedInputPerMillion: 0.10, OutputPerMillion: 1.60},
	"gpt-4.1-nano":        {Model: "gpt-4.1-nano", InputPerMillion: 0.10, CachedInputPerMillion: 0.025, OutputPerMillion: 0.40},
	"gpt-4o":              {Model: "gpt-4o", InputPerMillion: 2.50, CachedInputPerMillion: 1.25, OutputPerMillion: 10.00},
	"gpt-4o-mini":         {Model: "gpt-4o-mini", InputPerMillion: 0.15, CachedInputPerMillion: 0.075, OutputPerMillion: 0.60},
	"o3":                  {Model: "o3", InputPerMillion: 2.00, CachedInputPerMillion: 0.50, OutputPerMillion: 8.00},
	"o4-mini":             {Model: "o4-mini", InputPerMillion: 1.10, CachedInputPerMillion: 0.275, OutputPerMillion: 4.40},
}

// GetOpenAIRequestLogs returns summarized OpenAI request logs for the management page.
func (h *Handler) GetOpenAIRequestLogs(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "configuration unavailable"})
		return
	}

	dir := h.logDirectory()
	if strings.TrimSpace(dir) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "log directory not configured"})
		return
	}

	limit := parseOpenAIRequestLogLimit(c.Query("limit"))
	entries, errRead := os.ReadDir(dir)
	if errRead != nil {
		if os.IsNotExist(errRead) {
			c.JSON(http.StatusOK, openAIRequestLogResponse(nil))
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to list log directory: %v", errRead)})
		return
	}

	candidates := collectOpenAIRequestLogCandidates(dir, entries)
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Modified > candidates[j].Modified })
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	logs := make([]openAIRequestLogEntry, 0, len(candidates))
	for _, candidate := range candidates {
		entry := parseOpenAIRequestLogFile(filepath.Join(dir, candidate.Name), candidate)
		logs = append(logs, entry)
	}

	c.JSON(http.StatusOK, openAIRequestLogResponse(logs))
}

func openAIRequestLogResponse(logs []openAIRequestLogEntry) gin.H {
	if logs == nil {
		logs = []openAIRequestLogEntry{}
	}
	totals := openAIRequestLogTotals{Requests: len(logs)}
	for _, entry := range logs {
		totals.InputTokens += entry.InputTokens
		totals.OutputTokens += entry.OutputTokens
		totals.CachedTokens += entry.CachedTokens
		totals.TotalTokens += entry.TotalTokens
		totals.CostUSD += entry.CostUSD
	}
	totals.CostUSD = roundCost(totals.CostUSD)
	return gin.H{
		"logs":            logs,
		"totals":          totals,
		"pricing_source":  "https://openai.com/api/pricing/",
		"pricing_checked": "2026-05-29",
		"timezone":        "Asia/Shanghai",
	}
}

func parseOpenAIRequestLogLimit(raw string) int {
	limit := defaultOpenAIRequestLogLimit
	if parsed, errParse := strconv.Atoi(strings.TrimSpace(raw)); errParse == nil && parsed > 0 {
		limit = parsed
	}
	if limit > maxOpenAIRequestLogLimit {
		limit = maxOpenAIRequestLogLimit
	}
	return limit
}

func collectOpenAIRequestLogCandidates(dir string, entries []os.DirEntry) []openAIRequestLogEntry {
	candidates := make([]openAIRequestLogEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isRequestLogFileName(name) {
			continue
		}
		info, errInfo := entry.Info()
		if errInfo != nil || info.IsDir() {
			continue
		}
		candidates = append(candidates, openAIRequestLogEntry{
			Name:      name,
			RequestID: requestIDFromLogName(name),
			Modified:  info.ModTime().Unix(),
			Size:      info.Size(),
		})
	}
	return candidates
}

func parseOpenAIRequestLogFile(path string, base openAIRequestLogEntry) openAIRequestLogEntry {
	entry := base
	content, truncated := readRequestLogSummaryContent(path, base.Size)
	entry.Truncated = truncated
	if len(content) == 0 {
		applyOpenAICost(&entry)
		return entry
	}
	parseOpenAIRequestLogText(content, &entry)
	if entry.RequestID == "" {
		entry.RequestID = requestIDFromLogName(entry.Name)
	}
	if entry.TimeUnix == 0 && entry.Modified > 0 {
		setOpenAIRequestLogTime(&entry, time.Unix(entry.Modified, 0))
	}
	if entry.TotalTokens == 0 {
		entry.TotalTokens = entry.InputTokens + entry.OutputTokens
	}
	if entry.AuthFile == "" {
		entry.AuthFile = entry.AuthID
	}
	applyOpenAICost(&entry)
	return entry
}

func readRequestLogSummaryContent(path string, size int64) ([]byte, bool) {
	if size <= maxRequestLogSummaryBytes {
		data, errRead := os.ReadFile(path)
		if errRead != nil {
			return nil, false
		}
		return data, false
	}

	file, errOpen := os.Open(path)
	if errOpen != nil {
		return nil, false
	}
	defer func() { _ = file.Close() }()

	head := make([]byte, requestLogSummaryHeadBytes)
	nHead, _ := io.ReadFull(file, head)
	tailSize := maxRequestLogSummaryBytes - requestLogSummaryHeadBytes
	if _, errSeek := file.Seek(size-int64(tailSize), io.SeekStart); errSeek != nil {
		return head[:nHead], true
	}
	tail, _ := io.ReadAll(io.LimitReader(file, int64(tailSize)))
	out := make([]byte, 0, nHead+len(tail)+32)
	out = append(out, head[:nHead]...)
	out = append(out, []byte("\n=== LOG SUMMARY TRUNCATED ===\n")...)
	out = append(out, tail...)
	return out, true
}

func parseOpenAIRequestLogText(content []byte, entry *openAIRequestLogEntry) {
	if entry == nil {
		return
	}
	seenUsage := map[string]struct{}{}
	lines := strings.Split(string(content), "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(strings.TrimRight(rawLine, "\r"))
		if line == "" {
			continue
		}
		parseOpenAIRequestLogLine(line, entry)
		if strings.HasPrefix(line, "data:") {
			parseOpenAIJSONCandidate(strings.TrimSpace(strings.TrimPrefix(line, "data:")), entry, seenUsage)
			continue
		}
		if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[") {
			parseOpenAIJSONCandidate(line, entry, seenUsage)
		}
	}
	for _, body := range extractRequestLogBodyBlocks(string(content)) {
		parseOpenAIJSONCandidate(body, entry, seenUsage)
	}
}

func parseOpenAIRequestLogLine(line string, entry *openAIRequestLogEntry) {
	switch {
	case strings.HasPrefix(line, "Timestamp:"):
		if entry.TimeUnix == 0 {
			if parsed, ok := parseLogTimestamp(strings.TrimSpace(strings.TrimPrefix(line, "Timestamp:"))); ok {
				setOpenAIRequestLogTime(entry, parsed)
			}
		}
	case strings.HasPrefix(line, "Status:"):
		if parsed, errParse := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Status:"))); errParse == nil {
			entry.Status = parsed
		}
	case strings.HasPrefix(line, "Auth:"):
		parseOpenAIRequestAuthLine(strings.TrimSpace(strings.TrimPrefix(line, "Auth:")), entry)
	}
}

func parseLogTimestamp(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	if parsed, errParse := time.Parse(time.RFC3339Nano, raw); errParse == nil {
		return parsed, true
	}
	if parsed, errParse := time.ParseInLocation("2006-01-02 15:04:05", raw, time.Local); errParse == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func setOpenAIRequestLogTime(entry *openAIRequestLogEntry, value time.Time) {
	if entry == nil || value.IsZero() {
		return
	}
	beijing := value.In(beijingLocation)
	entry.TimeUnix = beijing.Unix()
	entry.Time = beijing.Format(time.RFC3339)
	entry.TimeBeijing = beijing.Format("2006-01-02 15:04:05")
}

func parseOpenAIRequestAuthLine(raw string, entry *openAIRequestLogEntry) {
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "auth_id":
			entry.AuthID = value
			entry.AuthFile = value
		case "label":
			entry.AuthLabel = value
		case "type":
			if value := firstField(value); value != "" {
				entry.AuthType = value
			}
		}
	}
}

func firstField(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func extractRequestLogBodyBlocks(content string) []string {
	var blocks []string
	markers := []string{"\nBody:\n", "\n=== REQUEST BODY ===\n"}
	for _, marker := range markers {
		start := 0
		for {
			idx := strings.Index(content[start:], marker)
			if idx == -1 {
				break
			}
			bodyStart := start + idx + len(marker)
			bodyEnd := strings.Index(content[bodyStart:], "\n===")
			if bodyEnd == -1 {
				bodyEnd = len(content)
			} else {
				bodyEnd += bodyStart
			}
			block := strings.TrimSpace(content[bodyStart:bodyEnd])
			if block != "" && block != "<empty>" {
				blocks = append(blocks, block)
			}
			start = bodyStart
		}
	}
	return blocks
}

func parseOpenAIJSONCandidate(raw string, entry *openAIRequestLogEntry, seenUsage map[string]struct{}) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[DONE]" || !json.Valid([]byte(raw)) {
		return
	}
	root := gjson.Parse(raw)
	if entry.Model == "" {
		entry.Model = firstGJSONString(root, "model", "request.model", "response.model")
	}
	for _, path := range []string{"usage", "response.usage"} {
		usageNode := root.Get(path)
		if !usageNode.Exists() || !usageNode.IsObject() {
			continue
		}
		usageRaw := usageNode.Raw
		if usageRaw == "" {
			continue
		}
		if _, exists := seenUsage[usageRaw]; exists {
			continue
		}
		seenUsage[usageRaw] = struct{}{}
		addOpenAIUsage(entry, usageNode)
	}
}

func firstGJSONString(root gjson.Result, paths ...string) string {
	for _, path := range paths {
		value := strings.TrimSpace(root.Get(path).String())
		if value != "" {
			return value
		}
	}
	return ""
}

func addOpenAIUsage(entry *openAIRequestLogEntry, usageNode gjson.Result) {
	if entry == nil || !usageNode.Exists() {
		return
	}
	input := firstGJSONInt(usageNode, "prompt_tokens", "input_tokens")
	output := firstGJSONInt(usageNode, "completion_tokens", "output_tokens")
	cached := firstGJSONInt(usageNode, "prompt_tokens_details.cached_tokens", "input_tokens_details.cached_tokens", "cache_read_input_tokens")
	reasoning := firstGJSONInt(usageNode, "completion_tokens_details.reasoning_tokens", "output_tokens_details.reasoning_tokens")
	total := usageNode.Get("total_tokens").Int()
	if total == 0 {
		total = input + output
	}
	entry.InputTokens += input
	entry.OutputTokens += output
	entry.CachedTokens += cached
	entry.ReasoningTokens += reasoning
	entry.TotalTokens += total
}

func firstGJSONInt(root gjson.Result, paths ...string) int64 {
	for _, path := range paths {
		value := root.Get(path)
		if value.Exists() {
			return value.Int()
		}
	}
	return 0
}

func applyOpenAICost(entry *openAIRequestLogEntry) {
	if entry == nil {
		return
	}
	price, ok := lookupOpenAIModelPrice(entry.Model)
	if !ok {
		entry.PriceAvailable = false
		return
	}
	cached := entry.CachedTokens
	if cached < 0 {
		cached = 0
	}
	if cached > entry.InputTokens {
		cached = entry.InputTokens
	}
	nonCachedInput := entry.InputTokens - cached
	inputPrice := price.InputPerMillion
	cachedInputPrice := price.CachedInputPerMillion
	outputPrice := price.OutputPerMillion
	if price.LongContextInputThreshold > 0 && entry.InputTokens > price.LongContextInputThreshold {
		inputMultiplier := price.LongContextInputMultiplier
		if inputMultiplier == 0 {
			inputMultiplier = 1
		}
		outputMultiplier := price.LongContextOutputMultiplier
		if outputMultiplier == 0 {
			outputMultiplier = 1
		}
		inputPrice *= inputMultiplier
		cachedInputPrice *= inputMultiplier
		outputPrice *= outputMultiplier
		entry.PricingNote = fmt.Sprintf("input tokens > %d; long-context pricing applied", price.LongContextInputThreshold)
	}
	cost := (float64(nonCachedInput)*inputPrice +
		float64(cached)*cachedInputPrice +
		float64(entry.OutputTokens)*outputPrice) / 1_000_000
	entry.CostUSD = roundCost(cost)
	entry.PriceAvailable = true
	entry.PricingModel = price.Model
	entry.PricingPerMTokens = map[string]float64{
		"input":        inputPrice,
		"cached_input": cachedInputPrice,
		"output":       outputPrice,
	}
}

func lookupOpenAIModelPrice(model string) (openAIModelPrice, bool) {
	key := normalizeOpenAIPriceModel(model)
	if key == "" {
		return openAIModelPrice{}, false
	}
	if price, ok := openAIModelPrices[key]; ok {
		return price, true
	}
	var matched openAIModelPrice
	var matchedLen int
	for candidate, price := range openAIModelPrices {
		if strings.HasPrefix(key, candidate+"-") {
			if len(candidate) > matchedLen {
				matched = price
				matchedLen = len(candidate)
			}
		}
	}
	if matchedLen > 0 {
		return matched, true
	}
	return openAIModelPrice{}, false
}

func normalizeOpenAIPriceModel(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	model = strings.TrimPrefix(model, "openai/")
	model = strings.TrimPrefix(model, "chatgpt/")
	return model
}

func roundCost(value float64) float64 {
	if value == 0 {
		return 0
	}
	return math.Round(value*1_000_000) / 1_000_000
}
