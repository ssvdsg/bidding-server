const BASE_URL = '/api';
const ACCESS_PASSWORD_KEY = 'bidding_access_password';

/** Exponential backoff retry config */
const RETRY_MAX = 2;
const RETRY_BASE_DELAY_MS = 1000;
const RETRYABLE_STATUS = new Set([502, 503, 504]);

/** Returns true for transient failures worth retrying */
function isRetryable(err: unknown): boolean {
  if (err instanceof TypeError) return true;              // network cut, DNS failure, etc.
  if (err instanceof APIError) return RETRYABLE_STATUS.has(err.status);
  return false;
}
const ACCESS_PASSWORD_PROTECTED_ENDPOINTS = new Set([
  '/settings/system',
  '/config',
  '/getConfig',
  '/saveConfig',
  '/settings/auto-exclude',
  '/executeSQL',
]);

class APIError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'APIError';
  }
}

async function request<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const url = endpoint.startsWith('http') ? endpoint : `${BASE_URL}${endpoint}`;
  
  const headers = new Headers(options?.headers);
  if (!headers.has('Content-Type') && !(options?.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json');
  }
  if (!headers.has('X-Access-Password') && requiresAccessPassword(endpoint)) {
    const password = getSessionStorage()?.getItem(ACCESS_PASSWORD_KEY)
      || getLegacyLocalStorage()?.getItem(ACCESS_PASSWORD_KEY)
      || '';
    if (password) headers.set('X-Access-Password', password);
  }

  let lastError: unknown;

  for (let attempt = 0; attempt <= RETRY_MAX; attempt++) {
    try {
      const response = await fetch(url, { ...options, headers });

      // Handle empty responses
      const text = await response.text();
      if (!text) return {} as T;

      let parsed: unknown = text;
      try {
        parsed = JSON.parse(text) as T;
      } catch {
        parsed = text;
      }

      if (!response.ok) {
        const message =
          typeof parsed === 'object' && parsed !== null
            ? ((parsed as Record<string, unknown>).error as string)
              || ((parsed as Record<string, unknown>).error_msg as string)
              || ((parsed as Record<string, unknown>).message as string)
            : undefined;
        throw new APIError(response.status, message || `HTTP error! status: ${response.status}`);
      }

      return parsed as T;
    } catch (err) {
      lastError = err;
      if (attempt < RETRY_MAX && isRetryable(err)) {
        await new Promise(r => setTimeout(r, RETRY_BASE_DELAY_MS * (1 << attempt)));
        continue;
      }
      throw err;
    }
  }

  throw lastError;
}

function requiresAccessPassword(endpoint: string): boolean {
  if (!endpoint) return false;

  const normalized = endpoint.startsWith(BASE_URL)
    ? endpoint.slice(BASE_URL.length)
    : endpoint.startsWith('http')
      ? new URL(endpoint).pathname.replace(BASE_URL, '')
      : endpoint;

  return ACCESS_PASSWORD_PROTECTED_ENDPOINTS.has(normalized);
}

function getSessionStorage(): Storage | null {
  if (typeof window === 'undefined') return null;
  return window.sessionStorage;
}

function getLegacyLocalStorage(): Storage | null {
  if (typeof window === 'undefined') return null;
  return window.localStorage;
}

export const client = {
  get: <T>(endpoint: string, options?: RequestInit) => 
    request<T>(endpoint, { ...options, method: 'GET' }),
    
  post: <T>(endpoint: string, body: any, options?: RequestInit) => 
    request<T>(endpoint, { 
      ...options, 
      method: 'POST', 
      body: body instanceof FormData ? body : JSON.stringify(body) 
    }),
    
  put: <T>(endpoint: string, body: any, options?: RequestInit) => 
    request<T>(endpoint, { 
      ...options, 
      method: 'PUT', 
      body: body instanceof FormData ? body : JSON.stringify(body) 
    }),
    
  delete: <T>(endpoint: string, options?: RequestInit) => 
    request<T>(endpoint, { ...options, method: 'DELETE' }),
};

export const accessPasswordStore = {
  get: () => {
    const sessionStorage = getSessionStorage();
    const localStorage = getLegacyLocalStorage();
    const sessionPassword = sessionStorage?.getItem(ACCESS_PASSWORD_KEY) || '';

    if (sessionPassword) return sessionPassword;

    const legacyPassword = localStorage?.getItem(ACCESS_PASSWORD_KEY) || '';
    if (legacyPassword && sessionStorage) {
      sessionStorage.setItem(ACCESS_PASSWORD_KEY, legacyPassword);
      localStorage?.removeItem(ACCESS_PASSWORD_KEY);
      return legacyPassword;
    }

    return legacyPassword;
  },
  set: (password: string) => {
    const sessionStorage = getSessionStorage();
    const localStorage = getLegacyLocalStorage();
    sessionStorage?.setItem(ACCESS_PASSWORD_KEY, password);
    localStorage?.removeItem(ACCESS_PASSWORD_KEY);
  },
  clear: () => {
    getSessionStorage()?.removeItem(ACCESS_PASSWORD_KEY);
    getLegacyLocalStorage()?.removeItem(ACCESS_PASSWORD_KEY);
  },
};

export type ApiEnvelope<T> = {
  error_code?: number;
  error?: string;
  error_msg?: string;
  message?: string;
  data?: T;
};

export function unwrapData<T>(payload: T | ApiEnvelope<T>): T {
  if (
    payload &&
    typeof payload === 'object' &&
    'data' in payload
  ) {
    return (payload as ApiEnvelope<T>).data as T;
  }

  return payload as T;
}
