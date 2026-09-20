import { getDb } from "../db";
import { middlewareMetrics, middlewareAlerts, middlewareHealthChecks } from "../../drizzle/schema";
import { desc, eq, and, gte } from "drizzle-orm";
import { createClient, RedisClientType } from "redis";
import { Kafka } from "kafkajs";

const REDIS_URL = process.env.REDIS_URL;
const APISIX_ADMIN_URL = process.env.APISIX_ADMIN_URL || "http://localhost:9180";
const KEYCLOAK_URL = process.env.KEYCLOAK_URL || "http://localhost:8080";
const PERMIFY_URL = process.env.PERMIFY_URL || "http://localhost:3476";
const DAPR_HTTP_PORT = process.env.DAPR_HTTP_PORT || "3500";
const TEMPORAL_URL = process.env.TEMPORAL_URL || "http://localhost:7233";
const FLUVIO_URL = process.env.FLUVIO_URL || "http://localhost:9003";
const KAFKA_BROKERS = process.env.KAFKA_BROKERS || "localhost:9092";

let redisClient: RedisClientType | null = null;

async function getRedisClient(): Promise<RedisClientType | null> {
  if (!REDIS_URL) return null;
  if (!redisClient) {
    try {
      redisClient = createClient({ url: REDIS_URL });
      await redisClient.connect();
    } catch (error) {
      console.error("[Monitoring] Failed to connect to Redis:", error);
      return null;
    }
  }
  return redisClient;
}

async function redisHealthCheck(): Promise<boolean> {
  try {
    const client = await getRedisClient();
    if (!client) return false;
    const pong = await client.ping();
    return pong === "PONG";
  } catch (error) {
    console.error("[Monitoring] Redis health check failed:", error);
    return false;
  }
}

async function apisixHealthCheck(): Promise<boolean> {
  try {
    const apiKey = process.env.APISIX_ADMIN_KEY;
    if (!apiKey) {
      console.warn("[Monitoring] APISIX_ADMIN_KEY not configured, health check unavailable");
      return false;
    }
    const response = await fetch(`${APISIX_ADMIN_URL}/apisix/admin/routes`, {
      method: "GET",
      headers: { "X-API-KEY": apiKey },
      signal: AbortSignal.timeout(5000),
    });
    return response.ok;
  } catch (error) {
    console.error("[Monitoring] APISIX health check failed:", error);
    return false;
  }
}

async function keycloakHealthCheck(): Promise<boolean> {
  try {
    const response = await fetch(`${KEYCLOAK_URL}/health/ready`, {
      method: "GET",
      signal: AbortSignal.timeout(5000),
    });
    return response.ok;
  } catch (error) {
    console.error("[Monitoring] Keycloak health check failed:", error);
    return false;
  }
}

async function permifyHealthCheck(): Promise<boolean> {
  try {
    const response = await fetch(`${PERMIFY_URL}/healthz`, {
      method: "GET",
      signal: AbortSignal.timeout(5000),
    });
    return response.ok;
  } catch (error) {
    console.error("[Monitoring] Permify health check failed:", error);
    return false;
  }
}

async function daprHealthCheck(): Promise<boolean> {
  try {
    const response = await fetch(`http://localhost:${DAPR_HTTP_PORT}/v1.0/healthz`, {
      method: "GET",
      signal: AbortSignal.timeout(5000),
    });
    return response.ok;
  } catch (error) {
    console.error("[Monitoring] Dapr health check failed:", error);
    return false;
  }
}

async function temporalHealthCheck(): Promise<boolean> {
  try {
    const response = await fetch(`${TEMPORAL_URL}/api/v1/namespaces`, {
      method: "GET",
      signal: AbortSignal.timeout(5000),
    });
    return response.ok || response.status === 401;
  } catch (error) {
    console.error("[Monitoring] Temporal health check failed:", error);
    return false;
  }
}

