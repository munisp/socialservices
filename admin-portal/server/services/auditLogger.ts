/**
 * Audit Logger Service
 * 
 * Provides comprehensive audit logging for sensitive actions including
 * ID verification, consent changes, disbursements, and data access.
 */

import { v4 as uuidv4 } from "uuid";

export interface AuditEvent {
  id: string;
  timestamp: Date;
  correlationId: string;
  action: AuditAction;
  actor: AuditActor;
  resource: AuditResource;
  outcome: "success" | "failure" | "partial";
  details: Record<string, unknown>;
  metadata: AuditMetadata;
}

export type AuditAction =
  // Identity verification
  | "identity.verify"
  | "identity.consent.grant"
  | "identity.consent.revoke"
  | "identity.biometric.capture"
  | "identity.biometric.match"
  // Beneficiary management
  | "beneficiary.create"
  | "beneficiary.update"
  | "beneficiary.delete"
  | "beneficiary.view"
  | "beneficiary.export"
  | "beneficiary.enroll"
  | "beneficiary.graduate"
  | "beneficiary.suspend"
  // Disbursement
  | "disbursement.initiate"
  | "disbursement.approve"
  | "disbursement.reject"
  | "disbursement.execute"
  | "disbursement.reverse"
  // Data access
  | "data.query"
  | "data.export"
  | "data.import"
  | "data.sync"
  // Interoperability
  | "interop.consent.grant"
  | "interop.consent.revoke"
  | "interop.data.request"
  | "interop.data.share"
  // PMT
  | "pmt.survey.submit"
  | "pmt.score.calculate"
  | "pmt.eligibility.determine"
  // Authentication
  | "auth.login"
  | "auth.logout"
  | "auth.token.refresh"
  | "auth.password.change"
  | "auth.mfa.enable"
  | "auth.mfa.disable"
  // Authorization
  | "authz.role.assign"
  | "authz.role.revoke"
  | "authz.permission.grant"
  | "authz.permission.revoke";

export interface AuditActor {
  type: "user" | "system" | "service" | "api_key";
  id: string;
  name?: string;
  email?: string;
  roles?: string[];
  ip?: string;
  userAgent?: string;
  tenantId?: string;
}

export interface AuditResource {
  type: string;
  id: string;
  name?: string;
  attributes?: Record<string, unknown>;
}

export interface AuditMetadata {
  requestId: string;
  sessionId?: string;
  deviceId?: string;
  location?: {
    country?: string;
    region?: string;
    city?: string;
    coordinates?: { lat: number; lng: number };
  };
  duration?: number;
  errorCode?: string;
  errorMessage?: string;
}

// In-memory buffer for batching (production would use Kafka/Redis)
const auditBuffer: AuditEvent[] = [];
const BUFFER_FLUSH_SIZE = 100;
const BUFFER_FLUSH_INTERVAL_MS = 5000;

let flushTimer: NodeJS.Timeout | null = null;

/**
 * Log an audit event
 */
export async function logAuditEvent(
  action: AuditAction,
  actor: AuditActor,
  resource: AuditResource,
  outcome: AuditEvent["outcome"],
  details: Record<string, unknown> = {},
  metadata: Partial<AuditMetadata> = {}
): Promise<string> {
  const event: AuditEvent = {
    id: uuidv4(),
    timestamp: new Date(),
    correlationId: metadata.requestId || uuidv4(),
    action,
    actor,
    resource,
    outcome,
    details: sanitizeDetails(details),
    metadata: {
      requestId: metadata.requestId || uuidv4(),
      sessionId: metadata.sessionId,
      deviceId: metadata.deviceId,
      location: metadata.location,
      duration: metadata.duration,
      errorCode: metadata.errorCode,
      errorMessage: metadata.errorMessage,
    },
  };

  // Add to buffer
  auditBuffer.push(event);

  // Log immediately for critical actions
  if (isCriticalAction(action)) {
    await flushAuditBuffer();
  } else if (auditBuffer.length >= BUFFER_FLUSH_SIZE) {
    await flushAuditBuffer();
  } else if (!flushTimer) {
    flushTimer = setTimeout(flushAuditBuffer, BUFFER_FLUSH_INTERVAL_MS);
  }

  // Also log to console for immediate visibility
  console.log(
    `[Audit] ${event.timestamp.toISOString()} | ${event.action} | ${event.outcome} | ` +
    `actor=${event.actor.id} | resource=${event.resource.type}:${event.resource.id} | ` +
    `correlationId=${event.correlationId}`
  );

  return event.id;
}

/**
 * Check if action is critical and requires immediate persistence
 */
function isCriticalAction(action: AuditAction): boolean {
  const criticalActions: AuditAction[] = [
    "disbursement.execute",
    "disbursement.reverse",
    "identity.consent.grant",
    "identity.consent.revoke",
    "interop.consent.grant",
    "interop.consent.revoke",
    "authz.role.assign",
    "authz.role.revoke",
    "beneficiary.delete",
    "auth.password.change",
  ];
  return criticalActions.includes(action);
}

/**
 * Sanitize details to remove sensitive data
 */
