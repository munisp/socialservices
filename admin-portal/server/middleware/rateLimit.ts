/**
 * Rate Limiting Middleware
 * Provides configurable rate limiting for API endpoints with Redis-backed storage
 */

import { TRPCError } from "@trpc/server";

interface RateLimitConfig {
  windowMs: number;      // Time window in milliseconds
  maxRequests: number;   // Maximum requests per window
  keyPrefix: string;     // Redis key prefix
  skipFailedRequests?: boolean;
  skipSuccessfulRequests?: boolean;
}

interface RateLimitResult {
  allowed: boolean;
  remaining: number;
  resetTime: number;
  retryAfter?: number;
}

// In-memory store for rate limiting (fallback when Redis unavailable)
const memoryStore = new Map<string, { count: number; resetTime: number }>();

// Default rate limit configurations by endpoint type
export const RATE_LIMIT_CONFIGS = {
  // Standard API endpoints
  standard: {
    windowMs: 60 * 1000,      // 1 minute
    maxRequests: 100,
    keyPrefix: "rl:standard",
  },
  // Authentication endpoints (stricter)
  auth: {
    windowMs: 15 * 60 * 1000, // 15 minutes
    maxRequests: 10,
    keyPrefix: "rl:auth",
  },
  // Batch operations (very strict)
  batch: {
    windowMs: 60 * 1000,      // 1 minute
    maxRequests: 5,
    keyPrefix: "rl:batch",
  },
  // Report generation (resource intensive)
  reports: {
    windowMs: 5 * 60 * 1000,  // 5 minutes
    maxRequests: 10,
    keyPrefix: "rl:reports",
  },
  // Search operations
  search: {
    windowMs: 60 * 1000,      // 1 minute
    maxRequests: 50,
    keyPrefix: "rl:search",
  },
  // Disbursement operations (financial - very strict)
  disbursement: {
    windowMs: 60 * 1000,      // 1 minute
    maxRequests: 10,
    keyPrefix: "rl:disbursement",
  },
  // Webhook endpoints
  webhook: {
    windowMs: 60 * 1000,      // 1 minute
    maxRequests: 1000,
    keyPrefix: "rl:webhook",
  },
} as const;

/**
 * Get rate limit key for a user/IP
 */
function getRateLimitKey(config: RateLimitConfig, identifier: string): string {
  return `${config.keyPrefix}:${identifier}`;
}

/**
 * Check rate limit using in-memory store (fallback)
 */
function checkMemoryRateLimit(key: string, config: RateLimitConfig): RateLimitResult {
  const now = Date.now();
  const record = memoryStore.get(key);

  if (!record || now > record.resetTime) {
    // Create new window
    memoryStore.set(key, {
      count: 1,
      resetTime: now + config.windowMs,
    });
    return {
      allowed: true,
      remaining: config.maxRequests - 1,
      resetTime: now + config.windowMs,
    };
  }

  if (record.count >= config.maxRequests) {
    return {
      allowed: false,
      remaining: 0,
      resetTime: record.resetTime,
      retryAfter: Math.ceil((record.resetTime - now) / 1000),
    };
  }

  record.count++;
  return {
    allowed: true,
    remaining: config.maxRequests - record.count,
    resetTime: record.resetTime,
  };
}

/**
 * Check rate limit using Redis
 */
async function checkRedisRateLimit(
  key: string,
  config: RateLimitConfig,
  redisClient: any
): Promise<RateLimitResult> {
  const now = Date.now();
  const windowStart = now - config.windowMs;

  try {
    // Use Redis sorted set for sliding window rate limiting
    const multi = redisClient.multi();
    
    // Remove old entries outside the window
    multi.zremrangebyscore(key, 0, windowStart);
    
    // Count current requests in window
    multi.zcard(key);
    
    // Add current request
    multi.zadd(key, now, `${now}:${Math.random()}`);
    
    // Set expiry on the key
    multi.expire(key, Math.ceil(config.windowMs / 1000) + 1);

    const results = await multi.exec();
    const currentCount = results[1][1] as number;

    if (currentCount >= config.maxRequests) {
      // Get the oldest request timestamp to calculate retry-after
      const oldestRequest = await redisClient.zrange(key, 0, 0, "WITHSCORES");
      const retryAfter = oldestRequest.length > 1 
        ? Math.ceil((parseInt(oldestRequest[1]) + config.windowMs - now) / 1000)
        : Math.ceil(config.windowMs / 1000);

      return {
        allowed: false,
        remaining: 0,
        resetTime: now + config.windowMs,
        retryAfter,
      };
    }

    return {
      allowed: true,
      remaining: config.maxRequests - currentCount - 1,
      resetTime: now + config.windowMs,
    };
  } catch (error) {
    console.error("[RateLimit] Redis error, falling back to memory:", error);
    return checkMemoryRateLimit(key, config);
  }
}