async function fluvioHealthCheck(): Promise<boolean> {
  try {
    const response = await fetch(`${FLUVIO_URL}/api/v1/health`, {
      method: "GET",
      signal: AbortSignal.timeout(5000),
    });
    return response.ok;
  } catch (error) {
    console.error("[Monitoring] Fluvio health check failed:", error);
    return false;
  }
}

async function kafkaHealthCheck(): Promise<boolean> {
  try {
    const brokers = KAFKA_BROKERS.split(",");
    const admin = new Kafka({ clientId: "middleware-health", brokers, connectionTimeout: 3000 }).admin();
    await admin.connect();
    await admin.listTopics();
    await admin.disconnect();
    return true;
  } catch (error) {
    console.error("[Monitoring] Kafka health check failed:", error);
    return false;
  }
}

/**
 * Middleware Monitoring Service
 * Unified monitoring for all 8 middleware components
 */

export type MiddlewareComponent =
  | "redis"
  | "apisix"
  | "kafka"
  | "fluvio"
  | "keycloak"
  | "permify"
  | "dapr"
  | "temporal";

export type HealthStatus = "healthy" | "degraded" | "unhealthy";

export interface MiddlewareHealth {
  component: MiddlewareComponent;
  status: HealthStatus;
  responseTime: number;
  errorMessage?: string;
  checkedAt: Date;
}

export interface MiddlewareMetric {
  component: MiddlewareComponent;
  metricType: string;
  metricValue: number;
  unit: string;
  timestamp: Date;
  metadata?: any;
}

export interface MiddlewareAlert {
  id: number;
  component: MiddlewareComponent;
  alertType: string;
  severity: "info" | "warning" | "critical";
  message: string;
  details?: any;
  status: "active" | "acknowledged" | "resolved";
  triggeredAt: Date;
}

/**
 * Record metric
 */
export async function recordMetric(metric: Omit<MiddlewareMetric, "timestamp">): Promise<void> {
  const db = await getDb();
  if (!db) return;

  try {
    await db.insert(middlewareMetrics).values({
      component: metric.component,
      metricType: metric.metricType,
      metricValue: metric.metricValue,
      unit: metric.unit,
      metadata: metric.metadata ? JSON.stringify(metric.metadata) : null,
    });
  } catch (error) {
    console.error("[Monitoring] Failed to record metric:", error);
  }
}

/**
 * Get recent metrics
 */
export async function getRecentMetrics(
  component: MiddlewareComponent,
  metricType: string,
  minutes: number = 60
): Promise<MiddlewareMetric[]> {
  const db = await getDb();
  if (!db) return [];

  try {
    const since = new Date(Date.now() - minutes * 60 * 1000);

    const results = await db
      .select()
      .from(middlewareMetrics)
      .where(
        and(
          eq(middlewareMetrics.component, component),
          eq(middlewareMetrics.metricType, metricType),
          gte(middlewareMetrics.timestamp, since)
        )
      )
      .orderBy(desc(middlewareMetrics.timestamp))
      .limit(1000);

    return results.map((r) => ({
      component: r.component as MiddlewareComponent,
      metricType: r.metricType,
      metricValue: r.metricValue,
      unit: r.unit || "",
      timestamp: r.timestamp,
      metadata: r.metadata ? JSON.parse(r.metadata) : undefined,
    }));
  } catch (error) {
    console.error("[Monitoring] Failed to get metrics:", error);
    return [];
  }
}

/**
 * Create alert
 */
export async function createAlert(
  alert: Omit<MiddlewareAlert, "id" | "status" | "triggeredAt">
): Promise<number | null> {
  const db = await getDb();
  if (!db) return null;

  try {
    const result = await db.insert(middlewareAlerts).values({
      component: alert.component,
      alertType: alert.alertType,
      severity: alert.severity,
      message: alert.message,
      details: alert.details ? JSON.stringify(alert.details) : null,
    });

    return result[0].insertId;
  } catch (error) {
    console.error("[Monitoring] Failed to create alert:", error);
    return null;
  }
}

