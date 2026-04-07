"use client";

import { useEffect, useRef, useCallback, useState } from "react";
import type { Envelope, MsgType } from "@/lib/protocol";
import { dbg } from "@/lib/debug";

type MessageHandler = (type: MsgType, payload: unknown) => void;

export function useWebSocket(onMessage: MessageHandler) {
  const wsRef = useRef<WebSocket | null>(null);
  const onMessageRef = useRef(onMessage);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const [connected, setConnected] = useState(false);

  onMessageRef.current = onMessage;

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    // Connect directly to the Go server — Next.js rewrites don't support WebSocket
    const wsHost = process.env.NEXT_PUBLIC_WS_URL || `ws://${window.location.hostname}:8080/ws`;
    const ws = new WebSocket(wsHost);

    ws.onopen = () => {
      dbg("WS open");
      setConnected(true);
    };

    ws.onmessage = (event) => {
      try {
        const env: Envelope = JSON.parse(event.data);
        // Skip super-noisy types (positions arrives ~20Hz)
        if (env.type !== "positions") {
          dbg("WS recv:", env.type);
        }
        onMessageRef.current(env.type, env.payload);
      } catch {
        console.error("Failed to parse message:", event.data);
      }
    };

    ws.onclose = (e) => {
      dbg("WS close:", e.code, e.reason);
      setConnected(false);
      wsRef.current = null;
      // Reconnect after 2 seconds
      reconnectTimer.current = setTimeout(connect, 2000);
    };

    ws.onerror = (e) => {
      dbg("WS error:", e);
      ws.close();
    };

    wsRef.current = ws;
  }, []);

  const send = useCallback((type: MsgType, payload: unknown = {}) => {
    if (wsRef.current?.readyState !== WebSocket.OPEN) return;
    const env: Envelope = { type, payload };
    wsRef.current.send(JSON.stringify(env));
  }, []);

  const disconnect = useCallback(() => {
    clearTimeout(reconnectTimer.current);
    wsRef.current?.close();
    wsRef.current = null;
  }, []);

  useEffect(() => {
    connect();
    return () => {
      clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  return { send, connected, disconnect };
}
