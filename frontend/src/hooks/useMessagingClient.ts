import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { loadWasmClient, WasmClient, WasmStateInfo } from '../lib/wasmClient';

export interface RegistrationForm {
  keysUrl: string;
  messagesUrl: string;
  userId?: string;
  deviceId?: string;
}

export interface RegistrationResult {
  state: string;
  userId: string;
  deviceId: string;
  keysUrl: string;
  messagesUrl: string;
  oneTimePrekeys: number;
}

export interface SendForm {
  convId: string;
  toDeviceId: string;
  message: string;
}

export interface MessageRecord {
  id: string;
  convId: string;
  fromDeviceId: string;
  toDeviceId: string;
  sentAt: string;
  plaintext: string;
}

export interface StateInfo {
  userId: string;
  deviceId: string;
  keysUrl: string;
  messagesUrl: string;
}

const STORAGE_KEY = 'secumsg-state-v1';

interface ListenerState {
  websocket?: WebSocket;
  status: 'idle' | 'connecting' | 'connected' | 'error';
  error?: string;
}

export function useMessagingClient() {
  const [state, setState] = useState<string | null>(null);
  const [info, setInfo] = useState<StateInfo | null>(null);
  const [messages, setMessages] = useState<MessageRecord[]>([]);
  const [listener, setListener] = useState<ListenerState>({ status: 'idle' });
  const [ready, setReady] = useState(false);
  const [engineError, setEngineError] = useState<string | null>(null);
  const stateRef = useRef<string | null>(null);
  const clientRef = useRef<WasmClient | null>(null);

  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      setState(saved);
      stateRef.current = saved;
      setInfo(extractStateInfo(saved));
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    loadWasmClient()
      .then((client) => {
        if (cancelled) {
          return;
        }
        clientRef.current = client;
        setReady(true);
        setEngineError(null);
        if (stateRef.current) {
          const currentInfo = client.stateInfo(stateRef.current);
          if (currentInfo) {
            setInfo(currentInfo);
          }
        }
      })
      .catch((err) => {
        if (cancelled) {
          return;
        }
        console.error('Failed to load wasm client', err);
        setEngineError(err instanceof Error ? err.message : String(err));
        setReady(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const updateInfo = useCallback((value: string) => {
    const client = clientRef.current;
    if (client) {
      const snapshot = client.stateInfo(value);
      if (snapshot) {
        setInfo(snapshot);
        return;
      }
    }
    setInfo(extractStateInfo(value));
  }, []);

  const persistState = useCallback(
    (value: string) => {
      stateRef.current = value;
      setState(value);
      localStorage.setItem(STORAGE_KEY, value);
      updateInfo(value);
    },
    [updateInfo]
  );

  const register = useCallback(
    async (form: RegistrationForm): Promise<RegistrationResult> => {
      if (!clientRef.current) {
        throw new Error('Encryption engine is still loading');
      }
      const payload = {
        keysUrl: form.keysUrl.trim(),
        messagesUrl: form.messagesUrl.trim(),
        userId: form.userId?.trim(),
        deviceId: form.deviceId?.trim()
      };
      const result = await clientRef.current.init(payload);
      persistState(result.state);
      setMessages([]);
      return {
        state: result.state,
        userId: result.userId,
        deviceId: result.deviceId,
        keysUrl: result.keysUrl,
        messagesUrl: result.messagesUrl,
        oneTimePrekeys: result.oneTimePrekeys
      };
    },
    [persistState]
  );

  const reset = useCallback(() => {
    setState(null);
    stateRef.current = null;
    localStorage.removeItem(STORAGE_KEY);
    setInfo(null);
    setMessages([]);
    if (listener.websocket) {
      listener.websocket.close();
    }
    setListener({ status: 'idle' });
  }, [listener.websocket]);

  const sendMessage = useCallback(
    async (form: SendForm) => {
      if (!stateRef.current || !info) {
        throw new Error('Device is not initialized');
      }
      if (!clientRef.current) {
        throw new Error('Encryption engine is still loading');
      }
      const result = await clientRef.current.prepareSend({
        state: stateRef.current,
        convId: form.convId.trim(),
        toDeviceId: form.toDeviceId.trim(),
        plaintext: form.message
      });
      await postEncryptedMessage(info.messagesUrl, result.request);
      persistState(result.state);
    },
    [info, persistState]
  );

  const connect = useCallback(() => {
    if (!stateRef.current || !info) {
      throw new Error('Device is not initialized');
    }
    if (listener.websocket && listener.status === 'connected') {
      return;
    }
    const wsUrl = buildWsUrl(info.messagesUrl, info.deviceId);
    const ws = new WebSocket(wsUrl);
    setListener({ status: 'connecting', websocket: ws });

    ws.onopen = () => {
      setListener({ status: 'connected', websocket: ws });
    };

    ws.onerror = () => {
      setListener({ status: 'error', websocket: ws, error: 'WebSocket error' });
    };

    ws.onclose = () => {
      setListener({ status: 'idle' });
    };

    ws.onmessage = async (event) => {
      if (!stateRef.current || !clientRef.current) {
        return;
      }
      try {
        const text = typeof event.data === 'string' ? event.data : '';
        if (!text) {
          return;
        }
        const envelope = JSON.parse(text) as InboundEnvelope;
        const response = await clientRef.current.handleEnvelope({
          state: stateRef.current,
          envelope
        });
        persistState(response.state);
        const record: MessageRecord = {
          id: envelope.id,
          convId: envelope.conv_id,
          fromDeviceId: envelope.from_device_id,
          toDeviceId: envelope.to_device_id,
          sentAt: envelope.sent_at,
          plaintext: response.plaintext
        };
        setMessages((prev) => [record, ...prev]);
      } catch (err) {
        console.error('Failed to process inbound message', err);
      }
    };
  }, [info, listener.status, listener.websocket, persistState]);

  const disconnect = useCallback(() => {
    if (listener.websocket) {
      listener.websocket.close();
    }
    setListener({ status: 'idle' });
  }, [listener.websocket]);

  const engineReady = useMemo(() => ready && engineError == null, [ready, engineError]);

  return {
    ready: engineReady,
    engineError,
    state,
    info,
    messages,
    listener,
    register,
    reset,
    sendMessage,
    connect,
    disconnect
  };
}

interface InboundEnvelope {
  id: string;
  conv_id: string;
  from_device_id: string;
  to_device_id: string;
  ciphertext: string;
  header: unknown;
  sent_at: string;
}

function buildWsUrl(baseUrl: string, deviceId: string): string {
  const url = new URL(baseUrl);
  url.pathname = url.pathname.replace(/\/?$/, '/ws');
  url.searchParams.set('device_id', deviceId);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  return url.toString();
}

function extractStateInfo(stateJSON: string): StateInfo | null {
  try {
    const raw = JSON.parse(stateJSON) as WasmStateInfo & {
      user_id?: string;
      device_id?: string;
      keys_base_url?: string;
      messages_base_url?: string;
    };
    if ('userId' in raw && raw.userId && 'deviceId' in raw && raw.deviceId) {
      return {
        userId: raw.userId,
        deviceId: raw.deviceId,
        keysUrl: raw.keysUrl,
        messagesUrl: raw.messagesUrl
      };
    }
    if (!raw || !raw.device_id || !raw.user_id || !raw.messages_base_url || !raw.keys_base_url) {
      return null;
    }
    return {
      userId: raw.user_id,
      deviceId: raw.device_id,
      keysUrl: raw.keys_base_url,
      messagesUrl: raw.messages_base_url
    };
  } catch (err) {
    console.error('Failed to parse state info', err);
    return null;
  }
}

async function postEncryptedMessage(baseUrl: string, body: Record<string, unknown>): Promise<void> {
  const target = joinUrl(baseUrl, '/messages/send');
  const response = await fetch(target, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || 'Failed to send message');
  }
}

function joinUrl(base: string, path: string): string {
  const normalized = base.trim().replace(/\/+$/, '');
  return `${normalized}${path}`;
}