/**
 * Get active alerts
 */
export async function getActiveAlerts(): Promise<MiddlewareAlert[]> {
  const db = await getDb();
  if (!db) return [];

  try {
    const results = await db
      .select()
      .from(middlewareAlerts)
      .where(eq(middlewareAlerts.status, "active"))
      .orderBy(desc(middlewareAlerts.triggeredAt))
      .limit(100);

    return results.map((r) => ({
      id: r.id,
      component: r.component as MiddlewareComponent,
      alertType: r.alertType,
      severity: r.severity,
      message: r.message,
      details: r.details ? JSON.parse(r.details) : undefined,
      status: r.status,
      triggeredAt: r.triggeredAt,
    }));
  } catch (error) {
    console.error("[Monitoring] Failed to get alerts:", error);
    return [];
  }
}

/**
 * Acknowledge alert
 */
export async function acknowledgeAlert(alertId: number, userId: number): Promise<void> {
  const db = await getDb();
  if (!db) return;

  try {
    await db
      .update(middlewareAlerts)
      .set({
        status: "acknowledged",
        acknowledgedAt: new Date(),
        acknowledgedBy: userId,
      })
      .where(eq(middlewareAlerts.id, alertId));
  } catch (error) {
    console.error("[Monitoring] Failed to acknowledge alert:", error);
  }
}

/**
 * Resolve alert
 */
export async function resolveAlert(alertId: number, userId: number): Promise<void> {
  const db = await getDb();
  if (!db) return;

  try {
    await db
      .update(middlewareAlerts)
      .set({
        status: "resolved",
        resolvedAt: new Date(),
        resolvedBy: userId,
      })
      .where(eq(middlewareAlerts.id, alertId));
  } catch (error) {
    console.error("[Monitoring] Failed to resolve alert:", error);
  }
}

/**
 * Perform health check for component
 */
async function checkComponentHealth(component: MiddlewareComponent): Promise<MiddlewareHealth> {
  const startTime = Date.now();
  let status: HealthStatus = "healthy";
  let errorMessage: string | undefined;

  try {
    let isHealthy = false;

    switch (component) {
      case "redis":
        isHealthy = await redisHealthCheck();
        break;
      case "apisix":
        isHealthy = await apisixHealthCheck();
        break;
      case "kafka":
        isHealthy = await kafkaHealthCheck();
        break;
      case "fluvio":
        isHealthy = await fluvioHealthCheck();
        break;
      case "keycloak":
        isHealthy = await keycloakHealthCheck();
        break;
      case "permify":
        isHealthy = await permifyHealthCheck();
        break;
      case "dapr":
        isHealthy = await daprHealthCheck();
        break;
      case "temporal":
        isHealthy = await temporalHealthCheck();
        break;
    }

    status = isHealthy ? "healthy" : "unhealthy";
  } catch (error: any) {
    status = "unhealthy";
    errorMessage = error.message;
  }

  const responseTime = Date.now() - startTime;

  return {
    component,
    status,
    responseTime,
    errorMessage,
    checkedAt: new Date(),
  };
}

/**
 * Record health check
 */
export async function recordHealthCheck(health: MiddlewareHealth): Promise<void> {
  const db = await getDb();
  if (!db) return;

  try {
    await db.insert(middlewareHealthChecks).values({
      component: health.component,
      status: health.status,
      responseTime: health.responseTime,
      errorMessage: health.errorMessage || null,
    });

    // Create alert if unhealthy
    if (health.status === "unhealthy") {
      await createAlert({
        component: health.component,
        alertType: "health",
        severity: "critical",
        message: `${health.component} is unhealthy`,
        details: { errorMessage: health.errorMessage, responseTime: health.responseTime },
      });
    }
  } catch (error) {
    console.error("[Monitoring] Failed to record health check:", error);
  }
}

/**
 * Check all middleware health
 */
