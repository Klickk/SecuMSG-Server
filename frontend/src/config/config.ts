const SERVICE_HOST_KEY = "secumsg-service-host";
const DEFAULT_HOST = "localhost";
const DEFAULT_PORT = "8080";

const normalizeHost = (raw: string) => {
  const trimmed = raw.trim();
  if (!trimmed) return DEFAULT_HOST;
  const withoutProtocol = trimmed.replace(/^https?:\/\//, "");
  return withoutProtocol.replace(/\/+$/, "");
};

export const getServiceHost = (): string => {
  try {
    return normalizeHost(localStorage.getItem(SERVICE_HOST_KEY) || DEFAULT_HOST);
  } catch {
    return DEFAULT_HOST;
  }
};

export const setServiceHost = (host: string) => {
  try {
    localStorage.setItem(SERVICE_HOST_KEY, normalizeHost(host));
  } catch {
    // Non-fatal: if storage fails, fallback to default host for this session
  }
};

export const getApiBaseUrl = (): string => {
  const host = getServiceHost();
  const hasPort = /:[0-9]+$/.test(host);
  const hostWithPort = hasPort ? host : `${host}:${DEFAULT_PORT}`;
  return `http://${hostWithPort}`;
};

const config = {
  get apiBaseUrl() {
    return getApiBaseUrl();
  },
};

export default config;
