import { DEFAULT_API_PORT, MANAGEMENT_API_PREFIX } from './constants';

const LOCAL_FRONTEND_PORTS = new Set(['4173', '4174']);

const normalizeLocalManagementPort = (value: string): string => {
  try {
    const parsed = new URL(value);
    const isLocalHost =
      parsed.hostname === '127.0.0.1' || parsed.hostname === 'localhost' || parsed.hostname === '::1';
    if (isLocalHost && LOCAL_FRONTEND_PORTS.has(parsed.port)) {
      parsed.port = String(DEFAULT_API_PORT);
      return parsed.toString().replace(/\/+$/i, '');
    }
  } catch {
    return value;
  }

  return value;
};

export const normalizeApiBase = (input: string): string => {
  let base = (input || '').trim();
  if (!base) return '';
  base = base.replace(/\/?v0\/management\/?$/i, '');
  base = base.replace(/\/+$/i, '');
  if (!/^https?:\/\//i.test(base)) {
    base = `http://${base}`;
  }
  return normalizeLocalManagementPort(base);
};

export const computeApiUrl = (base: string): string => {
  const normalized = normalizeApiBase(base);
  if (!normalized) return '';
  return `${normalized}${MANAGEMENT_API_PREFIX}`;
};

export const detectApiBaseFromLocation = (): string => {
  try {
    const { protocol, hostname, port } = window.location;
    const resolvedPort = LOCAL_FRONTEND_PORTS.has(port) ? String(DEFAULT_API_PORT) : port;
    const normalizedPort = resolvedPort ? `:${resolvedPort}` : '';
    return normalizeApiBase(`${protocol}//${hostname}${normalizedPort}`);
  } catch (error) {
    console.warn('Failed to detect api base from location, fallback to default', error);
    return normalizeApiBase(`http://localhost:${DEFAULT_API_PORT}`);
  }
};

export const isLocalhost = (hostname: string): boolean => {
  const value = (hostname || '').toLowerCase();
  return value === 'localhost' || value === '127.0.0.1' || value === '[::1]';
};
