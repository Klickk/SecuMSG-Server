import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { loadWasmClient, StateInfo, WasmClient } from '../lib/wasm';

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

const STORAGE_KEY = 'secumsg-state-v1';

interface ListenerState {
  websocket?: WebSocket;
  status: 'idle' | 'connecting' | 'connected' | 'error';
  error?: string;
}

export function useMessagingClient() {
  const [client, setClient] = useState<WasmClient | null>(null);
  const [state, setState] = useState<string | null>(null);
  const [info, setInfo] = useState<StateInfo | null>(null);
  const [messages, setMessages] = useState<MessageRecord[]>([]);
  const [listener, setListener] = useState<ListenerState>({ status: 'idle' });
  const stateRef = useRef<string | null>(null);

  useEffect(() => {
    let mounted = true;
    loadWasmClient()
      .then((loaded) => {
        if (mounted) {
          setClient(loaded);
        }
      })
      .catch((err) => {
        console.error('Failed to load wasm client', err);
      });
    return () => {
      mounted = false;
    };
  }, []);

  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      setState(saved);
      stateRef.current = saved;
    }
  }, []);

  useEffect(() => {
    if (client && state) {
      const infoValue = client.stateInfo(state);
      setInfo(infoValue ?? null);
    } else {
      setInfo(null);
    }
  }, [client, state]);

  const ready = useMemo(() => Boolean(client), [client]);

  const persistState = useCallback(
    (value: string) => {
      stateRef.current = value;
      setState(value);
      localStorage.setItem(STORAGE_KEY, value);
      if (client) {
        const infoValue = client.stateInfo(value);
        setInfo(infoValue ?? null);
      }
    },
    [client]
  );

  const register = useCallback(
    async (form: RegistrationForm) => {
      if (!client) {
        throw new Error('WASM client not ready');
      }
      const result = await client.init({
        keysURL: form.keysUrl.trim(),
        messagesURL: form.messagesUrl.trim(),
        userID: form.userId?.trim(),
        deviceID: form.deviceId?.trim()
      });
      persistState(result.state);
      setMessages([]);
      return result;
    },
    [client, persistState]
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
      if (!client) {
        throw new Error('WASM client not ready');
      }
      if (!stateRef.current || !info) {
        throw new Error('Device is not initialized');
      }
      const prepared = await client.prepareSend({
        state: stateRef.current,
        convId: form.convId.trim(),
        toDeviceId: form.toDeviceId.trim(),
        plaintext: form.message
      });
      const target = new URL('/messages/send', info.messagesUrl);
      const response = await fetch(target.toString(), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(prepared.request)
      });
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || 'Failed to send message');
      }
      persistState(prepared.state);
    },
    [client, info, persistState]
  );

  const connect = useCallback(() => {
    if (!client) {
      throw new Error('WASM client not ready');
    }
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
      if (!client || !stateRef.current) {
        return;
      }
      try {
        const text = typeof event.data === 'string' ? event.data : '';
        if (!text) {
          return;
        }
        const envelope = JSON.parse(text) as MessageRecord;
        const handled = await client.handleEnvelope({ state: stateRef.current, envelope: text });
        persistState(handled.state);
        const next: MessageRecord = {
          id: envelope.id,
          convId: envelope.convId,
          fromDeviceId: envelope.fromDeviceId,
          toDeviceId: envelope.toDeviceId,
          sentAt: envelope.sentAt,
          plaintext: handled.plaintext
        };
        setMessages((prev) => [next, ...prev]);
      } catch (err) {
        console.error('Failed to process inbound message', err);
      }
    };
  }, [client, info, listener.status, listener.websocket, persistState]);

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

function buildWsUrl(baseUrl: string, deviceId: string): string {
  const url = new URL(baseUrl);
  url.pathname = url.pathname.replace(/\/?$/, '/ws');
  url.searchParams.set('device_id', deviceId);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  return url.toString();
}
