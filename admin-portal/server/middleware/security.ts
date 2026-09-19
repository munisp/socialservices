/**
 * Security Middleware
 * CSRF protection, security headers, session management, and input sanitization
 */

import crypto from "crypto";
import { TRPCError } from "@trpc/server";

// ============================================================================
// CSRF Protection
// ============================================================================

interface CSRFConfig {
  tokenLength: number;
  cookieName: string;
  headerName: string;
  cookieOptions: {
    httpOnly: boolean;
    secure: boolean;
    sameSite: "strict" | "lax" | "none";
    maxAge: number;
    path: string;
  };
  ignoreMethods: string[];
  ignorePaths: string[];
}

const DEFAULT_CSRF_CONFIG: CSRFConfig = {
  tokenLength: 32,
  cookieName: "_csrf",
  headerName: "x-csrf-token",
  cookieOptions: {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "strict",
    maxAge: 24 * 60 * 60 * 1000, // 24 hours
    path: "/",
  },
  ignoreMethods: ["GET", "HEAD", "OPTIONS"],
  ignorePaths: ["/api/webhooks", "/api/health"],
};

/**
 * Generate CSRF token
 */
export function generateCSRFToken(): string {
  return crypto.randomBytes(DEFAULT_CSRF_CONFIG.tokenLength).toString("hex");
}

/**
 * Verify CSRF token
 */
export function verifyCSRFToken(token: string, storedToken: string): boolean {
  if (!token || !storedToken) {
    return false;
  }
  // Use timing-safe comparison to prevent timing attacks
  try {
    return crypto.timingSafeEqual(
      Buffer.from(token),
      Buffer.from(storedToken)
    );
  } catch {
    return false;
  }
}

/**
 * CSRF middleware for Express
 */
export function csrfMiddleware(config: Partial<CSRFConfig> = {}) {
  const cfg = { ...DEFAULT_CSRF_CONFIG, ...config };

  return (req: any, res: any, next: () => void) => {
    // Skip CSRF for ignored methods
    if (cfg.ignoreMethods.includes(req.method)) {
      return next();
    }

    // Skip CSRF for ignored paths
    if (cfg.ignorePaths.some(path => req.path.startsWith(path))) {
      return next();
    }

    // Get token from cookie
    const cookieToken = req.cookies?.[cfg.cookieName];

    // Get token from header or body
    const requestToken = req.headers[cfg.headerName] ||
                         req.headers[cfg.headerName.toLowerCase()] ||
                         req.body?._csrf;

    if (!cookieToken) {
      // Generate new token if none exists
      const newToken = generateCSRFToken();
      res.cookie(cfg.cookieName, newToken, cfg.cookieOptions);
      
      // For the first request, we need to reject if it's a state-changing method
      return res.status(403).json({
        error: "CSRF token missing",
        message: "Please refresh the page and try again",
      });
    }

    if (!verifyCSRFToken(requestToken, cookieToken)) {
      return res.status(403).json({
        error: "CSRF token invalid",
        message: "Security validation failed. Please refresh and try again.",
      });
    }

    next();
  };
}

/**
 * Generate and set CSRF token (call on page load)
 */
export function setCSRFToken(res: any): string {
  const token = generateCSRFToken();
  res.cookie(DEFAULT_CSRF_CONFIG.cookieName, token, DEFAULT_CSRF_CONFIG.cookieOptions);
  return token;
}

// ============================================================================
// Security Headers
// ============================================================================

interface SecurityHeadersConfig {
  contentSecurityPolicy: string;
  strictTransportSecurity: string;
  xContentTypeOptions: string;
  xFrameOptions: string;
  xXssProtection: string;
  referrerPolicy: string;
  permissionsPolicy: string;
}

