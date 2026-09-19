/**
 * Correlation ID Middleware
 * 
 * Generates and propagates correlation IDs across all service calls
 * for distributed tracing and debugging.
 */

import { v4 as uuidv4 } from "uuid";
import { AsyncLocalStorage } from "async_hooks";

// Correlation context stored in async local storage
interface CorrelationContext {
  correlationId: string;
  requestId: string;
  spanId: string;
  parentSpanId?: string;
  traceFlags?: number;
  userId?: string;
  tenantId?: string;
  startTime: number;
}

const correlationStorage = new AsyncLocalStorage<CorrelationContext>();

// Header names for correlation propagation
export const CORRELATION_HEADERS = {
  CORRELATION_ID: "x-correlation-id",
  REQUEST_ID: "x-request-id",
  SPAN_ID: "x-span-id",
  PARENT_SPAN_ID: "x-parent-span-id",
  TRACE_FLAGS: "x-trace-flags",
  USER_ID: "x-user-id",
  TENANT_ID: "x-tenant-id",
} as const;

/**
 * Generate a new correlation ID
 */
export function generateCorrelationId(): string {
  return uuidv4();
}

/**
 * Generate a new span ID (shorter than correlation ID)
 */
export function generateSpanId(): string {
  return uuidv4().replace(/-/g, "").substring(0, 16);
}

/**
 * Get current correlation context
 */
export function getCorrelationContext(): CorrelationContext | undefined {
  return correlationStorage.getStore();
}

/**
 * Get current correlation ID
 */
export function getCorrelationId(): string | undefined {
  return correlationStorage.getStore()?.correlationId;
}

/**
 * Get current request ID
 */
export function getRequestId(): string | undefined {
  return correlationStorage.getStore()?.requestId;
}

/**
 * Run a function with correlation context
 */
export function runWithCorrelation<T>(
  context: Partial<CorrelationContext>,
  fn: () => T
): T {
  const fullContext: CorrelationContext = {
    correlationId: context.correlationId || generateCorrelationId(),
    requestId: context.requestId || generateCorrelationId(),
    spanId: context.spanId || generateSpanId(),
    parentSpanId: context.parentSpanId,
    traceFlags: context.traceFlags,
    userId: context.userId,
    tenantId: context.tenantId,
    startTime: context.startTime || Date.now(),
  };

  return correlationStorage.run(fullContext, fn);
}

/**
 * Create a child span context
 */
export function createChildSpan(): CorrelationContext {
  const parent = getCorrelationContext();
  return {
    correlationId: parent?.correlationId || generateCorrelationId(),
    requestId: parent?.requestId || generateCorrelationId(),
    spanId: generateSpanId(),
    parentSpanId: parent?.spanId,
    traceFlags: parent?.traceFlags,
    userId: parent?.userId,
    tenantId: parent?.tenantId,
    startTime: Date.now(),
  };
}

/**
 * Extract correlation headers from incoming request
 */
export function extractCorrelationHeaders(
  headers: Record<string, string | string[] | undefined>
): Partial<CorrelationContext> {
  const getHeader = (name: string): string | undefined => {
    const value = headers[name] || headers[name.toLowerCase()];
    return Array.isArray(value) ? value[0] : value;
  };

  return {
    correlationId: getHeader(CORRELATION_HEADERS.CORRELATION_ID),
    requestId: getHeader(CORRELATION_HEADERS.REQUEST_ID),
    spanId: getHeader(CORRELATION_HEADERS.SPAN_ID),
    parentSpanId: getHeader(CORRELATION_HEADERS.PARENT_SPAN_ID),
    traceFlags: getHeader(CORRELATION_HEADERS.TRACE_FLAGS)
      ? parseInt(getHeader(CORRELATION_HEADERS.TRACE_FLAGS)!, 10)
      : undefined,
    userId: getHeader(CORRELATION_HEADERS.USER_ID),
    tenantId: getHeader(CORRELATION_HEADERS.TENANT_ID),
  };
}

/**
 * Create correlation headers for outgoing requests
 */
