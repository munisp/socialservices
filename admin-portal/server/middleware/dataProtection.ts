/**
 * Data Protection Middleware
 * PII masking, data retention policies, and audit logging
 */

import crypto from "crypto";

// ============================================================================
// PII Field Definitions
// ============================================================================

type PIICategory = "name" | "email" | "phone" | "address" | "ssn" | "dob" | "financial" | "biometric" | "health" | "id_document";

interface PIIFieldConfig {
  category: PIICategory;
  maskingStrategy: "full" | "partial" | "hash" | "tokenize" | "redact";
  retentionDays: number;
  requiresConsent: boolean;
  encryptAtRest: boolean;
}

// PII field configurations by field name pattern
const PII_FIELD_CONFIGS: Record<string, PIIFieldConfig> = {
  // Names
  firstName: { category: "name", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  lastName: { category: "name", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  fullName: { category: "name", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  name: { category: "name", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  
  // Contact
  email: { category: "email", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  emailAddress: { category: "email", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  phone: { category: "phone", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  phoneNumber: { category: "phone", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  mobile: { category: "phone", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  
  // Address
  address: { category: "address", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  streetAddress: { category: "address", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  city: { category: "address", maskingStrategy: "redact", retentionDays: 2555, requiresConsent: false, encryptAtRest: false },
  postalCode: { category: "address", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  zipCode: { category: "address", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  
  // Identity
  ssn: { category: "ssn", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  socialSecurityNumber: { category: "ssn", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  nationalId: { category: "id_document", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  passportNumber: { category: "id_document", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  idNumber: { category: "id_document", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  
  // Date of Birth
  dateOfBirth: { category: "dob", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  dob: { category: "dob", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  birthDate: { category: "dob", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  
  // Financial
  bankAccount: { category: "financial", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  accountNumber: { category: "financial", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  routingNumber: { category: "financial", maskingStrategy: "partial", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  cardNumber: { category: "financial", maskingStrategy: "partial", retentionDays: 365, requiresConsent: true, encryptAtRest: true },
  cvv: { category: "financial", maskingStrategy: "full", retentionDays: 0, requiresConsent: true, encryptAtRest: true },
  
  // Biometric
  fingerprint: { category: "biometric", maskingStrategy: "hash", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  faceTemplate: { category: "biometric", maskingStrategy: "hash", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  biometricData: { category: "biometric", maskingStrategy: "hash", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  
  // Health
  healthCondition: { category: "health", maskingStrategy: "redact", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  medicalRecord: { category: "health", maskingStrategy: "redact", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
  disability: { category: "health", maskingStrategy: "redact", retentionDays: 2555, requiresConsent: true, encryptAtRest: true },
};

// ============================================================================
// Masking Functions
// ============================================================================

/**
 * Mask a string value based on strategy
 */
export function maskValue(value: string, strategy: PIIFieldConfig["maskingStrategy"]): string {
  if (!value || typeof value !== "string") {
    return value;
  }

  switch (strategy) {
    case "full":
      return "***REDACTED***";
    
    case "partial":
      return partialMask(value);
    
    case "hash":
      return hashValue(value);
    
    case "tokenize":
      return tokenizeValue(value);
    
    case "redact":
      return "[REDACTED]";
    
    default:
      return value;
  }
}

/**
 * Partial masking - show first and last characters
 */
function partialMask(value: string): string {
  if (value.length <= 4) {
    return "****";
  }
  
  // Handle email addresses specially
  if (value.includes("@")) {
    const [local, domain] = value.split("@");
    const maskedLocal = local.length > 2 
      ? local[0] + "*".repeat(local.length - 2) + local[local.length - 1]
      : "**";
    return `${maskedLocal}@${domain}`;
  }
  
  // Handle phone numbers
  if (/^\+?[\d\s-]+$/.test(value)) {
    const digits = value.replace(/\D/g, "");
    if (digits.length >= 4) {
      return "*".repeat(digits.length - 4) + digits.slice(-4);
    }
    return "****";
  }
  
  // General partial masking
  const visibleChars = Math.min(2, Math.floor(value.length / 4));
  return value.slice(0, visibleChars) + 
         "*".repeat(value.length - visibleChars * 2) + 
         value.slice(-visibleChars);
}

/**
 * Hash value for comparison without revealing original
 */
function hashValue(value: string): string {
  const salt = process.env.PII_HASH_SALT || "default-salt-change-in-production";
  return crypto.createHmac("sha256", salt).update(value).digest("hex").slice(0, 16);
}

// Token storage for reversible tokenization
const tokenStore = new Map<string, string>();
const reverseTokenStore = new Map<string, string>();

/**
 * Tokenize value (reversible with proper access)
 */
function tokenizeValue(value: string): string {
  // Check if already tokenized
  if (tokenStore.has(value)) {
    return tokenStore.get(value)!;
  }
  
  // Generate token
  const token = `TOK_${crypto.randomBytes(16).toString("hex")}`;
  tokenStore.set(value, token);
  reverseTokenStore.set(token, value);
  
  return token;
}

/**
 * Detokenize value (requires authorization)
 */
export function detokenize(token: string): string | null {
  return reverseTokenStore.get(token) || null;
}

// ============================================================================
// Object Masking
// ============================================================================

/**
 * Mask PII fields in an object
 */
export function maskPII(obj: any, options: { 
  preserveStructure?: boolean;
  maskUnknownStrings?: boolean;
  context?: "logging" | "export" | "display";
} = {}): any {
  if (obj === null || obj === undefined) {
    return obj;
  }

  if (typeof obj === "string") {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map(item => maskPII(item, options));
  }

  if (typeof obj !== "object") {
    return obj;
  }

  const masked: any = {};
  
  for (const [key, value] of Object.entries(obj)) {
    const lowerKey = key.toLowerCase();
    const config = findPIIConfig(key);
    
    if (config && typeof value === "string") {
      masked[key] = maskValue(value, config.maskingStrategy);
    } else if (typeof value === "object") {
      masked[key] = maskPII(value, options);
    } else {
      masked[key] = value;
    }
  }
  
  return masked;
}

/**
 * Find PII configuration for a field name
 */
function findPIIConfig(fieldName: string): PIIFieldConfig | null {
  // Direct match
  if (PII_FIELD_CONFIGS[fieldName]) {
    return PII_FIELD_CONFIGS[fieldName];
  }
  
  // Case-insensitive match
  const lowerFieldName = fieldName.toLowerCase();
  for (const [key, config] of Object.entries(PII_FIELD_CONFIGS)) {
    if (key.toLowerCase() === lowerFieldName) {
      return config;
    }
  }
  
  // Pattern matching for common PII patterns
  if (/email/i.test(fieldName)) return PII_FIELD_CONFIGS.email;
  if (/phone|mobile|tel/i.test(fieldName)) return PII_FIELD_CONFIGS.phone;
  if (/ssn|social.*security/i.test(fieldName)) return PII_FIELD_CONFIGS.ssn;
  if (/passport|national.*id|id.*number/i.test(fieldName)) return PII_FIELD_CONFIGS.nationalId;
  if (/bank.*account|account.*number/i.test(fieldName)) return PII_FIELD_CONFIGS.bankAccount;
  if (/card.*number|credit.*card/i.test(fieldName)) return PII_FIELD_CONFIGS.cardNumber;
  if (/birth.*date|date.*birth|dob/i.test(fieldName)) return PII_FIELD_CONFIGS.dateOfBirth;
  if (/address|street/i.test(fieldName)) return PII_FIELD_CONFIGS.address;
  if (/name/i.test(fieldName) && !/user.*name|file.*name|column.*name/i.test(fieldName)) return PII_FIELD_CONFIGS.name;
  
  return null;
}

/**
 * Check if a field contains PII
 */
export function isPIIField(fieldName: string): boolean {
  return findPIIConfig(fieldName) !== null;
}

/**
 * Get PII category for a field
 */
export function getPIICategory(fieldName: string): PIICategory | null {
  const config = findPIIConfig(fieldName);
  return config?.category || null;
}

// ============================================================================
// Data Retention
// ============================================================================

interface RetentionPolicy {
  entityType: string;
  retentionDays: number;
  archiveBeforeDelete: boolean;
  softDelete: boolean;
  anonymizeOnExpiry: boolean;
}

const RETENTION_POLICIES: RetentionPolicy[] = [
  // Beneficiary data
  { entityType: "beneficiary", retentionDays: 2555, archiveBeforeDelete: true, softDelete: true, anonymizeOnExpiry: true },
  { entityType: "beneficiary_document", retentionDays: 2555, archiveBeforeDelete: true, softDelete: true, anonymizeOnExpiry: false },
  
  // Transaction data
  { entityType: "disbursement", retentionDays: 2555, archiveBeforeDelete: true, softDelete: true, anonymizeOnExpiry: true },
  { entityType: "payment", retentionDays: 2555, archiveBeforeDelete: true, softDelete: true, anonymizeOnExpiry: true },
  
  // Audit logs (longer retention)
  { entityType: "audit_log", retentionDays: 3650, archiveBeforeDelete: true, softDelete: false, anonymizeOnExpiry: false },
  
  // Session data (shorter retention)
  { entityType: "session", retentionDays: 90, archiveBeforeDelete: false, softDelete: false, anonymizeOnExpiry: false },
  { entityType: "login_attempt", retentionDays: 365, archiveBeforeDelete: false, softDelete: false, anonymizeOnExpiry: false },
  
  // Temporary data
  { entityType: "otp", retentionDays: 1, archiveBeforeDelete: false, softDelete: false, anonymizeOnExpiry: false },
  { entityType: "verification_token", retentionDays: 7, archiveBeforeDelete: false, softDelete: false, anonymizeOnExpiry: false },
  
  // Grievances
  { entityType: "grievance", retentionDays: 2555, archiveBeforeDelete: true, softDelete: true, anonymizeOnExpiry: true },
  
  // Program data
  { entityType: "program", retentionDays: 3650, archiveBeforeDelete: true, softDelete: true, anonymizeOnExpiry: false },
  { entityType: "enrollment", retentionDays: 2555, archiveBeforeDelete: true, softDelete: true, anonymizeOnExpiry: true },
];

/**
 * Get retention policy for an entity type
 */
export function getRetentionPolicy(entityType: string): RetentionPolicy | null {
  return RETENTION_POLICIES.find(p => p.entityType === entityType) || null;
}

/**
 * Check if data should be retained
 */
export function shouldRetain(entityType: string, createdAt: Date): boolean {
  const policy = getRetentionPolicy(entityType);
  if (!policy) {
    return true; // Default to retain if no policy
  }
  
  const expiryDate = new Date(createdAt);
  expiryDate.setDate(expiryDate.getDate() + policy.retentionDays);
  
  return new Date() < expiryDate;
}

/**
 * Get expiry date for an entity
 */
export function getExpiryDate(entityType: string, createdAt: Date): Date | null {
  const policy = getRetentionPolicy(entityType);
  if (!policy) {
    return null;
  }
  
  const expiryDate = new Date(createdAt);
  expiryDate.setDate(expiryDate.getDate() + policy.retentionDays);
  
  return expiryDate;
}

/**
 * Anonymize an object by removing/masking all PII
 */
export function anonymize(obj: any): any {
  if (obj === null || obj === undefined) {
    return obj;
  }

  if (typeof obj === "string") {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map(anonymize);
  }

  if (typeof obj !== "object") {
    return obj;
  }

  const anonymized: any = {};
  
  for (const [key, value] of Object.entries(obj)) {
    const config = findPIIConfig(key);
    
    if (config) {
      // Replace PII with anonymized placeholder
      anonymized[key] = `[ANONYMIZED_${config.category.toUpperCase()}]`;
    } else if (typeof value === "object") {
      anonymized[key] = anonymize(value);
    } else {
      anonymized[key] = value;
    }
  }
  
  return anonymized;
}

// ============================================================================
// Encryption
// ============================================================================

const ENCRYPTION_KEY = process.env.DATA_ENCRYPTION_KEY || crypto.randomBytes(32).toString("hex");
const ENCRYPTION_ALGORITHM = "aes-256-gcm";

/**
 * Encrypt sensitive data
 */
export function encrypt(data: string): { encrypted: string; iv: string; tag: string } {
  const iv = crypto.randomBytes(16);
  const key = Buffer.from(ENCRYPTION_KEY, "hex");
  const cipher = crypto.createCipheriv(ENCRYPTION_ALGORITHM, key, iv);
  
  let encrypted = cipher.update(data, "utf8", "hex");
  encrypted += cipher.final("hex");
  
  const tag = cipher.getAuthTag();
  
  return {
    encrypted,
    iv: iv.toString("hex"),
    tag: tag.toString("hex"),
  };
}

/**
 * Decrypt sensitive data
 */
export function decrypt(encrypted: string, iv: string, tag: string): string {
  const key = Buffer.from(ENCRYPTION_KEY, "hex");
  const decipher = crypto.createDecipheriv(
    ENCRYPTION_ALGORITHM,
    key,
    Buffer.from(iv, "hex")
  );
  
  decipher.setAuthTag(Buffer.from(tag, "hex"));
  
  let decrypted = decipher.update(encrypted, "hex", "utf8");
  decrypted += decipher.final("utf8");
  
  return decrypted;
}

/**
 * Encrypt PII fields in an object
 */
export function encryptPII(obj: any): any {
  if (obj === null || obj === undefined) {
    return obj;
  }

  if (typeof obj !== "object") {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map(encryptPII);
  }

  const encrypted: any = {};
  
  for (const [key, value] of Object.entries(obj)) {
    const config = findPIIConfig(key);
    
    if (config?.encryptAtRest && typeof value === "string") {
      const { encrypted: enc, iv, tag } = encrypt(value);
      encrypted[key] = { __encrypted: true, data: enc, iv, tag };
    } else if (typeof value === "object") {
      encrypted[key] = encryptPII(value);
    } else {
      encrypted[key] = value;
    }
  }
  
  return encrypted;
}

/**
 * Decrypt PII fields in an object
 */
export function decryptPII(obj: any): any {
  if (obj === null || obj === undefined) {
    return obj;
  }

  if (typeof obj !== "object") {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map(decryptPII);
  }

  const decrypted: any = {};
  
  for (const [key, value] of Object.entries(obj)) {
    if (value && typeof value === "object" && (value as any).__encrypted) {
      const { data, iv, tag } = value as { data: string; iv: string; tag: string };
      decrypted[key] = decrypt(data, iv, tag);
    } else if (typeof value === "object") {
      decrypted[key] = decryptPII(value);
    } else {
      decrypted[key] = value;
    }
  }
  
  return decrypted;
}

// ============================================================================
// Audit Logging
// ============================================================================

interface DataAccessLog {
  timestamp: Date;
  userId: string;
  action: "read" | "write" | "delete" | "export";
  entityType: string;
  entityId: string;
  fieldsAccessed: string[];
  piiFieldsAccessed: string[];
  ipAddress: string;
  userAgent: string;
  justification?: string;
}

const dataAccessLogs: DataAccessLog[] = [];
const MAX_ACCESS_LOGS = 100000;

/**
 * Log data access for audit
 */
export function logDataAccess(log: Omit<DataAccessLog, "timestamp">): void {
  const entry: DataAccessLog = {
    ...log,
    timestamp: new Date(),
  };
  
  dataAccessLogs.push(entry);
  
  // Trim logs if too many
  if (dataAccessLogs.length > MAX_ACCESS_LOGS) {
    dataAccessLogs.splice(0, dataAccessLogs.length - MAX_ACCESS_LOGS);
  }
  
  // Log to console for external collection
  console.info(JSON.stringify({
    type: "data_access_audit",
    ...entry,
  }));
}

/**
 * Get data access logs for an entity
 */
export function getDataAccessLogs(entityType: string, entityId: string): DataAccessLog[] {
  return dataAccessLogs.filter(
    log => log.entityType === entityType && log.entityId === entityId
  );
}

/**
 * Get data access logs for a user
 */
export function getUserDataAccessLogs(userId: string): DataAccessLog[] {
  return dataAccessLogs.filter(log => log.userId === userId);
}

// ============================================================================
// Middleware
// ============================================================================

/**
 * Express middleware for PII masking in responses
 */
export function piiMaskingMiddleware() {
  return (req: any, res: any, next: () => void) => {
    const originalJson = res.json;
    
    res.json = function (data: any) {
      // Mask PII in response data
      const masked = maskPII(data, { context: "display" });
      return originalJson.call(this, masked);
    };
    
    next();
  };
}

/**
 * tRPC middleware for data access logging
 */
export function createDataAccessMiddleware() {
  return async ({ ctx, path, input, next }: { ctx: any; path: string; input: any; next: () => Promise<any> }) => {
    const startTime = Date.now();
    
    try {
      const result = await next();
      
      // Log data access
      if (ctx.user) {
        const fieldsAccessed = input ? Object.keys(input) : [];
        const piiFieldsAccessed = fieldsAccessed.filter(isPIIField);
        
        if (piiFieldsAccessed.length > 0) {
          logDataAccess({
            userId: ctx.user.id,
            action: path.includes("create") || path.includes("update") ? "write" : "read",
            entityType: path.split(".")[0],
            entityId: input?.id || "unknown",
            fieldsAccessed,
            piiFieldsAccessed,
            ipAddress: ctx.req?.ip || "unknown",
            userAgent: ctx.req?.headers?.["user-agent"] || "unknown",
          });
        }
      }
      
      return result;
    } catch (error) {
      throw error;
    }
  };
}

/**
 * Soft delete helper
 */
export function softDelete<T extends { deletedAt?: Date | null }>(entity: T): T {
  return {
    ...entity,
    deletedAt: new Date(),
  };
}

/**
 * Check if entity is soft deleted
 */
export function isSoftDeleted(entity: { deletedAt?: Date | null }): boolean {
  return entity.deletedAt !== null && entity.deletedAt !== undefined;
}

/**
 * Filter out soft deleted entities
 */
export function excludeSoftDeleted<T extends { deletedAt?: Date | null }>(entities: T[]): T[] {
  return entities.filter(e => !isSoftDeleted(e));
}