const DEFAULT_SECURITY_HEADERS: SecurityHeadersConfig = {
  contentSecurityPolicy: [
    "default-src 'self'",
    "script-src 'self' 'unsafe-inline' 'unsafe-eval'",
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: https:",
    "font-src 'self' data:",
    "connect-src 'self' wss: https:",
    "frame-ancestors 'none'",
    "base-uri 'self'",
    "form-action 'self'",
  ].join("; "),
  strictTransportSecurity: "max-age=31536000; includeSubDomains; preload",
  xContentTypeOptions: "nosniff",
  xFrameOptions: "DENY",
  xXssProtection: "1; mode=block",
  referrerPolicy: "strict-origin-when-cross-origin",
  permissionsPolicy: [
    "accelerometer=()",
    "camera=()",
    "geolocation=()",
    "gyroscope=()",
    "magnetometer=()",
    "microphone=()",
    "payment=()",
    "usb=()",
  ].join(", "),
};

/**
 * Security headers middleware
 */
export function securityHeadersMiddleware(config: Partial<SecurityHeadersConfig> = {}) {
  const headers = { ...DEFAULT_SECURITY_HEADERS, ...config };

  return (req: any, res: any, next: () => void) => {
    res.setHeader("Content-Security-Policy", headers.contentSecurityPolicy);
    res.setHeader("Strict-Transport-Security", headers.strictTransportSecurity);
    res.setHeader("X-Content-Type-Options", headers.xContentTypeOptions);
    res.setHeader("X-Frame-Options", headers.xFrameOptions);
    res.setHeader("X-XSS-Protection", headers.xXssProtection);
    res.setHeader("Referrer-Policy", headers.referrerPolicy);
    res.setHeader("Permissions-Policy", headers.permissionsPolicy);
    
    // Remove potentially dangerous headers
    res.removeHeader("X-Powered-By");
    res.removeHeader("Server");

    next();
  };
}

// ============================================================================
// Session Management
// ============================================================================

interface SessionConfig {
  maxAge: number;              // Session max age in ms
  absoluteTimeout: number;     // Absolute session timeout in ms
  idleTimeout: number;         // Idle timeout in ms
  renewalThreshold: number;    // Renew session when this much time left (ms)
  maxConcurrentSessions: number;
}

const DEFAULT_SESSION_CONFIG: SessionConfig = {
  maxAge: 8 * 60 * 60 * 1000,        // 8 hours
  absoluteTimeout: 24 * 60 * 60 * 1000, // 24 hours
  idleTimeout: 30 * 60 * 1000,       // 30 minutes
  renewalThreshold: 15 * 60 * 1000,  // 15 minutes
  maxConcurrentSessions: 5,
};

interface SessionData {
  userId: string;
  createdAt: number;
  lastActivityAt: number;
  expiresAt: number;
  absoluteExpiresAt: number;
  userAgent: string;
  ipAddress: string;
  deviceId?: string;
}

// In-memory session store (use Redis in production)
const sessionStore = new Map<string, SessionData>();
const userSessions = new Map<string, Set<string>>();

/**
 * Generate session ID
 */
export function generateSessionId(): string {
  return crypto.randomBytes(32).toString("hex");
}

/**
 * Create new session
 */
export function createSession(
  userId: string,
  req: any,
  config: Partial<SessionConfig> = {}
): { sessionId: string; session: SessionData } {
  const cfg = { ...DEFAULT_SESSION_CONFIG, ...config };
  const sessionId = generateSessionId();
  const now = Date.now();

  const session: SessionData = {
    userId,
    createdAt: now,
    lastActivityAt: now,
    expiresAt: now + cfg.maxAge,
    absoluteExpiresAt: now + cfg.absoluteTimeout,
    userAgent: req.headers?.["user-agent"] || "unknown",
    ipAddress: req.ip || req.headers?.["x-forwarded-for"]?.split(",")[0] || "unknown",
  };

  // Store session
  sessionStore.set(sessionId, session);

  // Track user sessions
  if (!userSessions.has(userId)) {
    userSessions.set(userId, new Set());
  }
  const sessions = userSessions.get(userId)!;
  sessions.add(sessionId);

  // Enforce max concurrent sessions
  if (sessions.size > cfg.maxConcurrentSessions) {
    // Remove oldest sessions
    const sessionsToRemove = sessions.size - cfg.maxConcurrentSessions;
    const sessionIds = Array.from(sessions);
    
    for (let i = 0; i < sessionsToRemove; i++) {
      const oldSessionId = sessionIds[i];
      sessionStore.delete(oldSessionId);
      sessions.delete(oldSessionId);
    }
  }

  return { sessionId, session };
}

