/**
 * Idempotency Middleware
 * Ensures payment and critical operations are executed exactly once
 */

import { TRPCError } from "@trpc/server";
import crypto from "crypto";

interface IdempotencyRecord {
  key: string;
  status: "processing" | "completed" | "failed";
  response?: any;
  error?: string;
  createdAt: number;
  completedAt?: number;
  expiresAt: number;
  requestHash: string;
}

interface IdempotencyConfig {
  ttlMs: number;           // Time to live for idempotency records
  lockTimeoutMs: number;   // How long to wait for a lock
  keyPrefix: string;       // Prefix for storage keys
}

// Default configurations
const DEFAULT_CONFIG: IdempotencyConfig = {
  ttlMs: 24 * 60 * 60 * 1000, // 24 hours
  lockTimeoutMs: 30000,        // 30 seconds
  keyPrefix: "idempotency",
};

// In-memory store (fallback when Redis unavailable)
const memoryStore = new Map<string, IdempotencyRecord>();

/**
 * Generate a hash of the request body for validation
 */
function hashRequest(body: any): string {
  const normalized = JSON.stringify(body, Object.keys(body).sort());
  return crypto.createHash("sha256").update(normalized).digest("hex");
}

/**
 * Generate idempotency key from user and operation
 */
export function generateIdempotencyKey(
  userId: string,
  operation: string,
  resourceId?: string
): string {
  const parts = [userId, operation];
  if (resourceId) {
    parts.push(resourceId);
  }
  parts.push(Date.now().toString());
  parts.push(crypto.randomBytes(8).toString("hex"));
  return parts.join(":");
}

/**
 * Validate idempotency key format
 */
export function validateIdempotencyKey(key: string): boolean {
  // Key should be a non-empty string, max 256 characters
  if (!key || typeof key !== "string" || key.length > 256) {
    return false;
  }
  // Key should only contain alphanumeric, hyphens, underscores, colons
  return /^[a-zA-Z0-9\-_:]+$/.test(key);
}

/**
 * Get idempotency record from memory store
 */
function getFromMemory(key: string): IdempotencyRecord | null {
  const record = memoryStore.get(key);
  if (!record) {
    return null;
  }
  // Check if expired
  if (Date.now() > record.expiresAt) {
    memoryStore.delete(key);
    return null;
  }
  return record;
}

/**
 * Save idempotency record to memory store
 */
function saveToMemory(record: IdempotencyRecord): void {
  memoryStore.set(record.key, record);
}

/**
 * Get idempotency record from Redis
 */
async function getFromRedis(
  key: string,
  redisClient: any,
  config: IdempotencyConfig
): Promise<IdempotencyRecord | null> {
  try {
    const data = await redisClient.get(`${config.keyPrefix}:${key}`);
    if (!data) {
      return null;
    }
    return JSON.parse(data);
  } catch (error) {
    console.error("[Idempotency] Redis get error:", error);
    return getFromMemory(key);
  }
}

/**
 * Save idempotency record to Redis
 */
async function saveToRedis(
  record: IdempotencyRecord,
  redisClient: any,
  config: IdempotencyConfig
): Promise<void> {
  try {
    const ttlSeconds = Math.ceil((record.expiresAt - Date.now()) / 1000);
    await redisClient.setex(
      `${config.keyPrefix}:${record.key}`,
      ttlSeconds,
      JSON.stringify(record)
    );
  } catch (error) {
    console.error("[Idempotency] Redis save error:", error);
    saveToMemory(record);
  }
}

/**
 * Acquire lock for idempotency key
 */
async function acquireLock(
  key: string,
  redisClient: any | null,
  config: IdempotencyConfig
): Promise<boolean> {
  const lockKey = `lock:${config.keyPrefix}:${key}`;
  const lockValue = crypto.randomBytes(16).toString("hex");
  const lockTtl = Math.ceil(config.lockTimeoutMs / 1000);

  if (redisClient) {
    try {
      const result = await redisClient.set(lockKey, lockValue, "NX", "EX", lockTtl);
      return result === "OK";
    } catch (error) {
      console.error("[Idempotency] Lock acquisition error:", error);
    }
  }

  // Fallback to memory-based locking
  const existingLock = memoryStore.get(lockKey);
  if (existingLock && Date.now() < existingLock.expiresAt) {
    return false;
  }

  memoryStore.set(lockKey, {
    key: lockKey,
    status: "processing",
    createdAt: Date.now(),
    expiresAt: Date.now() + config.lockTimeoutMs,
    requestHash: lockValue,
  });

  return true;
}

/**
 * Release lock for idempotency key
 */
async function releaseLock(
  key: string,
  redisClient: any | null,
  config: IdempotencyConfig
): Promise<void> {
  const lockKey = `lock:${config.keyPrefix}:${key}`;

  if (redisClient) {
    try {
      await redisClient.del(lockKey);
    } catch (error) {
      console.error("[Idempotency] Lock release error:", error);
    }
  }

  memoryStore.delete(lockKey);
}

/**
 * Execute operation with idempotency guarantee
 */
