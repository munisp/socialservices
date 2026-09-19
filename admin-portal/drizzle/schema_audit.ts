import { int, mysqlEnum, mysqlTable, text, timestamp, varchar, json } from "drizzle-orm/mysql-core";

/**
 * Comprehensive Audit Log Schema
 * Tracks all system activities, user actions, and workflow events for compliance
 */

export const auditLogs = mysqlTable("audit_logs", {
  id: int("id").autoincrement().primaryKey(),
  
  // Event identification
  eventType: varchar("event_type", { length: 100 }).notNull(), // e.g., "user_login", "workflow_started", "beneficiary_created"
  eventCategory: mysqlEnum("event_category", ["authentication", "workflow", "data_modification", "system", "security", "approval"]).notNull(),
  eventAction: varchar("event_action", { length: 100 }).notNull(), // e.g., "create", "update", "delete", "execute"
  
  // Actor information
  userId: int("user_id"),
  userName: varchar("user_name", { length: 255 }),
  userRole: varchar("user_role", { length: 100 }),
  userIpAddress: varchar("user_ip_address", { length: 45 }), // IPv6 support
  userAgent: text("user_agent"),
  
  // Target information
  targetType: varchar("target_type", { length: 100 }), // e.g., "beneficiary", "workflow", "user"
  targetId: varchar("target_id", { length: 255 }),
  targetName: varchar("target_name", { length: 255 }),
  
  // Event details
  description: text("description").notNull(),
  changes: json("changes"), // Before/after values for data modifications
  metadata: json("metadata"), // Additional context
  
  // Request context
  requestId: varchar("request_id", { length: 255 }), // For correlating related events
  sessionId: varchar("session_id", { length: 255 }),
  
  // Workflow context (if applicable)
  workflowId: varchar("workflow_id", { length: 255 }),
  workflowType: varchar("workflow_type", { length: 100 }),
  
  // Result
  status: mysqlEnum("status", ["success", "failure", "partial"]).notNull(),
  errorMessage: text("error_message"),
  
  // Compliance
  severity: mysqlEnum("severity", ["low", "medium", "high", "critical"]).default("low").notNull(),
  requiresReview: int("requires_review").default(0).notNull(),
  reviewedBy: int("reviewed_by"),
  reviewedAt: timestamp("reviewed_at"),
  reviewNotes: text("review_notes"),
  
  // Timestamp
  timestamp: timestamp("timestamp").defaultNow().notNull(),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
});

export const auditLogExports = mysqlTable("audit_log_exports", {
  id: int("id").autoincrement().primaryKey(),
  
  // Export details
  exportType: mysqlEnum("export_type", ["pdf", "csv", "json"]).notNull(),
  filters: json("filters"), // Filters applied to the export
  
  // Date range
  startDate: timestamp("start_date").notNull(),
  endDate: timestamp("end_date").notNull(),
  
  // Results
  recordCount: int("record_count").notNull(),
  fileUrl: varchar("file_url", { length: 500 }),
  fileSize: int("file_size"), // bytes
  
  // Status
  status: mysqlEnum("status", ["pending", "processing", "completed", "failed"]).notNull().default("pending"),
  errorMessage: text("error_message"),
  
  // Expiry
  expiresAt: timestamp("expires_at"),
  
  // Requester
  requestedBy: int("requested_by").notNull(),
  requestedByName: varchar("requested_by_name", { length: 255 }),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  completedAt: timestamp("completed_at"),
});

export const complianceReports = mysqlTable("compliance_reports", {
  id: int("id").autoincrement().primaryKey(),
  
  // Report details
  reportType: varchar("report_type", { length: 100 }).notNull(), // e.g., "monthly_activity", "security_incidents", "data_access"
  reportPeriod: varchar("report_period", { length: 100 }).notNull(), // e.g., "2024-01", "Q1-2024"
  
  // Date range
  startDate: timestamp("start_date").notNull(),
  endDate: timestamp("end_date").notNull(),
  
  // Report content
  summary: json("summary").notNull(),
  details: json("details"),
  findings: json("findings"), // Array of compliance findings
  recommendations: json("recommendations"),
  
  // Files
  reportFileUrl: varchar("report_file_url", { length: 500 }),
  attachments: json("attachments"), // Array of attachment URLs
  
  // Status
  status: mysqlEnum("status", ["draft", "final", "archived"]).notNull().default("draft"),
  
  // Metadata
  generatedBy: int("generated_by").notNull(),
  reviewedBy: int("reviewed_by"),
  approvedBy: int("approved_by"),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
  publishedAt: timestamp("published_at"),
});

export const auditLogRetention = mysqlTable("audit_log_retention", {
  id: int("id").autoincrement().primaryKey(),
  
  // Retention policy
  eventCategory: varchar("event_category", { length: 100 }).notNull(),
  retentionDays: int("retention_days").notNull(), // 0 = indefinite
  archiveAfterDays: int("archive_after_days"), // Move to cold storage
  
  // Compliance requirement
  complianceRequirement: varchar("compliance_requirement", { length: 255 }), // e.g., "GDPR", "SOC2", "PCI-DSS"
  
  // Status
  enabled: int("enabled").default(1).notNull(),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export type AuditLog = typeof auditLogs.$inferSelect;
export type InsertAuditLog = typeof auditLogs.$inferInsert;

export type AuditLogExport = typeof auditLogExports.$inferSelect;
export type InsertAuditLogExport = typeof auditLogExports.$inferInsert;

export type ComplianceReport = typeof complianceReports.$inferSelect;
export type InsertComplianceReport = typeof complianceReports.$inferInsert;

export type AuditLogRetention = typeof auditLogRetention.$inferSelect;
export type InsertAuditLogRetention = typeof auditLogRetention.$inferInsert;
