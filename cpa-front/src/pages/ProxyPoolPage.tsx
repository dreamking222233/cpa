import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { EmptyState } from '@/components/ui/EmptyState';
import { Input } from '@/components/ui/Input';
import { Select } from '@/components/ui/Select';
import { ToggleSwitch } from '@/components/ui/ToggleSwitch';
import { IconPlus, IconRefreshCw, IconSearch, IconShield, IconTrash2 } from '@/components/ui/icons';
import { useHeaderRefresh } from '@/hooks/useHeaderRefresh';
import {
  proxyPoolApi,
  type ProxyPoolEntry,
  type ProxyPoolRequestLogEntry,
  type ProxyPoolStrategy,
  type ProxyPoolTestResult,
} from '@/services/api';
import { useAuthStore, useNotificationStore } from '@/stores';
import { formatFileSize } from '@/utils/format';
import styles from './ProxyPoolPage.module.scss';

const REQUEST_LOG_LIMIT = 100;
const DEFAULT_REQUESTS_PER_PROXY = 100;
const ALLOWED_PROTOCOLS = new Set(['socks5:', 'socks5h:', 'http:', 'https:']);
const MODE_OPTIONS = [
  { value: 'random', label: '随机使用' },
  { value: 'rotate', label: '顺序轮换' },
];

type ProxyTestState = {
  loading: boolean;
  result: ProxyPoolTestResult | null;
  error: string;
};

type BatchProxyParseResult = {
  entries: ProxyPoolEntry[];
  invalidValues: string[];
  duplicateCount: number;
  existingCount: number;
};

const parseProxyProtocol = (value: string): string => {
  try {
    const parsed = new URL(value.trim());
    return ALLOWED_PROTOCOLS.has(parsed.protocol) ? parsed.protocol.replace(':', '') : '';
  } catch {
    return '';
  }
};

const isSupportedProxyUrl = (value: string): boolean => Boolean(parseProxyProtocol(value));

const formatBeijingDateTime = (unixSeconds: number): string => {
  if (!unixSeconds) return '';
  const date = new Date(unixSeconds * 1000);
  if (Number.isNaN(date.getTime())) return '';
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(date);
};

const formatProxyChain = (item: ProxyPoolRequestLogEntry): string => {
  if (item.proxies.length > 1) return item.proxies.join(' -> ');
  if (item.proxies.length === 1) return item.proxies[0];
  return item.proxy || '';
};

const cloneEntry = (entry: ProxyPoolEntry): ProxyPoolEntry => ({
  name: entry.name,
  url: entry.url,
  disabled: entry.disabled,
});

const cloneStrategy = (strategy: ProxyPoolStrategy): ProxyPoolStrategy => ({
  mode: strategy.mode,
  requestsPerProxy: strategy.requestsPerProxy,
});

const tokenizeBatchProxyInput = (value: string): string[] =>
  value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean);

const parseBatchProxyInput = (
  value: string,
  existingEntries: ProxyPoolEntry[],
  disabled: boolean
): BatchProxyParseResult => {
  const existingUrls = new Set(existingEntries.map((entry) => entry.url.trim()).filter(Boolean));
  const seenUrls = new Set<string>();
  const entries: ProxyPoolEntry[] = [];
  const invalidValues: string[] = [];
  let duplicateCount = 0;
  let existingCount = 0;

  for (const token of tokenizeBatchProxyInput(value)) {
    if (!isSupportedProxyUrl(token)) {
      invalidValues.push(token);
      continue;
    }
    if (existingUrls.has(token)) {
      existingCount += 1;
      continue;
    }
    if (seenUrls.has(token)) {
      duplicateCount += 1;
      continue;
    }

    seenUrls.add(token);
    entries.push({
      name: `批量代理 ${existingEntries.length + entries.length + 1}`,
      url: token,
      disabled,
    });
  }

  return { entries, invalidValues, duplicateCount, existingCount };
};

const strategiesEqual = (left: ProxyPoolStrategy, right: ProxyPoolStrategy): boolean =>
  left.mode === right.mode && left.requestsPerProxy === right.requestsPerProxy;

