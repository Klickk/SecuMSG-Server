interface InitOptions {
  keysURL: string;
  messagesURL: string;
  userID?: string;
  deviceID?: string;
}

interface InitResult {
  state: string;
  userId: string;
  deviceId: string;
}

interface SendInput {
  state: string;
  convId: string;
  toDeviceId: string;
  plaintext: string;
}

interface SendResult {
  state: string;
  request: Record<string, unknown>;
}

interface HandleInput {
  state: string;
  envelope: string;
}

interface HandleResult {
  state: string;
  plaintext: string;
}

export interface StateInfo {
  userId: string;
  deviceId: string;
  keysUrl: string;
  messagesUrl: string;
}

export interface WasmClient {
  init(options: InitOptions): Promise<InitResult>;
  prepareSend(input: SendInput): Promise<SendResult>;
  handleEnvelope(input: HandleInput): Promise<HandleResult>;
  stateInfo(state: string): StateInfo | null;
}

declare global {
  interface Window {
    Go: any;
    msgClientInit?: (options: InitOptions) => Promise<InitResult>;
    msgClientPrepareSend?: (input: SendInput) => Promise<SendResult>;
    msgClientHandleEnvelope?: (input: HandleInput) => Promise<HandleResult>;
    msgClientStateInfo?: (state: string) => StateInfo | null;
  }
}

let clientPromise: Promise<WasmClient> | null = null;

export async function loadWasmClient(): Promise<WasmClient> {
  if (!clientPromise) {
    clientPromise = bootstrap();
  }
  return clientPromise;
}

async function bootstrap(): Promise<WasmClient> {
  await loadScript('/wasm_exec.js');
  const go = new (window as any).Go();
  const wasmResponse = await fetch('/msgclient.wasm');
  if (!wasmResponse.ok) {
    throw new Error(`failed to fetch wasm module: ${wasmResponse.status}`);
  }
  const result = await WebAssembly.instantiateStreaming(wasmResponse, go.importObject);
  go.run(result.instance);

  const init = window.msgClientInit;
  const prepareSend = window.msgClientPrepareSend;
  const handleEnvelope = window.msgClientHandleEnvelope;
  const stateInfo = window.msgClientStateInfo;

  if (!init || !prepareSend || !handleEnvelope || !stateInfo) {
    throw new Error('wasm client did not expose expected functions');
  }

  return {
    init,
    prepareSend,
    handleEnvelope,
    stateInfo
  };
}

function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (document.querySelector(`script[src="${src}"]`)) {
      resolve();
      return;
    }
    const script = document.createElement('script');
    script.src = src;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error(`failed to load ${src}`));
    document.head.appendChild(script);
  });
}
