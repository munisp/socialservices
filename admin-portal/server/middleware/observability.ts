/**
 * Observability Middleware
 * Prometheus metrics, distributed tracing, and structured logging
 */

// ============================================================================
// Prometheus Metrics
// ============================================================================

interface MetricLabels {
  [key: string]: string;
}

interface HistogramBuckets {
  buckets: number[];
}

// Metric types
type MetricType = "counter" | "gauge" | "histogram" | "summary";

interface MetricDefinition {
  name: string;
  help: string;
  type: MetricType;
  labels?: string[];
  buckets?: number[];
}

// In-memory metrics storage
const counters = new Map<string, Map<string, number>>();
const gauges = new Map<string, Map<string, number>>();
const histograms = new Map<string, Map<string, { sum: number; count: number; buckets: Map<number, number> }>>();

// Default histogram buckets for latency (in seconds)
const DEFAULT_LATENCY_BUCKETS = [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10];

// Metric definitions
const METRIC_DEFINITIONS: MetricDefinition[] = [
  // HTTP metrics
  {
    name: "http_requests_total",
    help: "Total number of HTTP requests",
    type: "counter",
    labels: ["method", "path", "status"],
  },
  {
    name: "http_request_duration_seconds",
    help: "HTTP request duration in seconds",
    type: "histogram",
    labels: ["method", "path", "status"],
    buckets: DEFAULT_LATENCY_BUCKETS,
  },
  {
    name: "http_requests_in_flight",
    help: "Number of HTTP requests currently being processed",
    type: "gauge",
    labels: ["method"],
  },
  
  // Database metrics
  {
    name: "db_queries_total",
    help: "Total number of database queries",
    type: "counter",
    labels: ["operation", "table", "status"],
  },
  {
    name: "db_query_duration_seconds",
    help: "Database query duration in seconds",
    type: "histogram",
    labels: ["operation", "table"],
    buckets: DEFAULT_LATENCY_BUCKETS,
  },
  {
    name: "db_connections_active",
    help: "Number of active database connections",
    type: "gauge",
    labels: ["pool"],
  },
  
  // Cache metrics
  {
    name: "cache_hits_total",
    help: "Total number of cache hits",
    type: "counter",
    labels: ["cache"],
  },
  {
    name: "cache_misses_total",
    help: "Total number of cache misses",
    type: "counter",
    labels: ["cache"],
  },
  {
    name: "cache_operation_duration_seconds",
    help: "Cache operation duration in seconds",
    type: "histogram",
    labels: ["cache", "operation"],
    buckets: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25],
  },
  
  // Message queue metrics
  {
    name: "mq_messages_produced_total",
    help: "Total number of messages produced",
    type: "counter",
    labels: ["topic"],
  },
  {
    name: "mq_messages_consumed_total",
    help: "Total number of messages consumed",
    type: "counter",
    labels: ["topic", "consumer_group"],
  },
  {
    name: "mq_consumer_lag",
    help: "Consumer lag (messages behind)",
    type: "gauge",
    labels: ["topic", "consumer_group", "partition"],
  },
  
  // Workflow metrics
  {
    name: "workflow_executions_total",
    help: "Total number of workflow executions",
    type: "counter",
    labels: ["workflow", "status"],
  },
  {
    name: "workflow_duration_seconds",
    help: "Workflow execution duration in seconds",
    type: "histogram",
    labels: ["workflow"],
    buckets: [1, 5, 10, 30, 60, 120, 300, 600],
  },
  {
    name: "workflow_activities_total",
    help: "Total number of workflow activities",
    type: "counter",
    labels: ["workflow", "activity", "status"],
  },
  
  // Business metrics
  {
    name: "beneficiaries_total",
    help: "Total number of beneficiaries",
    type: "gauge",
    labels: ["status"],
  },
  {
    name: "disbursements_total",
    help: "Total number of disbursements",
    type: "counter",
    labels: ["program", "status"],
  },
  {
    name: "disbursement_amount_total",
    help: "Total disbursement amount",
    type: "counter",
    labels: ["program", "currency"],
  },
  {
    name: "enrollments_total",
    help: "Total number of enrollments",
    type: "counter",
    labels: ["program", "status"],
  },
  
  // Circuit breaker metrics
  {
    name: "circuit_breaker_state",
    help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
    type: "gauge",
    labels: ["service"],
  },
  {
    name: "circuit_breaker_failures_total",
    help: "Total circuit breaker failures",
    type: "counter",
    labels: ["service"],
  },
  
  // Rate limiting metrics
  {
    name: "rate_limit_exceeded_total",
    help: "Total number of rate limit exceeded events",
    type: "counter",
    labels: ["endpoint", "limit_type"],
  },
  
  // Authentication metrics
  {
    name: "auth_attempts_total",
    help: "Total authentication attempts",
    type: "counter",
    labels: ["method", "status"],
  },
  {
    name: "active_sessions",
    help: "Number of active user sessions",
    type: "gauge",
    labels: [],
  },
];