const proxyEntriesEqual = (left: ProxyPoolEntry, right: ProxyPoolEntry): boolean =>
  left.name === right.name && left.url === right.url && left.disabled === right.disabled;

const protocolLabel = (value: string): string => {
  const scheme = parseProxyProtocol(value);
  return scheme ? scheme.toUpperCase() : 'UNKNOWN';
};

const normalizeRequestsPerProxyInput = (value: string): number => {
  const parsed = Number(value.trim());
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return DEFAULT_REQUESTS_PER_PROXY;
  }
  return Math.max(1, Math.round(parsed));
};

const testKeyForIndex = (index: number) => `existing-${index}`;
const newEntryTestKey = 'new-entry';

export function ProxyPoolPage() {
  const { t } = useTranslation();
  const { showNotification, showConfirmation } = useNotificationStore();
  const connectionStatus = useAuthStore((state) => state.connectionStatus);

  const [entries, setEntries] = useState<ProxyPoolEntry[]>([]);
  const [drafts, setDrafts] = useState<ProxyPoolEntry[]>([]);
  const [strategy, setStrategy] = useState<ProxyPoolStrategy>({
    mode: 'random',
    requestsPerProxy: DEFAULT_REQUESTS_PER_PROXY,
  });
  const [savedStrategy, setSavedStrategy] = useState<ProxyPoolStrategy>({
    mode: 'random',
    requestsPerProxy: DEFAULT_REQUESTS_PER_PROXY,
  });
  const [requestLogs, setRequestLogs] = useState<ProxyPoolRequestLogEntry[]>([]);
  const [newEntry, setNewEntry] = useState<ProxyPoolEntry>({ name: '', url: '', disabled: false });
  const [batchInput, setBatchInput] = useState('');
  const [batchDisabled, setBatchDisabled] = useState(false);
  const [search, setSearch] = useState('');
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [adding, setAdding] = useState(false);
  const [batchAdding, setBatchAdding] = useState(false);
  const [savingIndex, setSavingIndex] = useState<number | null>(null);
  const [deletingIndex, setDeletingIndex] = useState<number | null>(null);
  const [savingStrategy, setSavingStrategy] = useState(false);
  const [error, setError] = useState('');
  const [logsWarning, setLogsWarning] = useState('');
  const [testStates, setTestStates] = useState<Record<string, ProxyTestState>>({});

  const disableControls = connectionStatus !== 'connected';
  const strategyDirty = !strategiesEqual(strategy, savedStrategy);

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
      setLogsWarning('');

      try {
        const [proxySnapshotResult, logsResult] = await Promise.allSettled([
          proxyPoolApi.list(),
          proxyPoolApi.fetchRequestLogs(REQUEST_LOG_LIMIT),
        ]);

        if (proxySnapshotResult.status !== 'fulfilled') {
          throw proxySnapshotResult.reason;
        }

        setEntries(proxySnapshotResult.value.entries);
        setDrafts(proxySnapshotResult.value.entries.map(cloneEntry));
        setStrategy(cloneStrategy(proxySnapshotResult.value.strategy));
        setSavedStrategy(cloneStrategy(proxySnapshotResult.value.strategy));
        setTestStates({});

        if (logsResult.status === 'fulfilled') {
          setRequestLogs(logsResult.value);
        } else {
          setRequestLogs([]);
          setLogsWarning(
            t('proxy_pool.logs_warning', {
              defaultValue: '最近代理使用记录加载失败，但代理池配置仍可正常管理。',
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
    [disableControls, t]
  );

  useEffect(() => {
    void loadData();
  }, [loadData]);

  useHeaderRefresh(() => loadData(true));

  const filteredLogs = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return requestLogs;
    return requestLogs.filter((item) => {
      const haystack = [item.name, item.requestId, item.proxy, ...item.proxies].join(' ').toLowerCase();
      return haystack.includes(keyword);
    });
  }, [requestLogs, search]);

  const activeCount = entries.filter((item) => !item.disabled).length;
  const disabledCount = entries.length - activeCount;
  const proxyEntriesDirty = useMemo(
    () =>
      drafts.length !== entries.length ||
      drafts.some((entry, index) => {
        const original = entries[index];
        return !original || !proxyEntriesEqual(original, entry);
      }),
    [drafts, entries]
  );
  const pageDraftDirty = proxyEntriesDirty || strategyDirty;
  const batchPreview = useMemo(
    () => parseBatchProxyInput(batchInput, entries, batchDisabled),
    [batchDisabled, batchInput, entries]
  );

  const setTestState = useCallback((key: string, next: ProxyTestState) => {
    setTestStates((prev) => ({ ...prev, [key]: next }));
  }, []);

  const validateProxyUrl = useCallback(
    (value: string): boolean => {
      if (!value.trim()) {
        showNotification(
          t('proxy_pool.notifications.url_required', {
            defaultValue: '代理地址不能为空。',
          }),
          'error'
        );
        return false;
      }
      if (!isSupportedProxyUrl(value)) {
        showNotification(
          t('proxy_pool.notifications.protocol_required', {
            defaultValue: '仅支持 socks5、socks5h、http、https 代理地址。',
          }),
          'error'
        );
        return false;
      }
      return true;
    },
    [showNotification, t]
  );

  const handleDraftChange = useCallback((index: number, patch: Partial<ProxyPoolEntry>) => {
    setDrafts((prev) =>
      prev.map((item, itemIndex) =>
        itemIndex === index
          ? {
              ...item,
              ...patch,
            }
          : item
      )
    );
  }, []);

  const runProxyTest = useCallback(
    async (key: string, url: string) => {
      if (!validateProxyUrl(url)) {
        return;
      }

      setTestState(key, {
        loading: true,
        result: null,
        error: '',
      });

      try {
        const result = await proxyPoolApi.test(url.trim());
        setTestState(key, {
          loading: false,
          result,
          error: result.success ? '' : result.error || t('common.unknown_error'),
        });
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : t('common.unknown_error');
        setTestState(key, {
          loading: false,
          result: null,
          error: message,
        });
      }
    },
    [setTestState, t, validateProxyUrl]
  );

  const handleSave = useCallback(
    async (index: number) => {
      const draft = drafts[index];
      if (!draft || !validateProxyUrl(draft.url)) {
        return;
      }

      setSavingIndex(index);
      try {
        await proxyPoolApi.patch('update', index, {
          name: draft.name.trim(),
          url: draft.url.trim(),
          disabled: draft.disabled,
        });
        showNotification(
          t('proxy_pool.notifications.saved', {
            defaultValue: '代理池已更新。',
          }),
          'success'
        );
        await loadData(true);
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : t('common.unknown_error');
        showNotification(message, 'error');
      } finally {
        setSavingIndex(null);
      }
    },
    [drafts, loadData, showNotification, t, validateProxyUrl]
  );

  const handleDelete = useCallback(
    (index: number) => {
      showConfirmation({
        title: t('proxy_pool.delete_title', {
          defaultValue: '删除代理',
        }),
        message: t('proxy_pool.delete_confirm', {
          defaultValue: '确定要删除这个代理配置吗？',
        }),
        variant: 'danger',
        confirmText: t('common.confirm'),
        onConfirm: async () => {
          setDeletingIndex(index);
          try {
            await proxyPoolApi.patch('delete', index);
            showNotification(
              t('proxy_pool.notifications.deleted', {
                defaultValue: '代理配置已删除。',
              }),
              'success'
            );
            await loadData(true);
          } catch (err: unknown) {
            const message = err instanceof Error ? err.message : t('common.unknown_error');
            showNotification(message, 'error');
          } finally {
            setDeletingIndex(null);
          }
        },
      });
    },
    [loadData, showConfirmation, showNotification, t]
  );

  const handleAdd = useCallback(async () => {
    if (!validateProxyUrl(newEntry.url)) {
      return;
    }

    setAdding(true);
    try {
      await proxyPoolApi.patch('add', undefined, {
        name: newEntry.name.trim(),
        url: newEntry.url.trim(),
        disabled: newEntry.disabled,
      });
      setNewEntry({ name: '', url: '', disabled: false });
      setTestStates((prev) => {
        const next = { ...prev };
        delete next[newEntryTestKey];
        return next;
      });
      showNotification(
        t('proxy_pool.notifications.added', {
          defaultValue: '代理已加入代理池。',
        }),
        'success'
      );
      await loadData(true);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : t('common.unknown_error');
      showNotification(message, 'error');
    } finally {
      setAdding(false);
    }
  }, [loadData, newEntry, showNotification, t, validateProxyUrl]);

  const handleBatchAdd = useCallback(async () => {
    if (!batchInput.trim()) {
      showNotification(
        t('proxy_pool.notifications.batch_required', {
          defaultValue: '请先粘贴至少一个代理地址。',
        }),
        'error'
      );
      return;
    }

    if (batchPreview.entries.length === 0) {
      showNotification(
        t('proxy_pool.notifications.batch_no_valid', {
          defaultValue: '没有可添加的有效代理地址。',
        }),
        'error'
      );
      return;
    }

    if (pageDraftDirty) {
      showNotification(
        t('proxy_pool.notifications.batch_dirty_entries', {
          defaultValue: '请先保存或重置当前页面中的未保存修改，再批量添加。',
        }),
        'error'
      );
      return;
    }

    setBatchAdding(true);
    try {
      const result = await proxyPoolApi.batchAdd(batchPreview.entries);
      setBatchInput('');
      showNotification(
        t('proxy_pool.notifications.batch_added', {
          count: result.added,
          defaultValue: `已批量添加 ${result.added} 个代理。`,
        }),
        'success'
      );
      await loadData(true);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : t('common.unknown_error');
      showNotification(message, 'error');
      await loadData(true);
    } finally {
      setBatchAdding(false);
    }
  }, [batchInput, batchPreview.entries, loadData, pageDraftDirty, showNotification, t]);

  const handleSaveStrategy = useCallback(async () => {
    setSavingStrategy(true);
    try {
      const nextStrategy: ProxyPoolStrategy = {
        mode: strategy.mode,
        requestsPerProxy: normalizeRequestsPerProxyInput(String(strategy.requestsPerProxy)),
      };
      await proxyPoolApi.updateSettings(nextStrategy);
      setStrategy(cloneStrategy(nextStrategy));
      setSavedStrategy(cloneStrategy(nextStrategy));
      showNotification(
        t('proxy_pool.notifications.strategy_saved', {
          defaultValue: '代理调度策略已更新。',
        }),
        'success'
      );
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : t('common.unknown_error');
      showNotification(message, 'error');
    } finally {
      setSavingStrategy(false);
    }
  }, [showNotification, strategy, t]);

  const renderTestState = (state?: ProxyTestState) => {
    if (!state) return null;
    if (state.loading) {
      return (
        <div className={styles.testInfo}>
          {t('proxy_pool.test.testing', { defaultValue: '正在测试代理连通性...' })}
        </div>
      );
    }
    if (state.error) {
      return <div className={styles.testError}>{state.error}</div>;
    }
    if (!state.result) return null;

    return (
      <div className={styles.testResult}>
        <span className={`${styles.testBadge} ${state.result.success ? styles.testSuccess : styles.testFailure}`}>
          {state.result.success
            ? t('proxy_pool.test.available', { defaultValue: '可用' })
            : t('proxy_pool.test.unavailable', { defaultValue: '不可用' })}
        </span>
        <span>
          {t('proxy_pool.test.status', { defaultValue: '上游状态' })}: {state.result.upstreamStatus || '-'}
        </span>
        <span>
          {t('proxy_pool.test.latency', { defaultValue: '延迟' })}:{' '}
          {state.result.latencyMs ? `${state.result.latencyMs} ms` : '-'}
        </span>
        <span>
          {t('proxy_pool.test.exit_ip', { defaultValue: '出口 IP' })}: {state.result.exitIp || '-'}
        </span>
      </div>
    );
  };

  return (
    <div className={styles.container}>
      <div className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>
            {t('proxy_pool.title', { defaultValue: '代理池' })}
          </h1>
          <p className={styles.description}>
            {t('proxy_pool.description', {
              defaultValue:
                '维护多个上游代理，支持 socks5、socks5h、http、https，并可配置随机或按请求次数轮换使用。',
            })}
          </p>
        </div>
        <div className={styles.headerActions}>
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
      {logsWarning ? <div className={styles.warningBox}>{logsWarning}</div> : null}

      <div className={styles.statsGrid}>
        <Card className={styles.statCard}>
          <div className={styles.statLabel}>
            {t('proxy_pool.stats.total', { defaultValue: '代理总数' })}
          </div>
          <div className={styles.statValue}>{entries.length}</div>
        </Card>
        <Card className={styles.statCard}>
          <div className={styles.statLabel}>
            {t('proxy_pool.stats.active', { defaultValue: '可用代理' })}
          </div>
          <div className={styles.statValue}>{activeCount}</div>
        </Card>
        <Card className={styles.statCard}>
          <div className={styles.statLabel}>
            {t('proxy_pool.stats.disabled', { defaultValue: '已停用' })}
          </div>
          <div className={styles.statValue}>{disabledCount}</div>
        </Card>
      </div>

      <Card className={styles.card} title={t('proxy_pool.strategy_title', { defaultValue: '调度策略' })}>
        <div className={styles.cardDescription}>
          {t('proxy_pool.strategy_hint', {
            defaultValue: '随机模式每次请求随机挑选可用代理。顺序轮换模式会让同一代理连续处理指定次数后再切到下一个。',
          })}
        </div>

        <div className={styles.strategyGrid}>
          <div className={styles.fieldBlock}>
            <div className={styles.fieldLabel}>
              {t('proxy_pool.strategy.mode', { defaultValue: '使用模式' })}
            </div>
            <Select
              value={strategy.mode}
              options={MODE_OPTIONS}
              onChange={(value) =>
                setStrategy((prev) => ({
                  ...prev,
                  mode: value === 'rotate' ? 'rotate' : 'random',
                }))
              }
              disabled={disableControls}
            />
          </div>

          <Input
            label={t('proxy_pool.strategy.requests_per_proxy', { defaultValue: '单代理连续请求次数' })}
            type="number"
            min={1}
            value={String(strategy.requestsPerProxy)}
            onChange={(event) =>
              setStrategy((prev) => ({
                ...prev,
                requestsPerProxy: normalizeRequestsPerProxyInput(event.target.value),
              }))
            }
            disabled={disableControls || strategy.mode !== 'rotate'}
            hint={t('proxy_pool.strategy.requests_per_proxy_hint', {
              defaultValue: '仅在顺序轮换模式生效，例如设置为 100 表示同一代理连续处理 100 次后切换。',
            })}
          />
        </div>

        <div className={styles.rowActions}>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => setStrategy(cloneStrategy(savedStrategy))}
            disabled={!strategyDirty || disableControls}
          >
            {t('proxy_pool.actions.reset', { defaultValue: '重置' })}
          </Button>
          <Button
            size="sm"
            onClick={() => void handleSaveStrategy()}
            loading={savingStrategy}
            disabled={!strategyDirty || disableControls}
          >
            {t('common.save')}
          </Button>
        </div>
      </Card>

      <Card className={styles.card} title={t('proxy_pool.configured_title', { defaultValue: '当前代理列表' })}>
        <div className={styles.cardDescription}>
          {t('proxy_pool.configured_hint', {
            defaultValue: '支持 socks5、socks5h、http、https。可以先测试代理，再决定是否加入调度。',
          })}
        </div>

        {loading ? (
          <div className={styles.placeholderText}>{t('common.loading')}</div>
        ) : entries.length === 0 ? (
          <EmptyState
            title={t('proxy_pool.empty_title', { defaultValue: '还没有代理配置' })}
            description={t('proxy_pool.empty_desc', {
              defaultValue: '先添加至少一个代理，后端才会按你设置的策略为请求选择出口。',
            })}
          />
        ) : (
          <div className={styles.list}>
            {drafts.map((entry, index) => {
              const original = entries[index];
              const dirty =
                original?.name !== entry.name ||
                original?.url !== entry.url ||
                original?.disabled !== entry.disabled;
              const testState = testStates[testKeyForIndex(index)];

              return (
                <div className={styles.listRow} key={`${original?.url ?? 'proxy'}-${index}`}>
                  <div className={styles.rowHeader}>
                    <div className={styles.rowTitleGroup}>
                      <div className={styles.rowTitle}>
                        {entry.name.trim() || t('proxy_pool.unnamed', { defaultValue: `代理 ${index + 1}` })}
                      </div>
                      <span className={styles.protocolTag}>{protocolLabel(entry.url)}</span>
                    </div>
                    <span className={`${styles.statusBadge} ${entry.disabled ? styles.statusDisabled : styles.statusActive}`}>
                      {entry.disabled
                        ? t('proxy_pool.disabled', { defaultValue: '已停用' })
                        : t('proxy_pool.enabled', { defaultValue: '已启用' })}
                    </span>
                  </div>
                  <div className={styles.formGrid}>
                    <Input
                      label={t('proxy_pool.fields.name', { defaultValue: '名称' })}
                      value={entry.name}
                      onChange={(event) => handleDraftChange(index, { name: event.target.value })}
                      placeholder={t('proxy_pool.fields.name_placeholder', {
                        defaultValue: '可选，例如 HK-01',
                      })}
                      disabled={disableControls}
                    />
                    <Input
                      label={t('proxy_pool.fields.url', { defaultValue: '代理地址' })}
                      value={entry.url}
                      onChange={(event) => handleDraftChange(index, { url: event.target.value })}
                      placeholder="socks5://user:pass@host:1080"
                      hint={t('proxy_pool.fields.url_hint', {
                        defaultValue: '支持 socks5://、socks5h://、http://、https:// 代理地址。',
                      })}
                      disabled={disableControls}
                    />
                    <div className={styles.toggleGroup}>
                      <div className={styles.toggleLabel}>
                        {t('proxy_pool.fields.enabled', { defaultValue: '参与调度' })}
                      </div>
                      <ToggleSwitch
                        checked={!entry.disabled}
                        onChange={(checked) => handleDraftChange(index, { disabled: !checked })}
                        ariaLabel={t('proxy_pool.fields.enabled', { defaultValue: '参与调度' })}
                        disabled={disableControls}
                      />
                    </div>
                  </div>
                  {renderTestState(testState)}
                  <div className={styles.rowActions}>
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => void runProxyTest(testKeyForIndex(index), entry.url)}
                      loading={testState?.loading === true}
                      disabled={disableControls}
                    >
                      <span className={styles.buttonContent}>
                        <IconShield size={16} />
                        {t('proxy_pool.actions.test', { defaultValue: '测试代理' })}
                      </span>
                    </Button>
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() =>
                        setDrafts((prev) =>
                          prev.map((item, itemIndex) =>
                            itemIndex === index && original ? cloneEntry(original) : item
                          )
                        )
                      }
                      disabled={!dirty || disableControls}
                    >
                      {t('proxy_pool.actions.reset', { defaultValue: '重置' })}
                    </Button>
                    <Button
                      size="sm"
                      onClick={() => void handleSave(index)}
                      loading={savingIndex === index}
                      disabled={!dirty || disableControls}
                    >
                      {t('common.save')}
                    </Button>
                    <Button
                      size="sm"
                      variant="danger"
                      onClick={() => handleDelete(index)}
                      loading={deletingIndex === index}
                      disabled={disableControls}
                    >
                      <span className={styles.buttonContent}>
                        <IconTrash2 size={16} />
                        {t('common.delete')}
                      </span>
                    </Button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </Card>

      <Card className={styles.card} title={t('proxy_pool.add_title', { defaultValue: '新增代理' })}>
        <div className={styles.formGrid}>
          <Input
            label={t('proxy_pool.fields.name', { defaultValue: '名称' })}
            value={newEntry.name}
            onChange={(event) => setNewEntry((prev) => ({ ...prev, name: event.target.value }))}
            placeholder={t('proxy_pool.fields.name_placeholder', {
              defaultValue: '可选，例如 HK-01',
            })}
            disabled={disableControls}
          />
          <Input
            label={t('proxy_pool.fields.url', { defaultValue: '代理地址' })}
            value={newEntry.url}
            onChange={(event) => setNewEntry((prev) => ({ ...prev, url: event.target.value }))}
            placeholder="socks5://user:pass@host:1080"
            hint={t('proxy_pool.fields.url_hint', {
              defaultValue: '支持 socks5://、socks5h://、http://、https:// 代理地址。',
            })}
            disabled={disableControls}
          />
          <div className={styles.toggleGroup}>
            <div className={styles.toggleLabel}>
              {t('proxy_pool.fields.enabled', { defaultValue: '参与调度' })}
            </div>
            <ToggleSwitch
              checked={!newEntry.disabled}
              onChange={(checked) => setNewEntry((prev) => ({ ...prev, disabled: !checked }))}
              ariaLabel={t('proxy_pool.fields.enabled', { defaultValue: '参与调度' })}
              disabled={disableControls}
            />
          </div>
        </div>
        {renderTestState(testStates[newEntryTestKey])}
        <div className={styles.rowActions}>
          <Button
            variant="secondary"
            onClick={() => void runProxyTest(newEntryTestKey, newEntry.url)}
            loading={testStates[newEntryTestKey]?.loading === true}
            disabled={disableControls}
          >
            <span className={styles.buttonContent}>
              <IconShield size={16} />
              {t('proxy_pool.actions.test', { defaultValue: '测试代理' })}
            </span>
          </Button>
          <Button onClick={() => void handleAdd()} loading={adding} disabled={disableControls}>
            <span className={styles.buttonContent}>
              <IconPlus size={16} />
              {t('common.add')}
            </span>
          </Button>
        </div>

        <div className={styles.batchSection}>
          <div className={styles.rowHeader}>
            <div>
              <div className={styles.rowTitle}>
                {t('proxy_pool.batch.title', { defaultValue: '批量添加' })}
              </div>
              <div className={styles.batchHint}>
                {t('proxy_pool.batch.hint', {
                  defaultValue: '每行粘贴一个代理地址。',
                })}
              </div>
            </div>
            <div className={styles.toggleGroup}>
              <div className={styles.toggleLabel}>
                {t('proxy_pool.fields.enabled', { defaultValue: '参与调度' })}
              </div>
              <ToggleSwitch
                checked={!batchDisabled}
                onChange={(checked) => setBatchDisabled(!checked)}
                ariaLabel={t('proxy_pool.fields.enabled', { defaultValue: '参与调度' })}
                disabled={disableControls}
              />
            </div>
          </div>

          <textarea
            className={styles.batchTextarea}
            value={batchInput}
            onChange={(event) => setBatchInput(event.target.value)}
            placeholder={[
              'socks5://user:pass@host-a:3000',
              'socks5://user:pass@host-b:3000',
              'http://user:pass@host-c:8080',
            ].join('\n')}
            disabled={disableControls}
            rows={7}
          />

          {batchInput.trim() ? (
            <div className={styles.batchSummary}>
              <span className={styles.batchSummaryItem}>
                {t('proxy_pool.batch.valid_count', {
                  count: batchPreview.entries.length,
                  defaultValue: `可添加 ${batchPreview.entries.length} 个`,
                })}
              </span>
              {batchPreview.existingCount > 0 ? (
                <span className={styles.batchSummaryItem}>
                  {t('proxy_pool.batch.existing_count', {
                    count: batchPreview.existingCount,
                    defaultValue: `已存在 ${batchPreview.existingCount} 个`,
                  })}
                </span>
              ) : null}
              {batchPreview.duplicateCount > 0 ? (
                <span className={styles.batchSummaryItem}>
                  {t('proxy_pool.batch.duplicate_count', {
                    count: batchPreview.duplicateCount,
                    defaultValue: `重复 ${batchPreview.duplicateCount} 个`,
                  })}
                </span>
              ) : null}
              {batchPreview.invalidValues.length > 0 ? (
                <span className={`${styles.batchSummaryItem} ${styles.batchSummaryError}`}>
                  {t('proxy_pool.batch.invalid_count', {
                    count: batchPreview.invalidValues.length,
                    defaultValue: `无效 ${batchPreview.invalidValues.length} 个`,
                  })}
                </span>
              ) : null}
            </div>
          ) : null}

          {batchPreview.invalidValues.length > 0 ? (
            <div className={styles.batchInvalidList}>
              {batchPreview.invalidValues.slice(0, 3).map((item) => (
                <div className={styles.mono} key={item}>
                  {item}
                </div>
              ))}
              {batchPreview.invalidValues.length > 3 ? (
                <div>
                  {t('proxy_pool.batch.more_invalid', {
                    count: batchPreview.invalidValues.length - 3,
                    defaultValue: `还有 ${batchPreview.invalidValues.length - 3} 个无效地址未展示`,
                  })}
                </div>
              ) : null}
            </div>
          ) : null}

          {pageDraftDirty ? (
            <div className={styles.batchWarning}>
              {t('proxy_pool.batch.dirty_warning', {
                defaultValue: '当前页面有未保存修改，请先保存或重置后再批量添加。',
              })}
            </div>
          ) : null}

          <div className={styles.rowActions}>
            <Button
              variant="secondary"
              onClick={() => setBatchInput('')}
              disabled={!batchInput.trim() || disableControls}
            >
              {t('common.clear', { defaultValue: '清空' })}
            </Button>
            <Button
              onClick={() => void handleBatchAdd()}
              loading={batchAdding}
              disabled={disableControls || pageDraftDirty || batchPreview.entries.length === 0}
            >
              <span className={styles.buttonContent}>
                <IconPlus size={16} />
                {t('proxy_pool.batch.add_button', { defaultValue: '批量添加' })}
              </span>
            </Button>
          </div>
        </div>
      </Card>

      <Card
        className={styles.card}
        title={t('proxy_pool.logs_title', { defaultValue: '最近代理使用记录' })}
        extra={
          <div className={styles.searchBox}>
            <Input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t('proxy_pool.search_placeholder', {
                defaultValue: '搜索请求 ID、代理或日志文件',
              })}
              rightElement={<IconSearch size={16} className={styles.searchIcon} />}
            />
          </div>
        }
      >
        <div className={styles.cardDescription}>
          {t('proxy_pool.logs_hint', {
            defaultValue: '这里只展示最近 100 条请求日志里的代理链路，需要后端已开启 request-log。',
          })}
        </div>
        {filteredLogs.length === 0 ? (
          <EmptyState
            title={t('proxy_pool.logs_empty_title', { defaultValue: '还没有可展示的代理请求记录' })}
            description={t('proxy_pool.logs_empty_desc', {
              defaultValue: '确认 request-log 已开启，并且已经有请求经过代理池后再回来查看。',
            })}
          />
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>{t('proxy_pool.columns.time', { defaultValue: '北京时间' })}</th>
                  <th>{t('proxy_pool.columns.request_id', { defaultValue: '请求 ID' })}</th>
                  <th>{t('proxy_pool.columns.proxy', { defaultValue: '代理链路' })}</th>
                  <th>{t('proxy_pool.columns.log_file', { defaultValue: '日志文件' })}</th>
                  <th>{t('proxy_pool.columns.size', { defaultValue: '文件大小' })}</th>
                </tr>
              </thead>
              <tbody>
                {filteredLogs.map((item) => (
                  <tr key={`${item.name}-${item.requestId}`}>
                    <td>{formatBeijingDateTime(item.modified)}</td>
                    <td className={styles.mono}>{item.requestId || '-'}</td>
                    <td className={styles.mono}>{formatProxyChain(item) || '-'}</td>
                    <td className={styles.mono}>{item.name}</td>
                    <td>{item.size > 0 ? formatFileSize(item.size) : '-'}</td>
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