export async function checkAllMiddlewareHealth(): Promise<MiddlewareHealth[]> {
  const components: MiddlewareComponent[] = [
    "redis",
    "apisix",
    "kafka",
    "fluvio",
    "keycloak",
    "permify",
    "dapr",
    "temporal",
  ];

  const healthChecks = await Promise.all(components.map((c) => checkComponentHealth(c)));

  // Record all health checks
  await Promise.all(healthChecks.map((h) => recordHealthCheck(h)));

  return healthChecks;
}

/**
 * Get latest health status
 */
export async function getLatestHealthStatus(): Promise<Record<MiddlewareComponent, MiddlewareHealth>> {
  const db = await getDb();
  if (!db) {
    // Return default unhealthy status
    const components: MiddlewareComponent[] = [
      "redis",
      "apisix",
      "kafka",
      "fluvio",
      "keycloak",
      "permify",
      "dapr",
      "temporal",
    ];
    return Object.fromEntries(
      components.map((c) => [
        c,
        {
          component: c,
          status: "unhealthy" as HealthStatus,
          responseTime: 0,
          checkedAt: new Date(),
        },
      ])
    ) as Record<MiddlewareComponent, MiddlewareHealth>;
  }

  try {
    const components: MiddlewareComponent[] = [
      "redis",
      "apisix",
      "kafka",
      "fluvio",
      "keycloak",
      "permify",
      "dapr",
      "temporal",
    ];

    const healthStatus: Record<string, MiddlewareHealth> = {};

    for (const component of components) {
      const results = await db
        .select()
        .from(middlewareHealthChecks)
        .where(eq(middlewareHealthChecks.component, component))
        .orderBy(desc(middlewareHealthChecks.checkedAt))
        .limit(1);

      if (results.length > 0) {
        const r = results[0];
        healthStatus[component] = {
          component: r.component as MiddlewareComponent,
          status: r.status,
          responseTime: r.responseTime || 0,
          errorMessage: r.errorMessage || undefined,
          checkedAt: r.checkedAt,
        };
      } else {
        healthStatus[component] = {
          component,
          status: "unhealthy",
          responseTime: 0,
          checkedAt: new Date(),
        };
      }
    }

    return healthStatus as Record<MiddlewareComponent, MiddlewareHealth>;
  } catch (error) {
    console.error("[Monitoring] Failed to get health status:", error);
    return {} as Record<MiddlewareComponent, MiddlewareHealth>;
  }
}

/**
 * Collect Redis metrics
 */
async function collectRedisMetrics(): Promise<void> {
  try {
    const client: any = getRedisClient();
    if (!client) return;

    const info: any = await client.info("memory");
    const lines = info.split("\r\n");
    const memoryUsed = lines.find((l: string) => l.startsWith("used_memory:"));

    if (memoryUsed) {
      const bytes = parseInt(memoryUsed.split(":")[1]);
      const mb = bytes / (1024 * 1024);

      await recordMetric({
        component: "redis",
        metricType: "memory_usage",
        metricValue: mb,
        unit: "MB",
      });
    }

    // Get connected clients
    const clients = await client.info("clients");
    const connectedClients = clients.split("\r\n").find((l: string) => l.startsWith("connected_clients:"));

    if (connectedClients) {
      const count = parseInt(connectedClients.split(":")[1]);
      await recordMetric({
        component: "redis",
        metricType: "connected_clients",
        metricValue: count,
        unit: "count",
      });
    }
  } catch (error) {
    console.error("[Monitoring] Failed to collect Redis metrics:", error);
  }
}

/**
 * Start metrics collection
 */
export function startMetricsCollection(): void {
  // Health checks every 30 seconds
  setInterval(async () => {
    await checkAllMiddlewareHealth();
  }, 30000);

  // Metrics collection every 60 seconds
  setInterval(async () => {
    await collectRedisMetrics();
    // Add other metric collectors here
  }, 60000);

  console.log("[Monitoring] Metrics collection started");
}

// Auto-start metrics collection
startMetricsCollection();