/**
 * Generate label key from labels object
 */
function getLabelKey(labels: MetricLabels): string {
  return Object.entries(labels)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k}="${v}"`)
    .join(",");
}

/**
 * Increment counter
 */
export function incrementCounter(name: string, labels: MetricLabels = {}, value: number = 1): void {
  if (!counters.has(name)) {
    counters.set(name, new Map());
  }
  const metric = counters.get(name)!;
  const key = getLabelKey(labels);
  metric.set(key, (metric.get(key) || 0) + value);
}

/**
 * Set gauge value
 */
export function setGauge(name: string, labels: MetricLabels = {}, value: number): void {
  if (!gauges.has(name)) {
    gauges.set(name, new Map());
  }
  const metric = gauges.get(name)!;
  const key = getLabelKey(labels);
  metric.set(key, value);
}

/**
 * Increment gauge
 */
export function incrementGauge(name: string, labels: MetricLabels = {}, value: number = 1): void {
  if (!gauges.has(name)) {
    gauges.set(name, new Map());
  }
  const metric = gauges.get(name)!;
  const key = getLabelKey(labels);
  metric.set(key, (metric.get(key) || 0) + value);
}

/**
 * Decrement gauge
 */
export function decrementGauge(name: string, labels: MetricLabels = {}, value: number = 1): void {
  incrementGauge(name, labels, -value);
}

/**
 * Observe histogram value
 */
export function observeHistogram(
  name: string,
  labels: MetricLabels = {},
  value: number,
  buckets: number[] = DEFAULT_LATENCY_BUCKETS
): void {
  if (!histograms.has(name)) {
    histograms.set(name, new Map());
  }
  const metric = histograms.get(name)!;
  const key = getLabelKey(labels);
  
  if (!metric.has(key)) {
    const bucketMap = new Map<number, number>();
    buckets.forEach(b => bucketMap.set(b, 0));
    bucketMap.set(Infinity, 0);
    metric.set(key, { sum: 0, count: 0, buckets: bucketMap });
  }
  
  const data = metric.get(key)!;
  data.sum += value;
  data.count += 1;
  
  for (const [bucket, count] of data.buckets) {
    if (value <= bucket) {
      data.buckets.set(bucket, count + 1);
    }
  }
}

/**
 * Create timer for measuring duration
 */
export function startTimer(): () => number {
  const start = process.hrtime.bigint();
  return () => {
    const end = process.hrtime.bigint();
    return Number(end - start) / 1e9; // Convert to seconds
  };
}

/**
 * Generate Prometheus metrics output
 */
export function getPrometheusMetrics(): string {
  const lines: string[] = [];
  
  // Add metric definitions and values
  for (const def of METRIC_DEFINITIONS) {
    lines.push(`# HELP ${def.name} ${def.help}`);
    lines.push(`# TYPE ${def.name} ${def.type}`);
    
    if (def.type === "counter") {
      const metric = counters.get(def.name);
      if (metric) {
        for (const [labels, value] of metric) {
          const labelStr = labels ? `{${labels}}` : "";
          lines.push(`${def.name}${labelStr} ${value}`);
        }
      }
    } else if (def.type === "gauge") {
      const metric = gauges.get(def.name);
      if (metric) {
        for (const [labels, value] of metric) {
          const labelStr = labels ? `{${labels}}` : "";
          lines.push(`${def.name}${labelStr} ${value}`);
        }
      }
    } else if (def.type === "histogram") {
      const metric = histograms.get(def.name);
      if (metric) {
        for (const [labels, data] of metric) {
          const labelStr = labels ? `,${labels}` : "";
          
          // Output bucket values
          for (const [bucket, count] of data.buckets) {
            const le = bucket === Infinity ? "+Inf" : bucket.toString();
            lines.push(`${def.name}_bucket{le="${le}"${labelStr}} ${count}`);
          }
          
          lines.push(`${def.name}_sum{${labels}} ${data.sum}`);
          lines.push(`${def.name}_count{${labels}} ${data.count}`);
        }
      }
    }
    
    lines.push("");
  }
  
  return lines.join("\n");
}

/**
 * Metrics endpoint handler
 */
export function metricsHandler(req: any, res: any): void {
  res.setHeader("Content-Type", "text/plain; version=0.0.4; charset=utf-8");
  res.send(getPrometheusMetrics());
}

// ============================================================================
// Distributed Tracing
// ============================================================================

