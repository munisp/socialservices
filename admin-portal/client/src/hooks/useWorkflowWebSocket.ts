import { useEffect, useState, useCallback } from "react";
import { io, Socket } from "socket.io-client";

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

export interface ActivityUpdate {
  workflowId: string;
  activityName: string;
  status: string;
  timestamp: Date;
  data?: any;
}

export interface WorkflowAlert {
  workflowId: string;
  severity: string;
  message: string;
  timestamp: Date;
}

/**
 * Hook for workflow monitoring WebSocket connection
 */
export function useWorkflowMonitoring() {
  const [socket, setSocket] = useState<Socket | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [events, setEvents] = useState<WorkflowExecutionEvent[]>([]);
  const [metrics, setMetrics] = useState<WorkflowMetricsUpdate | null>(null);
  const [alerts, setAlerts] = useState<WorkflowAlert[]>([]);

  useEffect(() => {
    // Connect to WebSocket server
    const socketInstance = io({
      path: "/ws/workflow",
      transports: ["websocket", "polling"],
    });

    socketInstance.on("connect", () => {
      console.log("[WebSocket] Connected");
      setIsConnected(true);
      // Join workflow monitoring room
      socketInstance.emit("join:workflow-monitoring");
    });

    socketInstance.on("disconnect", () => {
      console.log("[WebSocket] Disconnected");
      setIsConnected(false);
    });

    socketInstance.on("workflow:event", (event: WorkflowExecutionEvent) => {
      console.log("[WebSocket] Workflow event:", event);
      setEvents((prev) => [event, ...prev].slice(0, 100)); // Keep last 100 events
    });

    socketInstance.on("metrics:update", (metricsUpdate: WorkflowMetricsUpdate) => {
      console.log("[WebSocket] Metrics update:", metricsUpdate);
      setMetrics(metricsUpdate);
    });

    socketInstance.on("workflow:alert", (alert: WorkflowAlert) => {
      console.log("[WebSocket] Workflow alert:", alert);
      setAlerts((prev) => [alert, ...prev].slice(0, 50)); // Keep last 50 alerts
    });

    setSocket(socketInstance);

    return () => {
      socketInstance.disconnect();
    };
  }, []);

  const clearEvents = useCallback(() => {
    setEvents([]);
  }, []);

  const clearAlerts = useCallback(() => {
    setAlerts([]);
  }, []);

  return {
    isConnected,
    events,
    metrics,
    alerts,
    clearEvents,
    clearAlerts,
  };
}

/**
 * Hook for specific workflow WebSocket updates
 */
export function useWorkflowUpdates(workflowId: string | null) {
  const [socket, setSocket] = useState<Socket | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [events, setEvents] = useState<WorkflowExecutionEvent[]>([]);
  const [activities, setActivities] = useState<ActivityUpdate[]>([]);

  useEffect(() => {
    if (!workflowId) return;

    const socketInstance = io({
      path: "/ws/workflow",
      transports: ["websocket", "polling"],
    });

    socketInstance.on("connect", () => {
      console.log(`[WebSocket] Connected for workflow ${workflowId}`);
      setIsConnected(true);
      socketInstance.emit("join:workflow", workflowId);
    });

    socketInstance.on("disconnect", () => {
      console.log(`[WebSocket] Disconnected for workflow ${workflowId}`);
      setIsConnected(false);
    });

    socketInstance.on("workflow:event", (event: WorkflowExecutionEvent) => {
      if (event.workflowId === workflowId) {
        console.log("[WebSocket] Workflow event:", event);
        setEvents((prev) => [event, ...prev]);
      }
    });

    socketInstance.on("activity:update", (activity: ActivityUpdate) => {
      if (activity.workflowId === workflowId) {
        console.log("[WebSocket] Activity update:", activity);
        setActivities((prev) => [activity, ...prev]);
      }
    });

    setSocket(socketInstance);

    return () => {
      if (workflowId) {
        socketInstance.emit("leave:workflow", workflowId);
      }
      socketInstance.disconnect();
    };
  }, [workflowId]);

  return {
    isConnected,
    events,
    activities,
  };
}
