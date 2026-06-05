import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { EmptyState } from '@/components/ui/EmptyState';
import { Input } from '@/components/ui/Input';
import { Select, type SelectOption } from '@/components/ui/Select';
import { IconRefreshCw, IconSearch } from '@/components/ui/icons';
import { useHeaderRefresh } from '@/hooks/useHeaderRefresh';
import {
  proxyPoolApi,
  requestLogsApi,
  type OpenAIRequestLogEntry,
  type OpenAIRequestLogsSummary,
  type ProxyPoolRequestLogEntry,
} from '@/services/api';
import { useAuthStore } from '@/stores';
import styles from './RequestLogsPage.module.scss';

type RequestLogViewItem = OpenAIRequestLogEntry & {
  proxy: string;
  proxyChain: string;
};

const LIMIT_OPTIONS: SelectOption[] = [
  { value: '50', label: '50' },
  { value: '100', label: '100' },
  { value: '200', label: '200' },
  { value: '500', label: '500' },
];

const EMPTY_SUMMARY: OpenAIRequestLogsSummary = {
  logs: [],
  totals: {
    requests: 0,
    inputTokens: 0,
    outputTokens: 0,
    cachedTokens: 0,
    totalTokens: 0,
    costUsd: 0,
  },
  pricingSource: '',
  pricingChecked: '',
  timezone: 'Asia/Shanghai',
};

const formatCost = (value: number): string => `$${value.toFixed(6)}`;

const formatProxyFromLog = (item: ProxyPoolRequestLogEntry | undefined): string => {
  if (!item) return '';
  if (item.proxies.length > 1) return item.proxies.join(' -> ');
  if (item.proxies.length === 1) return item.proxies[0];
  return item.proxy;
};

const buildProxyIndex = (items: ProxyPoolRequestLogEntry[]): Map<string, ProxyPoolRequestLogEntry> => {
  const index = new Map<string, ProxyPoolRequestLogEntry>();
  items.forEach((item) => {
    if (!item.requestId) return;
    if (!index.has(item.requestId)) {
      index.set(item.requestId, item);
    }
  });
  return index;
};