interface TraceContext {
  traceId: string;
  spanId: string;
  parentSpanId?: string;
  sampled: boolean;
  baggage: Map<string, string>;
}

interface Span {
  traceId: string;
  spanId: string;
  parentSpanId?: string;
  operationName: string;
  startTime: bigint;
  endTime?: bigint;
  tags: Map<string, string | number | boolean>;
  logs: Array<{ timestamp: bigint; fields: Record<string, any> }>;
  status: "ok" | "error";
}

// Active spans storage
const activeSpans = new Map<string, Span>();
const completedSpans: Span[] = [];
const MAX_COMPLETED_SPANS = 10000;

/**
 * Generate trace ID
 */
function generateTraceId(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes).map(b => b.toString(16).padStart(2, "0")).join("");
}

/**
 * Generate span ID
 */
function generateSpanId(): string {
  const bytes = new Uint8Array(8);
  crypto.getRandomValues(bytes);
  return Array.from(bytes).map(b => b.toString(16).padStart(2, "0")).join("");
}

/**
 * Extract trace context from headers
 */
export function extractTraceContext(headers: Record<string, string | string[] | undefined>): TraceContext | null {
  // Support W3C Trace Context format
  const traceparent = headers["traceparent"] as string;
  
  if (traceparent) {
    const parts = traceparent.split("-");
    if (parts.length === 4) {
      return {
        traceId: parts[1],
        spanId: generateSpanId(),
        parentSpanId: parts[2],
        sampled: parts[3] === "01",
        baggage: new Map(),
      };
    }
  }
  
  // Support B3 format (Zipkin)
  const b3TraceId = headers["x-b3-traceid"] as string;
  const b3SpanId = headers["x-b3-spanid"] as string;
  const b3Sampled = headers["x-b3-sampled"] as string;
  
  if (b3TraceId) {
    return {
      traceId: b3TraceId,
      spanId: generateSpanId(),
      parentSpanId: b3SpanId,
      sampled: b3Sampled === "1",
      baggage: new Map(),
    };
  }
  
  return null;
}

/**
 * Inject trace context into headers
 */
export function injectTraceContext(context: TraceContext, headers: Record<string, string>): void {
  // W3C Trace Context format
  const sampled = context.sampled ? "01" : "00";
  headers["traceparent"] = `00-${context.traceId}-${context.spanId}-${sampled}`;
  
  // B3 format for compatibility
  headers["x-b3-traceid"] = context.traceId;
  headers["x-b3-spanid"] = context.spanId;
  if (context.parentSpanId) {
    headers["x-b3-parentspanid"] = context.parentSpanId;
  }
  headers["x-b3-sampled"] = context.sampled ? "1" : "0";
}

/**
 * Start a new span
 */
export function startSpan(
  operationName: string,
  parentContext?: TraceContext
): { span: Span; context: TraceContext } {
  const traceId = parentContext?.traceId || generateTraceId();
  const spanId = generateSpanId();
  
  const span: Span = {
    traceId,
    spanId,
    parentSpanId: parentContext?.spanId,
    operationName,
    startTime: process.hrtime.bigint(),
    tags: new Map(),
    logs: [],
    status: "ok",
  };
  
  const context: TraceContext = {
    traceId,
    spanId,
    parentSpanId: parentContext?.spanId,
    sampled: parentContext?.sampled ?? Math.random() < 0.1, // 10% sampling by default
    baggage: parentContext?.baggage || new Map(),
  };
  
  activeSpans.set(spanId, span);
  
  return { span, context };
}

/**
 * Add tag to span
 */
export function setSpanTag(span: Span, key: string, value: string | number | boolean): void {
  span.tags.set(key, value);
}

/**
 * Add log to span
 */
export function logSpanEvent(span: Span, fields: Record<string, any>): void {
  span.logs.push({
    timestamp: process.hrtime.bigint(),
    fields,
  });
}

/**
 * End span
 */
export function endSpan(span: Span, status: "ok" | "error" = "ok"): void {
  span.endTime = process.hrtime.bigint();
  span.status = status;
  
  activeSpans.delete(span.spanId);
  completedSpans.push(span);
  
  // Trim completed spans if too many
  if (completedSpans.length > MAX_COMPLETED_SPANS) {
    completedSpans.splice(0, completedSpans.length - MAX_COMPLETED_SPANS);
  }
}

/**
 * Get span duration in milliseconds
 */
export function getSpanDuration(span: Span): number {
  if (!span.endTime) {
    return Number(process.hrtime.bigint() - span.startTime) / 1e6;
  }
  return Number(span.endTime - span.startTime) / 1e6;
}

// ============================================================================
// Structured Logging
// ============================================================================

type LogLevel = "debug" | "info" | "warn" | "error";