/**
 * Validate and refresh session
 */
export function validateSession(
  sessionId: string,
  req: any,
  config: Partial<SessionConfig> = {}
): { valid: boolean; session?: SessionData; renewed?: boolean } {
  const cfg = { ...DEFAULT_SESSION_CONFIG, ...config };
  const session = sessionStore.get(sessionId);
  const now = Date.now();

  if (!session) {
    return { valid: false };
  }

  // Check absolute timeout
  if (now > session.absoluteExpiresAt) {
    destroySession(sessionId);
    return { valid: false };
  }

  // Check session expiry
  if (now > session.expiresAt) {
    destroySession(sessionId);
    return { valid: false };
  }

  // Check idle timeout
  if (now - session.lastActivityAt > cfg.idleTimeout) {
    destroySession(sessionId);
    return { valid: false };
  }

  // Update last activity
  session.lastActivityAt = now;

  // Check if session needs renewal
  let renewed = false;
  if (session.expiresAt - now < cfg.renewalThreshold) {
    session.expiresAt = now + cfg.maxAge;
    renewed = true;
  }

  return { valid: true, session, renewed };
}

/**
 * Destroy session
 */
export function destroySession(sessionId: string): void {
  const session = sessionStore.get(sessionId);
  if (session) {
    const sessions = userSessions.get(session.userId);
    if (sessions) {
      sessions.delete(sessionId);
      if (sessions.size === 0) {
        userSessions.delete(session.userId);
      }
    }
  }
  sessionStore.delete(sessionId);
}

/**
 * Destroy all sessions for a user
 */
export function destroyAllUserSessions(userId: string): number {
  const sessions = userSessions.get(userId);
  if (!sessions) {
    return 0;
  }

  const count = sessions.size;
  for (const sessionId of sessions) {
    sessionStore.delete(sessionId);
  }
  userSessions.delete(userId);

  return count;
}

/**
 * Get active sessions for a user
 */
export function getUserSessions(userId: string): SessionData[] {
  const sessionIds = userSessions.get(userId);
  if (!sessionIds) {
    return [];
  }

  const sessions: SessionData[] = [];
  for (const sessionId of sessionIds) {
    const session = sessionStore.get(sessionId);
    if (session) {
      sessions.push(session);
    }
  }

  return sessions;
}

// ============================================================================
// Input Sanitization
// ============================================================================

/**
 * Sanitize string input (prevent XSS)
 */
