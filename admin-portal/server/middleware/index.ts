/**
 * Middleware Index
 * Exports all middleware for easy import across the application
 */

// Rate Limiting
export {
  checkRateLimit,
  createRateLimitMiddleware,
  expressRateLimitMiddleware,
  getRateLimitStatus,
  RATE_LIMIT_CONFIGS,
  cleanupMemoryStore as cleanupRateLimitStore,
} from "./rateLimit";

// Circuit Breaker
export {
  CircuitBreaker,
  CircuitBreakerOpenError,
  CircuitState,
  getCircuitBreaker,
  withCircuitBreaker,
  getAllCircuitBreakerHealth,
  onCircuitBreakerStateChange,
  resetAllCircuitBreakers,
  fetchWithCircuitBreaker,
  CIRCUIT_BREAKER_CONFIGS,
} from "./circuitBreaker";

// Idempotency
export {
  withIdempotency,
  createIdempotencyMiddleware,
  generateIdempotencyKey,
  validateIdempotencyKey,
  getIdempotencyStatus,
  invalidateIdempotencyKey,
  cleanupExpiredRecords as cleanupIdempotencyRecords,
  Idempotent,
} from "./idempotency";

// Security (CSRF, Headers, Session, Input Validation)
export {
  // CSRF
  generateCSRFToken,
  verifyCSRFToken,
  csrfMiddleware,
  setCSRFToken,
  // Security Headers
  securityHeadersMiddleware,
  // Session Management
  generateSessionId,
  createSession,
  validateSession,
  destroySession,
  destroyAllUserSessions,
  getUserSessions,
  cleanupExpiredSessions,
  // Input Sanitization
  sanitizeString,
  sanitizeObject,
  sanitizeEmail,
  sanitizePhone,
  isValidUUID,
  sanitizePath,
  detectSQLInjection,
  detectCommandInjection,
  createInputValidationMiddleware,
} from "./security";

// Observability (Metrics, Tracing, Logging)
export {
  // Prometheus Metrics
  incrementCounter,
  setGauge,
  incrementGauge,
  decrementGauge,
  observeHistogram,
  startTimer,
  getPrometheusMetrics,
  metricsHandler,
  // Distributed Tracing
  extractTraceContext,
  injectTraceContext,
  startSpan,
  setSpanTag,
  logSpanEvent,
  endSpan,
  getSpanDuration,
  // Structured Logging
  log,
  logger,
  // Middleware
  observabilityMiddleware,
  createObservabilityMiddleware,
} from "./observability";

// Data Protection (PII Masking, Retention, Encryption)
export {
  // PII Masking
  maskValue,
  maskPII,
  isPIIField,
  getPIICategory,
  detokenize,
  anonymize,
  // Data Retention
  getRetentionPolicy,
  shouldRetain,
  getExpiryDate,
  // Encryption
  encrypt,
  decrypt,
  encryptPII,
  decryptPII,
  // Audit Logging
  logDataAccess,
  getDataAccessLogs,
  getUserDataAccessLogs,
  // Soft Delete
  softDelete,
  isSoftDeleted,
  excludeSoftDeleted,
  // Middleware
  piiMaskingMiddleware,
  createDataAccessMiddleware,
} from "./dataProtection";

// Notifications
export {
  // Queue Management
  createNotification,
  queueNotification,
  // Delivery Tracking
  handleDeliveryWebhook,
  onDeliveryStatus,
  getNotificationStatus,
  getUserNotifications,
  // Dead Letter Queue
  getDeadLetterQueue,
  retryFromDeadLetterQueue,
  // User Preferences
  setUserPreference,
  getUserPreferences,
  // Metrics
  getNotificationMetrics,
  // Cleanup
  cleanupOldRecords as cleanupOldNotifications,
} from "./notifications";

// Permify Authorization
export {
  getPermifyClient,
  PERMIFY_SCHEMA,
  writeAuthorizationSchema,
  createRelationship,
  deleteRelationship,
  checkPermission,
  checkPermissions,
  getUserPermissions,
  listAccessibleEntities,
  grantBeneficiaryAccess,
  canViewBeneficiary,
  canEditBeneficiary,
  grantProgramAccess,
  canManageProgram,
  shareDocument,
  canViewDocument,
  permifyHealthCheck,
} from "./permify";

// Keycloak Authentication
export {
  getKeycloakClient,
  exchangeCodeForTokens,
  verifyAccessToken,
  refreshAccessToken,
  extractUserFromToken,
  keycloakHealthCheck,
} from "./keycloak";

/**
 * Initialize all middleware
 * Call this during application startup
 */
export async function initializeMiddleware(): Promise<void> {
  console.log("[Middleware] Initializing all middleware...");
  
  // Initialize Permify schema
  try {
    await writeAuthorizationSchema();
    console.log("[Middleware] Permify schema initialized");
  } catch (error) {
    console.warn("[Middleware] Permify schema initialization failed:", error);
  }
  
  // Start cleanup intervals (already started in individual modules)
  console.log("[Middleware] Cleanup intervals started");
  
  console.log("[Middleware] All middleware initialized");
}

/**
 * Health check for all middleware services
 */
export async function middlewareHealthCheck(): Promise<{
  permify: boolean;
  keycloak: boolean;
  circuitBreakers: Record<string, { healthy: boolean; state: string; failureRate: number }>;
}> {
  const [permifyHealth, keycloakHealth] = await Promise.all([
    permifyHealthCheck(),
    keycloakHealthCheck(),
  ]);
  
  return {
    permify: permifyHealth,
    keycloak: keycloakHealth,
    circuitBreakers: getAllCircuitBreakerHealth(),
  };
}
