import { Server as SocketIOServer } from "socket.io";
import { Server as HTTPServer } from "http";

/**
 * Workflow WebSocket Service
 * Provides real-time workflow execution updates via WebSocket
 */

export interface WorkflowExecutionEvent {
  type: "workflow_started" | "workflow_completed" | "workflow_failed" | "activity_started" | "activity_completed" | "activity_failed";
  workflowId: string;
  runId: string;
  workflowType: string;
  timestamp: Date;
  data?: any;
}

export interface WorkflowMetricsUpdate {
  totalExecutions: number;
  completedExecutions: number;
  failedExecutions: number;
  runningExecutions: number;
  successRate: number;
  averageDuration: number;
  timestamp: Date;
}

let io: SocketIOServer | null = null;

/**
 * Initialize WebSocket server
 */
export function initializeWorkflowWebSocket(httpServer: HTTPServer): SocketIOServer {
  io = new SocketIOServer(httpServer, {
    cors: {
      origin: "*", // Configure appropriately for production
      methods: ["GET", "POST"],
    },
    path: "/ws/workflow",
  });

  io.on("connection", (socket) => {
    console.log(`[WebSocket] Client connected: ${socket.id}`);

    // Join workflow monitoring room
    socket.on("join:workflow-monitoring", () => {
      socket.join("workflow-monitoring");
      console.log(`[WebSocket] Client ${socket.id} joined workflow-monitoring room`);
    });

    // Join specific workflow room
    socket.on("join:workflow", (workflowId: string) => {
      socket.join(`workflow:${workflowId}`);
      console.log(`[WebSocket] Client ${socket.id} joined workflow:${workflowId} room`);
    });

    // Leave workflow room
    socket.on("leave:workflow", (workflowId: string) => {
      socket.leave(`workflow:${workflowId}`);
      console.log(`[WebSocket] Client ${socket.id} left workflow:${workflowId} room`);
    });

    socket.on("disconnect", () => {
      console.log(`[WebSocket] Client disconnected: ${socket.id}`);
    });
  });

  return io;
}

/**
 * Broadcast workflow execution event
 */
export function broadcastWorkflowEvent(event: WorkflowExecutionEvent): void {
  if (!io) {
    console.warn("[WebSocket] Socket.IO not initialized");
    return;
  }

  // Broadcast to workflow monitoring room
  io.to("workflow-monitoring").emit("workflow:event", event);

  // Broadcast to specific workflow room
  io.to(`workflow:${event.workflowId}`).emit("workflow:event", event);

  console.log(`[WebSocket] Broadcasted ${event.type} for workflow ${event.workflowId}`);
}

/**
 * Broadcast workflow metrics update
 */
export function broadcastMetricsUpdate(metrics: WorkflowMetricsUpdate): void {
  if (!io) {
    console.warn("[WebSocket] Socket.IO not initialized");
    return;
  }

  io.to("workflow-monitoring").emit("metrics:update", metrics);
  console.log("[WebSocket] Broadcasted metrics update");
}

/**
 * Broadcast activity execution update
 */
export function broadcastActivityUpdate(workflowId: string, activityName: string, status: string, data?: any): void {
  if (!io) {
    console.warn("[WebSocket] Socket.IO not initialized");
    return;
  }

  const update = {
    workflowId,
    activityName,
    status,
    timestamp: new Date(),
    data,
  };

  io.to(`workflow:${workflowId}`).emit("activity:update", update);
  console.log(`[WebSocket] Broadcasted activity update for ${workflowId}/${activityName}`);
}

/**
 * Broadcast workflow alert
 */
export function broadcastWorkflowAlert(alert: {
  workflowId: string;
  severity: string;
  message: string;
  timestamp: Date;
}): void {
  if (!io) {
    console.warn("[WebSocket] Socket.IO not initialized");
    return;
  }

  io.to("workflow-monitoring").emit("workflow:alert", alert);
  console.log(`[WebSocket] Broadcasted workflow alert for ${alert.workflowId}`);
}

/**
 * Get connected clients count
 */
export function getConnectedClientsCount(): number {
  if (!io) return 0;
  return io.sockets.sockets.size;
}

/**
 * Get room members count
 */
export async function getRoomMembersCount(room: string): Promise<number> {
  if (!io) return 0;
  const sockets = await io.in(room).fetchSockets();
  return sockets.length;
}

/**
 * Workflow event emitter for integration with Temporal
 * This would be called from Temporal workflow activities
 */
export class WorkflowEventEmitter {
  static emitWorkflowStarted(workflowId: string, runId: string, workflowType: string, data?: any): void {
    broadcastWorkflowEvent({
      type: "workflow_started",
      workflowId,
      runId,
      workflowType,
      timestamp: new Date(),
      data,
    });
  }

  static emitWorkflowCompleted(workflowId: string, runId: string, workflowType: string, data?: any): void {
    broadcastWorkflowEvent({
      type: "workflow_completed",
      workflowId,
      runId,
      workflowType,
      timestamp: new Date(),
      data,
    });
  }

  static emitWorkflowFailed(workflowId: string, runId: string, workflowType: string, error: string): void {
    broadcastWorkflowEvent({
      type: "workflow_failed",
      workflowId,
      runId,
      workflowType,
      timestamp: new Date(),
      data: { error },
    });
  }

  static emitActivityStarted(workflowId: string, runId: string, workflowType: string, activityName: string): void {
    broadcastWorkflowEvent({
      type: "activity_started",
      workflowId,
      runId,
      workflowType,
      timestamp: new Date(),
      data: { activityName },
    });
  }

  static emitActivityCompleted(workflowId: string, runId: string, workflowType: string, activityName: string, result?: any): void {
    broadcastWorkflowEvent({
      type: "activity_completed",
      workflowId,
      runId,
      workflowType,
      timestamp: new Date(),
      data: { activityName, result },
    });
  }

  static emitActivityFailed(workflowId: string, runId: string, workflowType: string, activityName: string, error: string): void {
    broadcastWorkflowEvent({
      type: "activity_failed",
      workflowId,
      runId,
      workflowType,
      timestamp: new Date(),
      data: { activityName, error },
    });
  }
}

/**
 * Metrics aggregator for periodic updates
 * This would run on a schedule to push metrics updates
 */
export class WorkflowMetricsAggregator {
  private static intervalId: NodeJS.Timeout | null = null;

  static start(intervalMs: number = 5000): void {
    if (this.intervalId) {
      console.warn("[Metrics Aggregator] Already running");
      return;
    }

    this.intervalId = setInterval(async () => {
      try {
        // Fetch current metrics from database
        // This is a placeholder - implement actual database queries
        const metrics: WorkflowMetricsUpdate = {
          totalExecutions: 0,
          completedExecutions: 0,
          failedExecutions: 0,
          runningExecutions: 0,
          successRate: 0,
          averageDuration: 0,
          timestamp: new Date(),
        };

        broadcastMetricsUpdate(metrics);
      } catch (error) {
        console.error("[Metrics Aggregator] Error:", error);
      }
    }, intervalMs);

    console.log(`[Metrics Aggregator] Started with interval ${intervalMs}ms`);
  }

  static stop(): void {
    if (this.intervalId) {
      clearInterval(this.intervalId);
      this.intervalId = null;
      console.log("[Metrics Aggregator] Stopped");
    }
  }
}
