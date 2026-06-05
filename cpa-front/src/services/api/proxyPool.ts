import { apiClient } from './client';

export interface ProxyPoolEntry {
  name: string;
  url: string;
  disabled: boolean;
}

export interface ProxyPoolStrategy {
  mode: 'random' | 'rotate';
  requestsPerProxy: number;
}

export interface ProxyPoolRequestLogEntry {
  name: string;
  requestId: string;
  proxy: string;
  proxies: string[];
  modified: number;
  size: number;
}

export interface ProxyPoolTestResult {
  success: boolean;
  error: string;
  scheme: string;
  proxy: string;
  testUrl: string;
  ipCheckUrl: string;
  upstreamStatus: number;
  latencyMs: number;
  exitIp: string;
}

type ProxyPoolAction = 'add' | 'update' | 'delete';

type RawProxyPoolResponse = {
  'proxy-pool'?: unknown;
  'proxy-pool-strategy'?: unknown;
};

type RawProxyPoolRequestLogsResponse = {
  logs?: unknown;
};

type RawProxyPoolSettingsResponse = {
  'proxy-pool-strategy'?: unknown;
};

type RawProxyPoolTestResponse = Record<string, unknown>;

const toNumber = (value: unknown): number => {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (typeof value === 'string') {
    const parsed = Number(value.trim());
    if (Number.isFinite(parsed)) return parsed;
  }
  return 0;
};

const normalizeProxyPoolEntry = (value: unknown): ProxyPoolEntry | null => {
  if (!value || typeof value !== 'object') return null;
  const record = value as Record<string, unknown>;
  const url = String(record.url ?? '').trim();
  if (!url) return null;
  return {
    name: String(record.name ?? '').trim(),
    url,
    disabled: record.disabled === true,
  };
};

const normalizeProxyPoolStrategy = (value: unknown): ProxyPoolStrategy => {
  if (!value || typeof value !== 'object') {
    return { mode: 'random', requestsPerProxy: 100 };
  }
  const record = value as Record<string, unknown>;
  const mode = String(record.mode ?? '').trim().toLowerCase() === 'rotate' ? 'rotate' : 'random';
  const requestsPerProxy = Math.max(1, toNumber(record['requests-per-proxy'] ?? record.requestsPerProxy) || 100);
  return { mode, requestsPerProxy };
};

const normalizeProxyPoolRequestLogEntry = (value: unknown): ProxyPoolRequestLogEntry | null => {
  if (!value || typeof value !== 'object') return null;
  const record = value as Record<string, unknown>;
  const proxies = Array.isArray(record.proxies)
    ? record.proxies.map((item) => String(item ?? '').trim()).filter(Boolean)
    : [];

  return {
    name: String(record.name ?? '').trim(),
    requestId: String(record['request-id'] ?? record.request_id ?? '').trim(),
    proxy: String(record.proxy ?? '').trim(),
    proxies,
    modified: toNumber(record.modified),
    size: toNumber(record.size),
  };
};

const normalizeProxyPoolTestResult = (value: unknown): ProxyPoolTestResult => {
  const record = value && typeof value === 'object' ? (value as Record<string, unknown>) : {};
  return {
    success: record.success === true,
    error: String(record.error ?? '').trim(),
    scheme: String(record.scheme ?? '').trim(),
    proxy: String(record.proxy ?? '').trim(),
    testUrl: String(record['test-url'] ?? record.test_url ?? '').trim(),
    ipCheckUrl: String(record['ip-check-url'] ?? record.ip_check_url ?? '').trim(),
    upstreamStatus: toNumber(record['upstream-status'] ?? record.upstream_status),
    latencyMs: toNumber(record['latency-ms'] ?? record.latency_ms),
    exitIp: String(record['exit-ip'] ?? record.exit_ip ?? '').trim(),
  };
};

export interface ProxyPoolSnapshot {
  entries: ProxyPoolEntry[];
  strategy: ProxyPoolStrategy;
}

export const proxyPoolApi = {
  async list(): Promise<ProxyPoolSnapshot> {
    const data = await apiClient.get<RawProxyPoolResponse>('/proxy-pool');
    const entries = Array.isArray(data?.['proxy-pool'])
      ? data['proxy-pool']
      .map((item) => normalizeProxyPoolEntry(item))
      .filter((item): item is ProxyPoolEntry => item !== null)
      : [];
    return {
      entries,
      strategy: normalizeProxyPoolStrategy(data?.['proxy-pool-strategy']),
    };
  },

  patch(action: ProxyPoolAction, index?: number, value?: ProxyPoolEntry) {
    return apiClient.patch('/proxy-pool', {
      action,
      index,
      value,
    });
  },

  clear() {
    return apiClient.delete('/proxy-pool');
  },

  async getSettings(): Promise<ProxyPoolStrategy> {
    const data = await apiClient.get<RawProxyPoolSettingsResponse>('/proxy-pool/settings');
    return normalizeProxyPoolStrategy(data?.['proxy-pool-strategy']);
  },

  updateSettings(value: ProxyPoolStrategy) {
    return apiClient.put('/proxy-pool/settings', { value });
  },

  async test(url: string, testUrl?: string): Promise<ProxyPoolTestResult> {
    const data = await apiClient.post<RawProxyPoolTestResponse>('/proxy-pool/test', {
      url,
      'test-url': testUrl,
    });
    return normalizeProxyPoolTestResult(data);
  },

  async fetchRequestLogs(limit = 100): Promise<ProxyPoolRequestLogEntry[]> {
    const data = await apiClient.get<RawProxyPoolRequestLogsResponse>('/proxy-pool/request-logs', {
      params: { limit },
    });
    if (!Array.isArray(data?.logs)) return [];
    return data.logs
      .map((item) => normalizeProxyPoolRequestLogEntry(item))
      .filter((item): item is ProxyPoolRequestLogEntry => item !== null);
  },
};
