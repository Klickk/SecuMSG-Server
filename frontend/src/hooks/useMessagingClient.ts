import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

export interface RegistrationForm {
  keysUrl: string;
  messagesUrl: string;
  userId?: string;
  deviceId?: string;
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

interface ClientInitResponse {
  state: string;
  userId: string;
  deviceId: string;
  keysUrl: string;
  messagesUrl: string;
  oneTimePrekeys: number;
}

interface ClientSendResponse {
  state: string;
}

interface ClientEnvelopeResponse {
  state: string;
  plaintext: string;
}

const STORAGE_KEY = 'secumsg-state-v1';
const API_BASE_URL = (import.meta.env.VITE_CLIENT_API_URL as string | undefined) ?? 'http://localhost:8080';

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
  const stateRef = useRef<string | null>(null);

  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      setState(saved);
      stateRef.current = saved;
      setInfo(extractStateInfo(saved));
    }
  }, []);

  const ready = useMemo(() => true, []);

  const persistState = useCallback((value: string) => {
    stateRef.current = value;
    setState(value);
    localStorage.setItem(STORAGE_KEY, value);
    setInfo(extractStateInfo(value));
  }, []);

  const register = useCallback(
    async (form: RegistrationForm) => {
      const payload = {
        keysUrl: form.keysUrl.trim(),
        messagesUrl: form.messagesUrl.trim(),
        userId: form.userId?.trim(),
        deviceId: form.deviceId?.trim()
      };
      const result = await postJSON<ClientInitResponse>('/client/init', payload);
      persistState(result.state);
      setMessages([]);
      return result;
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
      if (!stateRef.current) {
        throw new Error('Device is not initialized');
      }
      const payload = {
        state: stateRef.current,
        convId: form.convId.trim(),
        toDeviceId: form.toDeviceId.trim(),
        plaintext: form.message
      };
      const result = await postJSON<ClientSendResponse>('/client/send', payload);
      persistState(result.state);
    },
    [persistState]
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
      if (!stateRef.current) {
        return;
      }
      try {
        const text = typeof event.data === 'string' ? event.data : '';
        if (!text) {
          return;
        }
        const envelope = JSON.parse(text) as InboundEnvelope;
        const response = await postJSON<ClientEnvelopeResponse>('/client/envelope', {
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

  return {
    ready,
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
    const raw = JSON.parse(stateJSON) as {
      user_id?: string;
      device_id?: string;
      keys_base_url?: string;
      messages_base_url?: string;
    };
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

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const target = new URL(path, API_BASE_URL);
  const response = await fetch(target.toString(), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || 'Request failed');
  }
  return (await response.json()) as T;
}