export function createCorrelationHeaders(): Record<string, string> {
  const context = getCorrelationContext();
  if (!context) {
    const newCorrelationId = generateCorrelationId();
    return {
      [CORRELATION_HEADERS.CORRELATION_ID]: newCorrelationId,
      [CORRELATION_HEADERS.REQUEST_ID]: generateCorrelationId(),
      [CORRELATION_HEADERS.SPAN_ID]: generateSpanId(),
    };
  }

  const headers: Record<string, string> = {
    [CORRELATION_HEADERS.CORRELATION_ID]: context.correlationId,
    [CORRELATION_HEADERS.REQUEST_ID]: context.requestId,
    [CORRELATION_HEADERS.SPAN_ID]: generateSpanId(),
    [CORRELATION_HEADERS.PARENT_SPAN_ID]: context.spanId,
  };

  if (context.traceFlags !== undefined) {
    headers[CORRELATION_HEADERS.TRACE_FLAGS] = context.traceFlags.toString();
  }
  if (context.userId) {
    headers[CORRELATION_HEADERS.USER_ID] = context.userId;
  }
  if (context.tenantId) {
    headers[CORRELATION_HEADERS.TENANT_ID] = context.tenantId;
  }

  return headers;
}

/**
 * Wrap fetch with correlation headers
 */
export async function correlatedFetch(
  url: string,
  options: RequestInit = {}
): Promise<Response> {
  const correlationHeaders = createCorrelationHeaders();
  const headers = new Headers(options.headers);
  
  for (const [key, value] of Object.entries(correlationHeaders)) {
    headers.set(key, value);
  }

  return fetch(url, {
    ...options,
    headers,
  });
}

/**
 * Log with correlation context
 */
export function correlatedLog(
  level: "debug" | "info" | "warn" | "error",
  message: string,
  data?: Record<string, unknown>
): void {
  const context = getCorrelationContext();
  const logData = {
    timestamp: new Date().toISOString(),
    level,
    message,
    correlationId: context?.correlationId,
    requestId: context?.requestId,
    spanId: context?.spanId,
    parentSpanId: context?.parentSpanId,
    userId: context?.userId,
    tenantId: context?.tenantId,
    durationMs: context ? Date.now() - context.startTime : undefined,
    ...data,
  };

  const logFn = console[level] || console.log;
  logFn(JSON.stringify(logData));
}

/**
 * Express/Connect middleware for correlation IDs
 */
export function correlationMiddleware() {
  return (
    req: { headers: Record<string, string | string[] | undefined> },
    res: { setHeader: (name: string, value: string) => void },
    next: () => void
  ) => {
    const extractedContext = extractCorrelationHeaders(req.headers);
    
    runWithCorrelation(extractedContext, () => {
      const context = getCorrelationContext()!;
      
      // Set response headers
      res.setHeader(CORRELATION_HEADERS.CORRELATION_ID, context.correlationId);
      res.setHeader(CORRELATION_HEADERS.REQUEST_ID, context.requestId);
      
      next();
    });
  };
}

/**
 * tRPC context with correlation
 */
export function createTRPCCorrelationContext(
  headers: Record<string, string | string[] | undefined>
): CorrelationContext {
  const extracted = extractCorrelationHeaders(headers);
  return {
    correlationId: extracted.correlationId || generateCorrelationId(),
    requestId: extracted.requestId || generateCorrelationId(),
    spanId: extracted.spanId || generateSpanId(),
    parentSpanId: extracted.parentSpanId,
    traceFlags: extracted.traceFlags,
    userId: extracted.userId,
    tenantId: extracted.tenantId,
    startTime: Date.now(),
  };
}

export default {
  generateCorrelationId,
  generateSpanId,
  getCorrelationContext,
  getCorrelationId,
  getRequestId,
  runWithCorrelation,
  createChildSpan,
  extractCorrelationHeaders,
  createCorrelationHeaders,
  correlatedFetch,
  correlatedLog,
  correlationMiddleware,
  createTRPCCorrelationContext,
  CORRELATION_HEADERS,
};
