export interface WasmRegistrationResult {
  state: string;
  userId: string;
  deviceId: string;
  keysUrl: string;
  messagesUrl: string;
  oneTimePrekeys: number;
}

export interface WasmPrepareSendResult {
  state: string;
  request: Record<string, unknown>;
}

export interface WasmHandleEnvelopeResult {
  state: string;
  plaintext: string;
}

export interface WasmStateInfo {
  userId: string;
  deviceId: string;
  keysUrl: string;
  messagesUrl: string;
}

export interface WasmClient {
  init(options: {
    keysUrl: string;
    messagesUrl: string;
    userId?: string;
    deviceId?: string;
    accessToken?: string;
  }): Promise<WasmRegistrationResult>;
  prepareSend(options: {
    state: string;
    convId: string;
    toDeviceId: string;
    plaintext: string;
  }): Promise<WasmPrepareSendResult>;
  handleEnvelope(options: {
    state: string;
    envelope: unknown;
  }): Promise<WasmHandleEnvelopeResult>;
  stateInfo(state: string): WasmStateInfo | null;
}

declare class Go {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

type GoConstructor = new () => Go;

type GlobalWithRuntime = typeof globalThis & {
  Go?: GoConstructor;
  msgClientInit?: (options: Record<string, unknown>) => Promise<unknown>;
  msgClientPrepareSend?: (options: Record<string, unknown>) => Promise<unknown>;
  msgClientHandleEnvelope?: (options: Record<string, unknown>) => Promise<unknown>;
  msgClientStateInfo?: (state: string) => unknown;
};

const WASM_EXEC = '/wasm_exec.js';
const WASM_MODULE = '/msgclient.wasm';

let activeGo: Go | null = null;

let clientPromise: Promise<WasmClient> | null = null;

export function loadWasmClient(): Promise<WasmClient> {
  if (!clientPromise) {
    clientPromise = initialize().catch((err) => {
      clientPromise = null;
      throw err;
    });
  }
  return clientPromise;
}

async function initialize(): Promise<WasmClient> {
  const globalRuntime = globalThis as GlobalWithRuntime;
  await ensureGoRuntime(globalRuntime);
  const GoCtor = globalRuntime.Go!;
  const go = new GoCtor();
  activeGo = go;
  const instance = await instantiateModule(go);
  // Start the Go runtime without awaiting the returned promise (it resolves when the program exits).
  go.run(instance)
    .catch((err) => {
    console.error('Go runtime exited unexpectedly', err);
  })
    .finally(() => {
      activeGo = null;
    });

  const call = <T>(name: keyof GlobalWithRuntime, ...args: unknown[]): T => {
    const fn = globalRuntime[name];
    if (typeof fn !== 'function') {
      throw new Error(`WASM export ${String(name)} is not available`);
    }
    return (fn as (...innerArgs: unknown[]) => unknown)(...args) as T;
  };

  const init = async (options: {
    keysUrl: string;
    messagesUrl: string;
    userId?: string;
    deviceId?: string;
    accessToken?: string;
  }): Promise<WasmRegistrationResult> => {
    const payload = {
      keysURL: options.keysUrl,
      messagesURL: options.messagesUrl,
      userID: options.userId ?? '',
      deviceID: options.deviceId ?? '',
      accessToken: options.accessToken ?? ''
    };
    const result = (await call<Promise<Record<string, unknown>>>(
      'msgClientInit',
      payload
    )) as Record<string, unknown>;
    return normalizeRegistration(result);
  };

  const prepareSend = async (options: {
    state: string;
    convId: string;
    toDeviceId: string;
    plaintext: string;
  }): Promise<WasmPrepareSendResult> => {
    const payload = {
      state: options.state,
      convId: options.convId,
      toDeviceId: options.toDeviceId,
      plaintext: options.plaintext
    };
    const result = (await call<Promise<Record<string, unknown>>>(
      'msgClientPrepareSend',
      payload
    )) as Record<string, unknown>;
    return normalizePrepareSend(result);
  };

  const handleEnvelope = async (options: {
    state: string;
    envelope: unknown;
  }): Promise<WasmHandleEnvelopeResult> => {
    const payload = {
      state: options.state,
      envelope: JSON.stringify(options.envelope)
    };
    const result = (await call<Promise<Record<string, unknown>>>(
      'msgClientHandleEnvelope',
      payload
    )) as Record<string, unknown>;
    return normalizeHandleEnvelope(result);
  };

  const stateInfo = (state: string): WasmStateInfo | null => {
    try {
      const raw = call<Record<string, unknown> | null>('msgClientStateInfo', state);
      if (!raw) {
        return null;
      }
      return normalizeStateInfo(raw);
    } catch (err) {
      console.error('Failed to extract state info', err);
      return null;
    }
  };

  return { init, prepareSend, handleEnvelope, stateInfo };
}

async function ensureGoRuntime(globalRuntime: GlobalWithRuntime): Promise<void> {
  if (typeof globalRuntime.Go === 'function') {
    return;
  }
  await loadScript(WASM_EXEC);
  if (typeof globalRuntime.Go !== 'function') {
    throw new Error('Go WebAssembly runtime (wasm_exec.js) is unavailable');
  }
}

async function loadScript(src: string): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    const existing = document.querySelector(`script[src="${src}"]`) as
      | (HTMLScriptElement & { dataset: DOMStringMap & { loaded?: string } })
      | null;
    if (existing) {
      if (existing.dataset.loaded === 'true') {
        resolve();
        return;
      }
      existing.addEventListener('load', () => resolve(), { once: true });
      existing.addEventListener('error', () => reject(new Error(`failed to load ${src}`)), {
        once: true
      });
      return;
    }
    const script = document.createElement('script');
    script.src = src;
    script.async = true;
    script.dataset.loaded = 'false';
    script.addEventListener(
      'load',
      () => {
        script.dataset.loaded = 'true';
        resolve();
      },
      { once: true }
    );
    script.addEventListener('error', () => reject(new Error(`failed to load ${src}`)), {
      once: true
    });
    document.head.appendChild(script);
  });
}