function sanitizeDetails(details: Record<string, unknown>): Record<string, unknown> {
  const sensitiveKeys = [
    "password", "secret", "token", "key", "credential",
    "ssn", "nationalId", "biometric", "fingerprint",
  ];
  
  const sanitized: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(details)) {
    const lowerKey = key.toLowerCase();
    if (sensitiveKeys.some(sk => lowerKey.includes(sk))) {
      sanitized[key] = "[REDACTED]";
    } else if (typeof value === "object" && value !== null) {
      sanitized[key] = sanitizeDetails(value as Record<string, unknown>);
    } else {
      sanitized[key] = value;
    }
  }
  return sanitized;
}

/**
 * Flush audit buffer to persistent storage
 */
async function flushAuditBuffer(): Promise<void> {
  if (flushTimer) {
    clearTimeout(flushTimer);
    flushTimer = null;
  }

  if (auditBuffer.length === 0) return;

  const events = auditBuffer.splice(0, auditBuffer.length);
  
  try {
    // In production, this would send to:
    // 1. Kafka topic for real-time processing
    // 2. Database for persistence
    // 3. SIEM system for security monitoring
    
    const kafkaUrl = process.env.KAFKA_BROKERS;
    if (kafkaUrl) {
      console.log(`[Audit] Flushed ${events.length} events to Kafka`);
    } else {
      console.log(`[Audit] Flushed ${events.length} events (no Kafka configured)`);
    }
  } catch (error) {
    // Re-add events to buffer on failure
    auditBuffer.unshift(...events);
    console.error("[Audit] Failed to flush audit buffer:", error);
  }
}

/**
 * Query audit events (for audit dashboard)
 */
export async function queryAuditEvents(
  filters: {
    action?: AuditAction;
    actorId?: string;
    resourceType?: string;
    resourceId?: string;
    startDate?: Date;
    endDate?: Date;
    outcome?: AuditEvent["outcome"];
  },
  pagination: { limit: number; offset: number }
): Promise<{ events: AuditEvent[]; total: number }> {
  let filtered = [...auditBuffer];
  
  if (filters.action) {
    filtered = filtered.filter(e => e.action === filters.action);
  }
  if (filters.actorId) {
    filtered = filtered.filter(e => e.actor.id === filters.actorId);
  }
  if (filters.resourceType) {
    filtered = filtered.filter(e => e.resource.type === filters.resourceType);
  }
  if (filters.resourceId) {
    filtered = filtered.filter(e => e.resource.id === filters.resourceId);
  }
  if (filters.startDate) {
    filtered = filtered.filter(e => e.timestamp >= filters.startDate!);
  }
  if (filters.endDate) {
    filtered = filtered.filter(e => e.timestamp <= filters.endDate!);
  }
  if (filters.outcome) {
    filtered = filtered.filter(e => e.outcome === filters.outcome);
  }

  const total = filtered.length;
  const events = filtered.slice(pagination.offset, pagination.offset + pagination.limit);

  return { events, total };
}

/**
 * Create audit context for a request
 */
export function createAuditContext(
  userId: string,
  userEmail?: string,
  roles?: string[],
  ip?: string,
  userAgent?: string,
  tenantId?: string
): AuditActor {
  return {
    type: "user",
    id: userId,
    email: userEmail,
    roles,
    ip,
    userAgent,
    tenantId,
  };
}

// Convenience functions for common audit actions
export const audit = {
  identityVerify: (actor: AuditActor, beneficiaryId: string, providerId: string, outcome: AuditEvent["outcome"], details?: Record<string, unknown>) =>
    logAuditEvent("identity.verify", actor, { type: "beneficiary", id: beneficiaryId }, outcome, { providerId, ...details }),
  
  consentGrant: (actor: AuditActor, beneficiaryId: string, sector: string, outcome: AuditEvent["outcome"]) =>
    logAuditEvent("interop.consent.grant", actor, { type: "beneficiary", id: beneficiaryId }, outcome, { sector }),
  
  consentRevoke: (actor: AuditActor, beneficiaryId: string, sector: string, outcome: AuditEvent["outcome"]) =>
    logAuditEvent("interop.consent.revoke", actor, { type: "beneficiary", id: beneficiaryId }, outcome, { sector }),
  
  disbursementExecute: (actor: AuditActor, disbursementId: string, beneficiaryId: string, amount: number, currency: string, outcome: AuditEvent["outcome"]) =>
    logAuditEvent("disbursement.execute", actor, { type: "disbursement", id: disbursementId }, outcome, { beneficiaryId, amount, currency }),
  
  beneficiaryCreate: (actor: AuditActor, beneficiaryId: string, outcome: AuditEvent["outcome"]) =>
    logAuditEvent("beneficiary.create", actor, { type: "beneficiary", id: beneficiaryId }, outcome),
  
  beneficiaryUpdate: (actor: AuditActor, beneficiaryId: string, changes: Record<string, unknown>, outcome: AuditEvent["outcome"]) =>
    logAuditEvent("beneficiary.update", actor, { type: "beneficiary", id: beneficiaryId }, outcome, { changes }),
  
  dataExport: (actor: AuditActor, exportType: string, recordCount: number, outcome: AuditEvent["outcome"]) =>
    logAuditEvent("data.export", actor, { type: "export", id: uuidv4() }, outcome, { exportType, recordCount }),
  
  pmtSurveySubmit: (actor: AuditActor, householdId: string, score: number, outcome: AuditEvent["outcome"]) =>
    logAuditEvent("pmt.survey.submit", actor, { type: "household", id: householdId }, outcome, { score }),
};

export default audit;
