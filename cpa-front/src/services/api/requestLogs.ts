import { apiClient } from './client';

export interface OpenAIRequestLogEntry {
  name: string;
  requestId: string;
  time: string;
  timeUnix: number;
  timeBeijing: string;
  model: string;
  authFile: string;
  authId: string;
  authLabel: string;
  authType: string;
  status: number;
  inputTokens: number;
  outputTokens: number;
  cachedTokens: number;
  reasoningTokens: number;
  totalTokens: number;
  costUsd: number;
  priceAvailable: boolean;
  pricingModel: string;
  pricingNote: string;
  modified: number;
  size: number;
  truncated: boolean;
}

export interface OpenAIRequestLogTotals {
  requests: number;
  inputTokens: number;
  outputTokens: number;
  cachedTokens: number;
  totalTokens: number;
  costUsd: number;
}

export interface OpenAIRequestLogsSummary {
  logs: OpenAIRequestLogEntry[];
  totals: OpenAIRequestLogTotals;
  pricingSource: string;
  pricingChecked: string;
  timezone: string;
}

type RawOpenAIRequestLogsSummary = {
  logs?: unknown;
  totals?: Record<string, unknown>;
  pricing_source?: unknown;
  pricing_checked?: unknown;
  timezone?: unknown;
};

const toNumber = (value: unknown): number => {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (typeof value === 'string') {
    const parsed = Number(value.trim());
    if (Number.isFinite(parsed)) return parsed;
  }
  return 0;
};

const normalizeOpenAIRequestLogEntry = (value: unknown): OpenAIRequestLogEntry | null => {
  if (!value || typeof value !== 'object') return null;
  const record = value as Record<string, unknown>;
  return {
    name: String(record.name ?? '').trim(),
    requestId: String(record.request_id ?? '').trim(),
    time: String(record.time ?? '').trim(),
    timeUnix: toNumber(record.time_unix),
    timeBeijing: String(record.time_beijing ?? '').trim(),
    model: String(record.model ?? '').trim(),
    authFile: String(record.auth_file ?? '').trim(),
    authId: String(record.auth_id ?? '').trim(),
    authLabel: String(record.auth_label ?? '').trim(),
    authType: String(record.auth_type ?? '').trim(),
    status: toNumber(record.status),
    inputTokens: toNumber(record.input_tokens),
    outputTokens: toNumber(record.output_tokens),
    cachedTokens: toNumber(record.cached_tokens),
    reasoningTokens: toNumber(record.reasoning_tokens),
    totalTokens: toNumber(record.total_tokens),
    costUsd: toNumber(record.cost_usd),
    priceAvailable: record.price_available === true,
    pricingModel: String(record.pricing_model ?? '').trim(),
    pricingNote: String(record.pricing_note ?? '').trim(),
    modified: toNumber(record.modified),
    size: toNumber(record.size),
    truncated: record.truncated === true,
  };
};

const normalizeTotals = (value: Record<string, unknown> | undefined): OpenAIRequestLogTotals => ({
  requests: toNumber(value?.requests),
  inputTokens: toNumber(value?.input_tokens),
  outputTokens: toNumber(value?.output_tokens),
  cachedTokens: toNumber(value?.cached_tokens),
  totalTokens: toNumber(value?.total_tokens),
  costUsd: toNumber(value?.cost_usd),
});

export const requestLogsApi = {
  async list(limit = 100): Promise<OpenAIRequestLogsSummary> {
    const data = await apiClient.get<RawOpenAIRequestLogsSummary>('/openai-request-logs', {
      params: { limit },
    });
    const logs = Array.isArray(data?.logs)
      ? data.logs
          .map((item) => normalizeOpenAIRequestLogEntry(item))
          .filter((item): item is OpenAIRequestLogEntry => item !== null)
      : [];

    return {
      logs,
      totals: normalizeTotals(data?.totals),
      pricingSource: String(data?.pricing_source ?? '').trim(),
      pricingChecked: String(data?.pricing_checked ?? '').trim(),
      timezone: String(data?.timezone ?? '').trim(),
    };
  },
};