async function instantiateModule(go: Go): Promise<WebAssembly.Instance> {
  if ('instantiateStreaming' in WebAssembly) {
    try {
      const response = await fetch(WASM_MODULE);
      return (await WebAssembly.instantiateStreaming(response, go.importObject)).instance;
    } catch (err) {
      console.warn('instantiateStreaming failed, retrying with ArrayBuffer', err);
    }
  }
  const response = await fetch(WASM_MODULE);
  const buffer = await response.arrayBuffer();
  return (await WebAssembly.instantiate(buffer, go.importObject)).instance;
}

function normalizeRegistration(raw: Record<string, unknown>): WasmRegistrationResult {
  return {
    state: String(raw.state ?? ''),
    userId: String(raw.userId ?? ''),
    deviceId: String(raw.deviceId ?? ''),
    keysUrl: String(raw.keysUrl ?? ''),
    messagesUrl: String(raw.messagesUrl ?? ''),
    oneTimePrekeys: Number(raw.oneTimePrekeys ?? 0)
  };
}

function normalizePrepareSend(raw: Record<string, unknown>): WasmPrepareSendResult {
  const request = (raw.request as Record<string, unknown>) ?? {};
  return {
    state: String(raw.state ?? ''),
    request
  };
}

function normalizeHandleEnvelope(raw: Record<string, unknown>): WasmHandleEnvelopeResult {
  return {
    state: String(raw.state ?? ''),
    plaintext: String(raw.plaintext ?? '')
  };
}

function normalizeStateInfo(raw: Record<string, unknown>): WasmStateInfo {
  return {
    userId: String(raw.userId ?? ''),
    deviceId: String(raw.deviceId ?? ''),
    keysUrl: String(raw.keysUrl ?? ''),
    messagesUrl: String(raw.messagesUrl ?? '')
  };
}