/**
 * Rate limit checker function
 */
export async function checkRateLimit(
  identifier: string,
  configType: keyof typeof RATE_LIMIT_CONFIGS,
  redisClient?: any
): Promise<RateLimitResult> {
  const config = RATE_LIMIT_CONFIGS[configType];
  const key = getRateLimitKey(config, identifier);

  if (redisClient) {
    return checkRedisRateLimit(key, config, redisClient);
  }

  return checkMemoryRateLimit(key, config);
}

/**
 * Rate limit middleware for tRPC procedures
 */
export function createRateLimitMiddleware(configType: keyof typeof RATE_LIMIT_CONFIGS) {
  return async ({ ctx, next }: { ctx: any; next: () => Promise<any> }) => {
    // Get identifier (user ID or IP address)
    const identifier = ctx.user?.id?.toString() || 
                       ctx.req?.ip || 
                       ctx.req?.headers?.["x-forwarded-for"]?.split(",")[0] ||
                       "anonymous";

    const result = await checkRateLimit(identifier, configType, ctx.redis);

    if (!result.allowed) {
      throw new TRPCError({
        code: "TOO_MANY_REQUESTS",
        message: `Rate limit exceeded. Please retry after ${result.retryAfter} seconds.`,
      });
    }

    // Add rate limit headers to response
    if (ctx.res) {
      ctx.res.setHeader("X-RateLimit-Limit", RATE_LIMIT_CONFIGS[configType].maxRequests);
      ctx.res.setHeader("X-RateLimit-Remaining", result.remaining);
      ctx.res.setHeader("X-RateLimit-Reset", result.resetTime);
    }

    return next();
  };
}

/**
 * Express middleware for rate limiting
 */
export function expressRateLimitMiddleware(configType: keyof typeof RATE_LIMIT_CONFIGS) {
  return async (req: any, res: any, next: () => void) => {
    const identifier = req.user?.id?.toString() ||
                       req.ip ||
                       req.headers["x-forwarded-for"]?.split(",")[0] ||
                       "anonymous";

    const result = await checkRateLimit(identifier, configType, req.redis);

    res.setHeader("X-RateLimit-Limit", RATE_LIMIT_CONFIGS[configType].maxRequests);
    res.setHeader("X-RateLimit-Remaining", result.remaining);
    res.setHeader("X-RateLimit-Reset", result.resetTime);

    if (!result.allowed) {
      res.setHeader("Retry-After", result.retryAfter);
      return res.status(429).json({
        error: "Too Many Requests",
        message: `Rate limit exceeded. Please retry after ${result.retryAfter} seconds.`,
        retryAfter: result.retryAfter,
      });
    }

    next();
  };
}

/**
 * Clean up expired entries from memory store (run periodically)
 */
export function cleanupMemoryStore(): void {
  const now = Date.now();
  for (const [key, record] of memoryStore.entries()) {
    if (now > record.resetTime) {
      memoryStore.delete(key);
    }
  }
}

// Run cleanup every minute
setInterval(cleanupMemoryStore, 60 * 1000);

/**
 * Get current rate limit status for a user
 */
export async function getRateLimitStatus(
  identifier: string,
  configType: keyof typeof RATE_LIMIT_CONFIGS,
  redisClient?: any
): Promise<{ used: number; limit: number; remaining: number; resetTime: number }> {
  const config = RATE_LIMIT_CONFIGS[configType];
  const key = getRateLimitKey(config, identifier);
  const now = Date.now();

  if (redisClient) {
    try {
      const windowStart = now - config.windowMs;
      await redisClient.zremrangebyscore(key, 0, windowStart);
      const count = await redisClient.zcard(key);
      
      return {
        used: count,
        limit: config.maxRequests,
        remaining: Math.max(0, config.maxRequests - count),
        resetTime: now + config.windowMs,
      };
    } catch (error) {
      console.error("[RateLimit] Redis error:", error);
    }
  }

  const record = memoryStore.get(key);
  if (!record || now > record.resetTime) {
    return {
      used: 0,
      limit: config.maxRequests,
      remaining: config.maxRequests,
      resetTime: now + config.windowMs,
    };
  }

  return {
    used: record.count,
    limit: config.maxRequests,
    remaining: Math.max(0, config.maxRequests - record.count),
    resetTime: record.resetTime,
  };
}