export async function withIdempotency<T>(
  idempotencyKey: string,
  requestBody: any,
  operation: () => Promise<T>,
  options: {
    redisClient?: any;
    config?: Partial<IdempotencyConfig>;
  } = {}
): Promise<T> {
  const config = { ...DEFAULT_CONFIG, ...options.config };
  const { redisClient } = options;

  // Validate key
  if (!validateIdempotencyKey(idempotencyKey)) {
    throw new TRPCError({
      code: "BAD_REQUEST",
      message: "Invalid idempotency key format",
    });
  }

  const requestHash = hashRequest(requestBody);

  // Check for existing record
  const existingRecord = redisClient
    ? await getFromRedis(idempotencyKey, redisClient, config)
    : getFromMemory(idempotencyKey);

  if (existingRecord) {
    // Validate request hash matches
    if (existingRecord.requestHash !== requestHash) {
      throw new TRPCError({
        code: "CONFLICT",
        message: "Idempotency key already used with different request parameters",
      });
    }

    // Return cached response if completed
    if (existingRecord.status === "completed") {
      console.info(`[Idempotency] Returning cached response for key: ${idempotencyKey}`);
      return existingRecord.response;
    }

    // Return cached error if failed
    if (existingRecord.status === "failed") {
      throw new TRPCError({
        code: "INTERNAL_SERVER_ERROR",
        message: existingRecord.error || "Previous request failed",
      });
    }

    // Request is still processing
    if (existingRecord.status === "processing") {
      throw new TRPCError({
        code: "CONFLICT",
        message: "Request with this idempotency key is already being processed",
      });
    }
  }

  // Acquire lock
  const lockAcquired = await acquireLock(idempotencyKey, redisClient, config);
  if (!lockAcquired) {
    throw new TRPCError({
      code: "CONFLICT",
      message: "Could not acquire lock for idempotency key",
    });
  }

  // Create processing record
  const record: IdempotencyRecord = {
    key: idempotencyKey,
    status: "processing",
    createdAt: Date.now(),
    expiresAt: Date.now() + config.ttlMs,
    requestHash,
  };

  if (redisClient) {
    await saveToRedis(record, redisClient, config);
  } else {
    saveToMemory(record);
  }

  try {
    // Execute the operation
    const result = await operation();

    // Update record with success
    record.status = "completed";
    record.response = result;
    record.completedAt = Date.now();

    if (redisClient) {
      await saveToRedis(record, redisClient, config);
    } else {
      saveToMemory(record);
    }

    console.info(`[Idempotency] Operation completed for key: ${idempotencyKey}`);
    return result;
  } catch (error) {
    // Update record with failure
    record.status = "failed";
    record.error = error instanceof Error ? error.message : "Unknown error";
    record.completedAt = Date.now();

    if (redisClient) {
      await saveToRedis(record, redisClient, config);
    } else {
      saveToMemory(record);
    }

    console.error(`[Idempotency] Operation failed for key: ${idempotencyKey}`, error);
    throw error;
  } finally {
    // Release lock
    await releaseLock(idempotencyKey, redisClient, config);
  }
}

/**
 * tRPC middleware for idempotent operations
 */
export function createIdempotencyMiddleware(operationType: string) {
  return async ({ ctx, input, next }: { ctx: any; input: any; next: () => Promise<any> }) => {
    // Get idempotency key from header or generate one
    const idempotencyKey = ctx.req?.headers?.["idempotency-key"] ||
                           ctx.req?.headers?.["x-idempotency-key"];

    if (!idempotencyKey) {
      // If no idempotency key provided, proceed without idempotency
      console.warn(`[Idempotency] No idempotency key provided for ${operationType}`);
      return next();
    }

    return withIdempotency(
      idempotencyKey,
      input,
      () => next(),
      { redisClient: ctx.redis }
    );
  };
}

/**
 * Clean up expired records from memory store
 */
export function cleanupExpiredRecords(): void {
  const now = Date.now();
  for (const [key, record] of memoryStore.entries()) {
    if (now > record.expiresAt) {
      memoryStore.delete(key);
    }
  }
}

// Run cleanup every hour
setInterval(cleanupExpiredRecords, 60 * 60 * 1000);

/**
 * Get idempotency record status (for debugging/monitoring)
 */
export async function getIdempotencyStatus(
  key: string,
  redisClient?: any
): Promise<IdempotencyRecord | null> {
  if (redisClient) {
    return getFromRedis(key, redisClient, DEFAULT_CONFIG);
  }
  return getFromMemory(key);
}

/**
 * Manually invalidate an idempotency key (use with caution)
 */
export async function invalidateIdempotencyKey(
  key: string,
  redisClient?: any
): Promise<void> {
  if (redisClient) {
    try {
      await redisClient.del(`${DEFAULT_CONFIG.keyPrefix}:${key}`);
    } catch (error) {
      console.error("[Idempotency] Invalidation error:", error);
    }
  }
  memoryStore.delete(key);
}

/**
 * Decorator for idempotent operations (for use with class methods)
 */
export function Idempotent(operationType: string) {
  return function (
    target: any,
    propertyKey: string,
    descriptor: PropertyDescriptor
  ) {
    const originalMethod = descriptor.value;

    descriptor.value = async function (...args: any[]) {
      const idempotencyKey = args[0]?.idempotencyKey;
      
      if (!idempotencyKey) {
        return originalMethod.apply(this, args);
      }

      return withIdempotency(
        idempotencyKey,
        args,
        () => originalMethod.apply(this, args)
      );
    };

    return descriptor;
  };
}