interface LogEntry {
  timestamp: string;
  level: LogLevel;
  message: string;
  traceId?: string;
  spanId?: string;
  service: string;
  [key: string]: any;
}

const SERVICE_NAME = process.env.SERVICE_NAME || "social-protection-platform";

/**
 * Create structured log entry
 */
function createLogEntry(
  level: LogLevel,
  message: string,
  context?: TraceContext,
  extra?: Record<string, any>
): LogEntry {
  return {
    timestamp: new Date().toISOString(),
    level,
    message,
    service: SERVICE_NAME,
    traceId: context?.traceId,
    spanId: context?.spanId,
    ...extra,
  };
}

/**
 * Log with structured format
 */
export function log(
  level: LogLevel,
  message: string,
  context?: TraceContext,
  extra?: Record<string, any>
): void {
  const entry = createLogEntry(level, message, context, extra);
  const output = JSON.stringify(entry);
  
  switch (level) {
    case "debug":
      console.debug(output);
      break;
    case "info":
      console.info(output);
      break;
    case "warn":
      console.warn(output);
      break;
    case "error":
      console.error(output);
      break;
  }
}

/**
 * Convenience logging functions
 */
export const logger = {
  debug: (message: string, context?: TraceContext, extra?: Record<string, any>) =>
    log("debug", message, context, extra),
  info: (message: string, context?: TraceContext, extra?: Record<string, any>) =>
    log("info", message, context, extra),
  warn: (message: string, context?: TraceContext, extra?: Record<string, any>) =>
    log("warn", message, context, extra),
  error: (message: string, context?: TraceContext, extra?: Record<string, any>) =>
    log("error", message, context, extra),
};

// ============================================================================
// Middleware
// ============================================================================

/**
 * Express middleware for observability
 */
export function observabilityMiddleware() {
  return (req: any, res: any, next: () => void) => {
    // Extract or create trace context
    const parentContext = extractTraceContext(req.headers);
    const { span, context } = startSpan(`${req.method} ${req.path}`, parentContext || undefined);
    
    // Attach context to request
    req.traceContext = context;
    req.span = span;
    
    // Set standard tags
    setSpanTag(span, "http.method", req.method);
    setSpanTag(span, "http.url", req.url);
    setSpanTag(span, "http.user_agent", req.headers["user-agent"] || "unknown");
    
    // Increment in-flight gauge
    incrementGauge("http_requests_in_flight", { method: req.method });
    
    // Start timer
    const timer = startTimer();
    
    // Capture response
    const originalEnd = res.end;
    res.end = function (...args: any[]) {
      const duration = timer();
      
      // Record metrics
      const labels = {
        method: req.method,
        path: req.route?.path || req.path,
        status: res.statusCode.toString(),
      };
      
      incrementCounter("http_requests_total", labels);
      observeHistogram("http_request_duration_seconds", labels, duration);
      decrementGauge("http_requests_in_flight", { method: req.method });
      
      // Complete span
      setSpanTag(span, "http.status_code", res.statusCode);
      endSpan(span, res.statusCode >= 400 ? "error" : "ok");
      
      // Log request
      logger.info("HTTP request completed", context, {
        method: req.method,
        path: req.path,
        status: res.statusCode,
        duration: duration * 1000, // ms
      });
      
      return originalEnd.apply(this, args);
    };
    
    // Inject trace headers into response
    res.setHeader("x-trace-id", context.traceId);
    
    next();
  };
}

/**
 * tRPC middleware for observability
 */
export function createObservabilityMiddleware() {
  return async ({ ctx, path, type, next }: { ctx: any; path: string; type: string; next: () => Promise<any> }) => {
    const parentContext = ctx.traceContext;
    const { span, context } = startSpan(`trpc.${type}.${path}`, parentContext);
    
    setSpanTag(span, "rpc.system", "trpc");
    setSpanTag(span, "rpc.method", path);
    setSpanTag(span, "rpc.type", type);
    
    const timer = startTimer();
    
    try {
      const result = await next();
      
      const duration = timer();
      observeHistogram("http_request_duration_seconds", {
        method: "POST",
        path: `/trpc/${path}`,
        status: "200",
      }, duration);
      
      endSpan(span, "ok");
      return result;
    } catch (error) {
      const duration = timer();
      observeHistogram("http_request_duration_seconds", {
        method: "POST",
        path: `/trpc/${path}`,
        status: "500",
      }, duration);
      
      setSpanTag(span, "error", true);
      logSpanEvent(span, { event: "error", message: error instanceof Error ? error.message : "Unknown error" });
      endSpan(span, "error");
      
      throw error;
    }
  };
}

// Import crypto for browser compatibility
import crypto from "crypto";