export function sanitizeString(input: string): string {
  if (typeof input !== "string") {
    return "";
  }
  
  return input
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#x27;")
    .replace(/\//g, "&#x2F;")
    .replace(/`/g, "&#x60;")
    .replace(/=/g, "&#x3D;");
}

/**
 * Sanitize object recursively
 */
export function sanitizeObject(obj: any): any {
  if (obj === null || obj === undefined) {
    return obj;
  }

  if (typeof obj === "string") {
    return sanitizeString(obj);
  }

  if (Array.isArray(obj)) {
    return obj.map(sanitizeObject);
  }

  if (typeof obj === "object") {
    const sanitized: any = {};
    for (const [key, value] of Object.entries(obj)) {
      sanitized[sanitizeString(key)] = sanitizeObject(value);
    }
    return sanitized;
  }

  return obj;
}

/**
 * Validate and sanitize email
 */
export function sanitizeEmail(email: string): string | null {
  if (typeof email !== "string") {
    return null;
  }
  
  const trimmed = email.trim().toLowerCase();
  const emailRegex = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/;
  
  if (!emailRegex.test(trimmed)) {
    return null;
  }
  
  return trimmed;
}

/**
 * Validate and sanitize phone number
 */
export function sanitizePhone(phone: string): string | null {
  if (typeof phone !== "string") {
    return null;
  }
  
  // Remove all non-digit characters except +
  const cleaned = phone.replace(/[^\d+]/g, "");
  
  // Validate format (international format)
  const phoneRegex = /^\+?[1-9]\d{6,14}$/;
  
  if (!phoneRegex.test(cleaned)) {
    return null;
  }
  
  return cleaned;
}

/**
 * Validate UUID format
 */
export function isValidUUID(uuid: string): boolean {
  const uuidRegex = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
  return uuidRegex.test(uuid);
}

/**
 * Sanitize file path (prevent path traversal)
 */
export function sanitizePath(path: string): string {
  if (typeof path !== "string") {
    return "";
  }
  
  // Remove path traversal attempts
  return path
    .replace(/\.\./g, "")
    .replace(/\/\//g, "/")
    .replace(/\\/g, "/")
    .replace(/^\/+/, "");
}

/**
 * Detect SQL injection attempts
 */
export function detectSQLInjection(input: string): boolean {
  if (typeof input !== "string") {
    return false;
  }
  
  const sqlPatterns = [
    /(\b(SELECT|INSERT|UPDATE|DELETE|DROP|UNION|ALTER|CREATE|TRUNCATE)\b)/i,
    /(--)|(\/\*)|(\*\/)/,
    /(;|\||\||&&)/,
    /(\bOR\b|\bAND\b).*?=/i,
    /['"].*?(=|<|>|LIKE)/i,
  ];
  
  return sqlPatterns.some(pattern => pattern.test(input));
}

/**
 * Detect command injection attempts
 */
export function detectCommandInjection(input: string): boolean {
  if (typeof input !== "string") {
    return false;
  }
  
  const cmdPatterns = [
    /[;&|`$(){}[\]]/,
    /\b(cat|ls|rm|mv|cp|chmod|chown|wget|curl|bash|sh|python|perl|ruby|php)\b/i,
    /\$\(.*\)/,
    /`.*`/,
  ];
  
  return cmdPatterns.some(pattern => pattern.test(input));
}

/**
 * Input validation middleware for tRPC
 */
export function createInputValidationMiddleware() {
  return async ({ input, next }: { input: any; next: () => Promise<any> }) => {
    // Check for injection attempts in string inputs
    const checkInput = (value: any, path: string = ""): void => {
      if (typeof value === "string") {
        if (detectSQLInjection(value)) {
          throw new TRPCError({
            code: "BAD_REQUEST",
            message: `Potentially malicious input detected at ${path}`,
          });
        }
        if (detectCommandInjection(value)) {
          throw new TRPCError({
            code: "BAD_REQUEST",
            message: `Potentially malicious input detected at ${path}`,
          });
        }
      } else if (Array.isArray(value)) {
        value.forEach((item, index) => checkInput(item, `${path}[${index}]`));
      } else if (value && typeof value === "object") {
        Object.entries(value).forEach(([key, val]) => 
          checkInput(val, path ? `${path}.${key}` : key)
        );
      }
    };

    if (input) {
      checkInput(input);
    }

    return next();
  };
}

// ============================================================================
// Cleanup
// ============================================================================

/**
 * Clean up expired sessions
 */
export function cleanupExpiredSessions(): number {
  const now = Date.now();
  let cleaned = 0;

  for (const [sessionId, session] of sessionStore.entries()) {
    if (now > session.expiresAt || now > session.absoluteExpiresAt) {
      destroySession(sessionId);
      cleaned++;
    }
  }

  return cleaned;
}

// Run cleanup every 5 minutes
setInterval(cleanupExpiredSessions, 5 * 60 * 1000);
