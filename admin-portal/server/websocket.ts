import { Server as SocketIOServer } from "socket.io";
import type { Server as HTTPServer } from "http";

let io: SocketIOServer | null = null;

/**
 * Initialize Socket.IO server for real-time updates
 */
export function initializeWebSocket(httpServer: HTTPServer): SocketIOServer {
  io = new SocketIOServer(httpServer, {
    cors: {
      origin: "*", // In production, restrict this to your domain
      methods: ["GET", "POST"],
    },
    path: "/api/socket.io/",
  });

  io.on("connection", (socket) => {
    console.log(`[WebSocket] Client connected: ${socket.id}`);

    socket.on("disconnect", () => {
      console.log(`[WebSocket] Client disconnected: ${socket.id}`);
    });

    // Send initial connection confirmation
    socket.emit("connected", { message: "Connected to Admin Portal real-time updates" });
  });

  console.log("[WebSocket] Server initialized");
  return io;
}

/**
 * Get the Socket.IO server instance
 */
export function getWebSocketServer(): SocketIOServer | null {
  return io;
}

/**
 * Broadcast dashboard metrics update to all connected clients
 */
export function broadcastDashboardUpdate(metrics: {
  totalUsers: number;
  benefitPrograms: number;
  featureFlags: number;
  upcomingDisbursements: number;
}): void {
  if (!io) return;
  io.emit("dashboard:update", metrics);
}

/**
 * Broadcast new activity to all connected clients
 */
export function broadcastActivity(activity: {
  id: number;
  action: string;
  performedBy: number;
  performedByName: string;
  performedAt: Date;
  details?: any;
}): void {
  if (!io) return;
  io.emit("activity:new", activity);
}

/**
 * Broadcast program update to all connected clients
 */
export function broadcastProgramUpdate(programId: number, action: "created" | "updated" | "deleted"): void {
  if (!io) return;
  io.emit("program:update", { programId, action });
}

/**
 * Broadcast user update to all connected clients
 */
export function broadcastUserUpdate(userId: number, action: "role_changed" | "created" | "deleted"): void {
  if (!io) return;
  io.emit("user:update", { userId, action });
}

/**
 * Broadcast feature flag update to all connected clients
 */
export function broadcastFeatureFlagUpdate(flagId: number, action: "toggled" | "created" | "deleted"): void {
  if (!io) return;
  io.emit("feature_flag:update", { flagId, action });
}