export function RequestLogsPage() {
  const { t } = useTranslation();
  const connectionStatus = useAuthStore((state) => state.connectionStatus);

  const [limit, setLimit] = useState('100');
  const [search, setSearch] = useState('');
  const [summary, setSummary] = useState<OpenAIRequestLogsSummary>(EMPTY_SUMMARY);
  const [proxyLogs, setProxyLogs] = useState<ProxyPoolRequestLogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [proxyWarning, setProxyWarning] = useState('');

  const disableControls = connectionStatus !== 'connected';

  const loadData = useCallback(
    async (silent = false) => {
      if (disableControls) {
        setLoading(false);
        return;
      }

      if (silent) {
        setRefreshing(true);
      } else {
        setLoading(true);
      }
      setError('');
      setProxyWarning('');

      try {
        const numericLimit = Number(limit);
        const [requestSummaryResult, proxyRequestLogsResult] = await Promise.allSettled([
          requestLogsApi.list(numericLimit),
          proxyPoolApi.fetchRequestLogs(numericLimit),
        ]);

        if (requestSummaryResult.status !== 'fulfilled') {
          throw requestSummaryResult.reason;
        }

        setSummary(requestSummaryResult.value);

        if (proxyRequestLogsResult.status === 'fulfilled') {
          setProxyLogs(proxyRequestLogsResult.value);
        } else {
          setProxyLogs([]);
          setProxyWarning(
            t('request_logs.proxy_warning', {
              defaultValue: '代理出口记录加载失败，当前仍展示核心请求日志与 Token 汇总。',
            })
          );
        }
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : t('common.unknown_error');
        setError(message);
      } finally {
        setLoading(false);
        setRefreshing(false);
      }
    },
    [disableControls, limit, t]
  );

  useEffect(() => {
    void loadData();
  }, [loadData]);

  useHeaderRefresh(() => loadData(true));

  const proxyIndex = useMemo(() => buildProxyIndex(proxyLogs), [proxyLogs]);

  const logs = useMemo<RequestLogViewItem[]>(() => {
    return summary.logs.map((item) => {
      const proxyItem = proxyIndex.get(item.requestId);
      return {
        ...item,
        proxy: proxyItem?.proxy || '',
        proxyChain: formatProxyFromLog(proxyItem),
      };
    });
  }, [proxyIndex, summary.logs]);

  const filteredLogs = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return logs;
    return logs.filter((item) => {
      const haystack = [
        item.timeBeijing,
        item.model,
        item.authFile,
        item.authId,
        item.authLabel,
        item.authType,
        item.requestId,
        item.name,
        item.proxyChain,
        item.status,
      ]
        .join(' ')
        .toLowerCase();
      return haystack.includes(keyword);
    });
  }, [logs, search]);

  return (
    <div className={styles.container}>
      <div className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>
            {t('request_logs.title', { defaultValue: '请求日志' })}
          </h1>
          <p className={styles.description}>
            {t('request_logs.description', {
              defaultValue:
                '按北京时间查看每次 OpenAI 请求的模型、授权文件、token 用量、缓存命中、费用估算，以及对应的代理出口。',
            })}
          </p>
        </div>
        <div className={styles.headerActions}>
          <div className={styles.limitField}>
            <span className={styles.limitLabel}>
              {t('request_logs.limit', { defaultValue: '条数' })}
            </span>
            <Select
              value={limit}
              options={LIMIT_OPTIONS}
              onChange={setLimit}
              ariaLabel={t('request_logs.limit', { defaultValue: '条数' })}
              fullWidth={false}
            />
          </div>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void loadData(true)}
            disabled={disableControls}
            loading={refreshing}
          >
            <span className={styles.buttonContent}>
              <IconRefreshCw size={16} />
              {t('common.refresh')}
            </span>
          </Button>
        </div>
      </div>

      {error ? <div className={styles.errorBox}>{error}</div> : null}
      {proxyWarning ? <div className={styles.warningBox}>{proxyWarning}</div> : null}

      <div className={styles.summaryGrid}>
        <Card className={styles.summaryCard}>
          <div className={styles.metricLabel}>
            {t('request_logs.metrics.requests', { defaultValue: '请求数' })}
          </div>
          <div className={styles.metricValue}>{summary.totals.requests}</div>
        </Card>
        <Card className={styles.summaryCard}>
          <div className={styles.metricLabel}>
            {t('request_logs.metrics.input', { defaultValue: '输入 Token' })}
          </div>
          <div className={styles.metricValue}>{summary.totals.inputTokens.toLocaleString()}</div>
        </Card>
        <Card className={styles.summaryCard}>
          <div className={styles.metricLabel}>
            {t('request_logs.metrics.output', { defaultValue: '输出 Token' })}
          </div>
          <div className={styles.metricValue}>{summary.totals.outputTokens.toLocaleString()}</div>
        </Card>
        <Card className={styles.summaryCard}>
          <div className={styles.metricLabel}>
            {t('request_logs.metrics.cached', { defaultValue: '缓存 Token' })}
          </div>
          <div className={styles.metricValue}>{summary.totals.cachedTokens.toLocaleString()}</div>
        </Card>
        <Card className={styles.summaryCard}>
          <div className={styles.metricLabel}>
            {t('request_logs.metrics.cost', { defaultValue: '估算费用' })}
          </div>
          <div className={styles.metricValue}>{formatCost(summary.totals.costUsd)}</div>
        </Card>
      </div>

      <Card
        className={styles.tableCard}
        title={t('request_logs.table_title', { defaultValue: '请求明细' })}
        extra={
          <div className={styles.searchBox}>
            <Input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t('request_logs.search_placeholder', {
                defaultValue: '搜索模型、授权文件、代理或请求 ID',
              })}
              rightElement={<IconSearch size={16} className={styles.searchIcon} />}
            />
          </div>
        }
      >
        <div className={styles.pricingNote}>
          {t('request_logs.pricing_note', {
            defaultValue:
              '价格按 OpenAI 官方 API Pricing 估算。检查日期：{{date}}，时区：{{timezone}}。',
            date: summary.pricingChecked || '-',
            timezone: summary.timezone || 'Asia/Shanghai',
          })}
          {summary.pricingSource ? (
            <>
              {' '}
              <a href={summary.pricingSource} target="_blank" rel="noreferrer">
                {t('request_logs.pricing_source', { defaultValue: '价格来源' })}
              </a>
            </>
          ) : null}
        </div>

        {loading ? (
          <div className={styles.placeholderText}>{t('common.loading')}</div>
        ) : filteredLogs.length === 0 ? (
          <EmptyState
            title={t('request_logs.empty_title', { defaultValue: '还没有请求日志' })}
            description={t('request_logs.empty_desc', {
              defaultValue: '确认后端已开启 request-log，并且已经有 OpenAI 请求通过系统后再回来查看。',
            })}
          />
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>{t('request_logs.columns.time', { defaultValue: '北京时间' })}</th>
                  <th>{t('request_logs.columns.model', { defaultValue: '模型' })}</th>
                  <th>{t('request_logs.columns.auth_file', { defaultValue: '授权文件' })}</th>
                  <th>{t('request_logs.columns.proxy', { defaultValue: '代理出口' })}</th>
                  <th className={styles.numberCell}>{t('request_logs.columns.input', { defaultValue: '输入' })}</th>
                  <th className={styles.numberCell}>{t('request_logs.columns.output', { defaultValue: '输出' })}</th>
                  <th className={styles.numberCell}>{t('request_logs.columns.cached', { defaultValue: '缓存' })}</th>
                  <th className={styles.numberCell}>{t('request_logs.columns.reasoning', { defaultValue: '推理' })}</th>
                  <th className={styles.numberCell}>{t('request_logs.columns.total', { defaultValue: '合计' })}</th>
                  <th className={styles.numberCell}>{t('request_logs.columns.cost', { defaultValue: '费用' })}</th>
                  <th>{t('request_logs.columns.status', { defaultValue: '状态' })}</th>
                  <th>{t('request_logs.columns.request_id', { defaultValue: '请求 ID' })}</th>
                  <th>{t('request_logs.columns.log_file', { defaultValue: '日志文件' })}</th>
                </tr>
              </thead>
              <tbody>
                {filteredLogs.map((item) => (
                  <tr key={`${item.name}-${item.requestId}`}>
                    <td>{item.timeBeijing || '-'}</td>
                    <td className={styles.mono}>{item.model || '-'}</td>
                    <td>
                      <div className={styles.cellStack}>
                        <span className={styles.mono}>{item.authFile || item.authId || '-'}</span>
                        {item.authLabel ? <span className={styles.subtle}>{item.authLabel}</span> : null}
                      </div>
                    </td>
                    <td className={styles.mono}>{item.proxyChain || '-'}</td>
                    <td className={styles.numberCell}>{item.inputTokens.toLocaleString()}</td>
                    <td className={styles.numberCell}>{item.outputTokens.toLocaleString()}</td>
                    <td className={styles.numberCell}>{item.cachedTokens.toLocaleString()}</td>
                    <td className={styles.numberCell}>{item.reasoningTokens.toLocaleString()}</td>
                    <td className={styles.numberCell}>{item.totalTokens.toLocaleString()}</td>
                    <td className={styles.numberCell}>
                      {item.priceAvailable ? formatCost(item.costUsd) : t('request_logs.unmatched_price', { defaultValue: '未匹配' })}
                    </td>
                    <td>{item.status || '-'}</td>
                    <td className={styles.mono}>{item.requestId || '-'}</td>
                    <td>
                      <div className={styles.cellStack}>
                        <span className={styles.mono}>{item.name || '-'}</span>
                        {item.truncated ? (
                          <span className={styles.subtle}>
                            {t('request_logs.truncated', { defaultValue: '日志已截断读取' })}
                          </span>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}
