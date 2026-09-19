import { useEffect, useState } from "react";
import { io, Socket } from "socket.io-client";

let socket: Socket | null = null;
let reconnectAttempts = 0;

export function useWebSocket() {
  const [isConnected, setIsConnected] = useState(false);
  const [attempts, setAttempts] = useState(0);

  useEffect(() => {
    // Initialize socket connection if not already connected
    if (!socket) {
      socket = io({
        path: "/api/socket.io/",
        transports: ["websocket", "polling"],
        reconnection: true,
        reconnectionDelay: 1000,
        reconnectionDelayMax: 5000,
        reconnectionAttempts: Infinity,
      });

      socket.on("connect", () => {
        console.log("[WebSocket] Connected");
        setIsConnected(true);
        reconnectAttempts = 0;
        setAttempts(0);
      });

      socket.on("disconnect", () => {
        console.log("[WebSocket] Disconnected");
        setIsConnected(false);
      });

      socket.on("reconnect_attempt", (attempt) => {
        console.log(`[WebSocket] Reconnection attempt ${attempt}`);
        reconnectAttempts = attempt;
        setAttempts(attempt);
      });

      socket.on("reconnect_error", (error) => {
        console.error("[WebSocket] Reconnection error:", error);
      });

      socket.on("reconnect_failed", () => {
        console.error("[WebSocket] Reconnection failed");
      });

      socket.on("connected", (data) => {
        console.log("[WebSocket]", data.message);
      });
    }

    return () => {
      // Don't disconnect on component unmount to maintain connection across pages
    };
  }, []);

  return { socket, isConnected, reconnectAttempts: attempts };
}

export function getSocket(): Socket | null {
  return socket;
}
