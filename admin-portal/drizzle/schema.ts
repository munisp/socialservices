import { int, mysqlEnum, mysqlTable, text, timestamp, varchar, json, decimal, boolean, float } from "drizzle-orm/mysql-core";

/**
 * Core user table backing auth flow.
 */
export const users = mysqlTable("users", {
  id: int("id").autoincrement().primaryKey(),
  openId: varchar("openId", { length: 64 }).notNull().unique(),
  name: text("name"),
  email: varchar("email", { length: 320 }),
  loginMethod: varchar("loginMethod", { length: 64 }),
  role: mysqlEnum("role", ["user", "admin"]).default("user").notNull(),
  createdAt: timestamp("createdAt").defaultNow().notNull(),
  updatedAt: timestamp("updatedAt").defaultNow().onUpdateNow().notNull(),
  lastSignedIn: timestamp("lastSignedIn").defaultNow().notNull(),
});

export type User = typeof users.$inferSelect;
export type InsertUser = typeof users.$inferInsert;

/**
 * Social benefit programs (e.g., Food Subsidy, Education Grant)
 */
export const benefitPrograms = mysqlTable("benefit_programs", {
  id: int("id").autoincrement().primaryKey(),
  name: varchar("name", { length: 255 }).notNull(),
  accountType: varchar("accountType", { length: 100 }).notNull().unique(), // e.g., FOOD_BENEFIT
  description: text("description"),
  status: mysqlEnum("status", ["active", "inactive", "pending_review"]).default("active").notNull(),
  createdAt: timestamp("createdAt").defaultNow().notNull(),
  updatedAt: timestamp("updatedAt").defaultNow().onUpdateNow().notNull(),
  createdBy: int("createdBy").references(() => users.id),
  // Soft delete fields
  deletedAt: timestamp("deletedAt"),
  deletedBy: int("deletedBy").references(() => users.id),
});

export type BenefitProgram = typeof benefitPrograms.$inferSelect;
export type InsertBenefitProgram = typeof benefitPrograms.$inferInsert;

/**
 * MCC (Merchant Category Code) rules for earmarked spending
 * Each rule links a benefit program to an approved MCC
 */
export const mccRules = mysqlTable("mcc_rules", {
  id: int("id").autoincrement().primaryKey(),
  programId: int("programId").notNull().references(() => benefitPrograms.id, { onDelete: "cascade" }),
  mccCode: varchar("mccCode", { length: 10 }).notNull(), // e.g., "5411"
  mccDescription: varchar("mccDescription", { length: 255 }), // e.g., "Grocery Stores"
  createdAt: timestamp("createdAt").defaultNow().notNull(),
  createdBy: int("createdBy").references(() => users.id),
});

export type MccRule = typeof mccRules.$inferSelect;
export type InsertMccRule = typeof mccRules.$inferInsert;

/**
 * Audit log for tracking changes to MCC rules
 */
export const mccRuleAudit = mysqlTable("mcc_rule_audit", {
  id: int("id").autoincrement().primaryKey(),
  programId: int("programId").notNull().references(() => benefitPrograms.id),
  action: mysqlEnum("action", ["add_mcc", "remove_mcc", "update_program"]).notNull(),
  mccCode: varchar("mccCode", { length: 10 }), // NULL for program updates
  justification: text("justification").notNull(),
  performedBy: int("performedBy").notNull().references(() => users.id),
  performedAt: timestamp("performedAt").defaultNow().notNull(),
});

export type MccRuleAudit = typeof mccRuleAudit.$inferSelect;
export type InsertMccRuleAudit = typeof mccRuleAudit.$inferInsert;

/**
 * Feature flags for controlling platform features
 */
export const featureFlags = mysqlTable("feature_flags", {
  id: int("id").autoincrement().primaryKey(),
  name: varchar("name", { length: 100 }).notNull().unique(),
  description: text("description"),
  enabled: boolean("enabled").default(false).notNull(),
  environment: mysqlEnum("environment", ["all", "production", "staging", "development"]).default("all").notNull(),
  createdAt: timestamp("createdAt").defaultNow().notNull(),
  updatedAt: timestamp("updatedAt").defaultNow().onUpdateNow().notNull(),
  updatedBy: int("updatedBy").references(() => users.id),
});

export type FeatureFlag = typeof featureFlags.$inferSelect;
export type InsertFeatureFlag = typeof featureFlags.$inferInsert;

/**
 * Disbursement schedules for social benefit payments
 */
export const disbursementSchedules = mysqlTable("disbursement_schedules", {
  id: int("id").autoincrement().primaryKey(),
  programId: int("programId").notNull().references(() => benefitPrograms.id),
  scheduledDate: timestamp("scheduledDate").notNull(),
  amount: int("amount").notNull(), // in smallest currency unit (e.g., Kobo)
  beneficiaryCount: int("beneficiaryCount"),
  status: mysqlEnum("status", ["pending", "processing", "completed", "failed"]).default("pending").notNull(),
  metadata: json("metadata"), // Additional details (e.g., batch ID, notes)
  createdAt: timestamp("createdAt").defaultNow().notNull(),
  updatedAt: timestamp("updatedAt").defaultNow().onUpdateNow().notNull(),
  createdBy: int("createdBy").references(() => users.id),
});

export type DisbursementSchedule = typeof disbursementSchedules.$inferSelect;
export type InsertDisbursementSchedule = typeof disbursementSchedules.$inferInsert;

/**
 * Global MCC database for reference (optional, for auto-complete in UI)
 */
export const mccDatabase = mysqlTable("mcc_database", {
  id: int("id").autoincrement().primaryKey(),
  mccCode: varchar("mccCode", { length: 10 }).notNull().unique(),
  description: varchar("description", { length: 255 }).notNull(),
  category: varchar("category", { length: 100 }), // e.g., "Food & Groceries"
});

export type MccDatabaseEntry = typeof mccDatabase.$inferSelect;
export type InsertMccDatabaseEntry = typeof mccDatabase.$inferInsert;

/**
 * Admin action audit log for tracking all administrative actions
 */
export const adminActionAudit = mysqlTable("admin_action_audit", {
  id: int("id").autoincrement().primaryKey(),
  action: varchar("action", { length: 100 }).notNull(), // e.g., "role_change", "feature_flag_toggle"
  targetUserId: int("targetUserId").references(() => users.id), // User affected by the action
  details: json("details"), // Additional context (e.g., old/new role)
  justification: text("justification"),
  performedBy: int("performedBy").notNull().references(() => users.id),
  performedAt: timestamp("performedAt").defaultNow().notNull(),
});

export type AdminActionAudit = typeof adminActionAudit.$inferSelect;
export type InsertAdminActionAudit = typeof adminActionAudit.$inferInsert;

/**
 * Email notification preferences for administrators
 */
export const notificationPreferences = mysqlTable("notification_preferences", {
  id: int("id").autoincrement().primaryKey(),
  userId: int("userId").notNull(),
  notifyOnRoleChange: boolean("notifyOnRoleChange").default(true).notNull(),
  notifyOnBatchOperations: boolean("notifyOnBatchOperations").default(true).notNull(),
  notifyOnProgramChanges: boolean("notifyOnProgramChanges").default(false).notNull(),
  notifyOnMccRuleChanges: boolean("notifyOnMccRuleChanges").default(false).notNull(),
  emailAddress: varchar("emailAddress", { length: 320 }),
  createdAt: timestamp("createdAt").defaultNow().notNull(),
  updatedAt: timestamp("updatedAt").defaultNow().onUpdateNow().notNull(),
});

export type NotificationPreference = typeof notificationPreferences.$inferSelect;export type InsertNotificationPreferences = typeof notificationPreferences.$inferInsert;

export const savedSearchFilters = mysqlTable("saved_search_filters", {
  id: int("id").autoincrement().primaryKey(),
  userId: int("user_id").notNull(),
  name: varchar("name", { length: 255 }).notNull(),
  query: text("query").notNull(),
  entityType: varchar("entity_type", { length: 50 }).notNull(),
  createdAt: timestamp("created_at").defaultNow().notNull(),
});

export type SavedSearchFilter = typeof savedSearchFilters.$inferSelect;
export type InsertSavedSearchFilter = typeof savedSearchFilters.$inferInsert;

/**
 * Approval requests for critical operations requiring multi-level approval
 */
export const approvalRequests = mysqlTable("approval_requests", {
  id: int("id").autoincrement().primaryKey(),
  requestType: mysqlEnum("request_type", ["ROLE_CHANGE", "BATCH_PROGRAM_UPDATE", "BATCH_FLAG_TOGGLE", "PROGRAM_DELETE", "USER_DELETE"]).notNull(),
  requestedBy: int("requested_by").notNull(),
  targetId: int("target_id"),
  requestData: text("request_data").notNull(), // JSON string
  justification: text("justification").notNull(),
  status: mysqlEnum("status", ["pending", "approved", "rejected"]).default("pending").notNull(),
  requiredApprovals: int("required_approvals").default(2).notNull(),
  currentApprovals: int("current_approvals").default(0).notNull(),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  completedAt: timestamp("completed_at"),
});

export type ApprovalRequest = typeof approvalRequests.$inferSelect;
export type InsertApprovalRequest = typeof approvalRequests.$inferInsert;

/**
 * Individual approvals for approval requests
 */
export const approvals = mysqlTable("approvals", {
  id: int("id").autoincrement().primaryKey(),
  requestId: int("request_id").notNull(),
  approvedBy: int("approved_by").notNull(),
  decision: mysqlEnum("decision", ["approved", "rejected"]).notNull(),
  comment: text("comment"),
  createdAt: timestamp("created_at").defaultNow().notNull(),
});

export type Approval = typeof approvals.$inferSelect;
export type InsertApproval = typeof approvals.$inferInsert;

export const scheduledReports = mysqlTable("scheduled_reports", {
  id: int("id").autoincrement().primaryKey(),
  name: varchar("name", { length: 255 }).notNull(),
  description: text("description"),
  reportType: mysqlEnum("report_type", ["weekly", "monthly"]).notNull(),
  recipients: text("recipients").notNull(), // JSON array of email addresses
  isActive: int("is_active").default(1).notNull(),
  lastRunAt: timestamp("last_run_at"),
  nextRunAt: timestamp("next_run_at").notNull(),
  createdBy: int("created_by"),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export type ScheduledReport = typeof scheduledReports.$inferSelect;
export type InsertScheduledReport = typeof scheduledReports.$inferInsert;

export const reportHistory = mysqlTable("report_history", {
  id: int("id").autoincrement().primaryKey(),
  reportId: int("report_id").notNull(),
  generatedAt: timestamp("generated_at").defaultNow().notNull(),
  reportData: text("report_data").notNull(), // JSON summary data
  status: mysqlEnum("status", ["success", "failed"]).notNull(),
  errorMessage: text("error_message"),
});

export type ReportHistory = typeof reportHistory.$inferSelect;
export type InsertReportHistory = typeof reportHistory.$inferInsert;

/**
 * Program templates for quick duplication of benefit programs
 */
export const programTemplates = mysqlTable("program_templates", {
  id: int("id").autoincrement().primaryKey(),
  name: varchar("name", { length: 255 }).notNull(),
  description: text("description"),
  accountType: varchar("accountType", { length: 100 }).notNull(),
  mccRules: json("mccRules").$type<Array<{ mccCode: string; mccDescription: string | null }>>().notNull(),
  createdBy: int("createdBy").notNull().references(() => users.id),
  createdAt: timestamp("createdAt").defaultNow().notNull(),
  updatedAt: timestamp("updatedAt").defaultNow().onUpdateNow().notNull(),
});

export type ProgramTemplate = typeof programTemplates.$inferSelect;
export type InsertProgramTemplate = typeof programTemplates.$inferInsert;


/**
 * Beneficiaries enrolled in social protection programs
 */
export const beneficiaries = mysqlTable("beneficiaries", {
  id: int("id").autoincrement().primaryKey(),
  firstName: varchar("first_name", { length: 100 }).notNull(),
  lastName: varchar("last_name", { length: 100 }).notNull(),
  dateOfBirth: timestamp("date_of_birth").notNull(),
  nationalId: varchar("national_id", { length: 50 }).unique(),
  phoneNumber: varchar("phone_number", { length: 20 }),
  email: varchar("email", { length: 320 }),
  address: text("address"),
  city: varchar("city", { length: 100 }),
  state: varchar("state", { length: 100 }),
  postalCode: varchar("postal_code", { length: 20 }),
  enrollmentStatus: mysqlEnum("enrollment_status", ["pending", "approved", "rejected", "suspended"]).default("pending").notNull(),
  kycStatus: mysqlEnum("kyc_status", ["not_started", "in_progress", "completed", "failed"]).default("not_started").notNull(),
  enrolledAt: timestamp("enrolled_at").defaultNow().notNull(),
  approvedAt: timestamp("approved_at"),
  approvedBy: int("approved_by").references(() => users.id),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export type Beneficiary = typeof beneficiaries.$inferSelect;
export type InsertBeneficiary = typeof beneficiaries.$inferInsert;

/**
 * Program enrollments linking beneficiaries to specific benefit programs
 */
export const programEnrollments = mysqlTable("program_enrollments", {
  id: int("id").autoincrement().primaryKey(),
  beneficiaryId: int("beneficiary_id").notNull().references(() => beneficiaries.id, { onDelete: "cascade" }),
  programId: int("program_id").notNull().references(() => benefitPrograms.id, { onDelete: "cascade" }),
  enrollmentDate: timestamp("enrollment_date").defaultNow().notNull(),
  status: mysqlEnum("status", ["active", "inactive", "suspended"]).default("active").notNull(),
  monthlyAllocation: int("monthly_allocation").notNull(), // Amount in smallest currency unit
  lastDisbursement: timestamp("last_disbursement"),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export type ProgramEnrollment = typeof programEnrollments.$inferSelect;
export type InsertProgramEnrollment = typeof programEnrollments.$inferInsert;

/**
 * KYC documents uploaded by or for beneficiaries
 */
export const kycDocuments = mysqlTable("kyc_documents", {
  id: int("id").autoincrement().primaryKey(),
  beneficiaryId: int("beneficiary_id").notNull().references(() => beneficiaries.id, { onDelete: "cascade" }),
  documentType: mysqlEnum("document_type", ["national_id", "passport", "drivers_license", "birth_certificate", "proof_of_address", "photo", "other"]).notNull(),
  documentUrl: varchar("document_url", { length: 500 }).notNull(),
  fileKey: varchar("file_key", { length: 500 }).notNull(),
  fileName: varchar("file_name", { length: 255 }).notNull(),
  mimeType: varchar("mime_type", { length: 100 }),
  fileSize: int("file_size"),
  verificationStatus: mysqlEnum("verification_status", ["pending", "verified", "rejected"]).default("pending").notNull(),
  verifiedBy: int("verified_by").references(() => users.id),
  verifiedAt: timestamp("verified_at"),
  rejectionReason: text("rejection_reason"),
  uploadedAt: timestamp("uploaded_at").defaultNow().notNull(),
});

export type KycDocument = typeof kycDocuments.$inferSelect;
export type InsertKycDocument = typeof kycDocuments.$inferInsert;

/**
 * Benefit cards issued to beneficiaries
 */
export const benefitCards = mysqlTable("benefit_cards", {
  id: int("id").autoincrement().primaryKey(),
  beneficiaryId: int("beneficiary_id").notNull().references(() => beneficiaries.id, { onDelete: "cascade" }),
  cardNumber: varchar("card_number", { length: 20 }).notNull().unique(),
  cardType: mysqlEnum("card_type", ["physical", "virtual"]).notNull(),
  status: mysqlEnum("status", ["pending", "active", "blocked", "expired", "lost", "stolen"]).default("pending").notNull(),
  issuedAt: timestamp("issued_at").defaultNow().notNull(),
  expiresAt: timestamp("expires_at").notNull(),
  activatedAt: timestamp("activated_at"),
  blockedAt: timestamp("blocked_at"),
  blockReason: text("block_reason"),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export type BenefitCard = typeof benefitCards.$inferSelect;
export type InsertBenefitCard = typeof benefitCards.$inferInsert;

/**
 * Transaction records for monitoring and compliance
 */
export const transactions = mysqlTable("transactions", {
  id: int("id").autoincrement().primaryKey(),
  transactionId: varchar("transaction_id", { length: 100 }).notNull().unique(),
  beneficiaryId: int("beneficiary_id").notNull().references(() => beneficiaries.id),
  cardId: int("card_id").references(() => benefitCards.id),
  programId: int("program_id").notNull().references(() => benefitPrograms.id),
  merchantName: varchar("merchant_name", { length: 255 }),
  merchantId: varchar("merchant_id", { length: 100 }),
  mccCode: varchar("mcc_code", { length: 10 }).notNull(),
  mccDescription: varchar("mcc_description", { length: 255 }),
  amount: int("amount").notNull(), // Amount in smallest currency unit
  currency: varchar("currency", { length: 3 }).default("NGN").notNull(),
  transactionType: mysqlEnum("transaction_type", ["purchase", "refund", "reversal"]).notNull(),
  status: mysqlEnum("status", ["pending", "approved", "declined", "reversed"]).notNull(),
  complianceStatus: mysqlEnum("compliance_status", ["compliant", "non_compliant", "under_review"]).default("compliant").notNull(),
  declineReason: text("decline_reason"),
  fraudScore: int("fraud_score"), // 0-100
  fraudFlags: json("fraud_flags").$type<string[]>(),
  transactionDate: timestamp("transaction_date").notNull(),
  settledAt: timestamp("settled_at"),
  createdAt: timestamp("created_at").defaultNow().notNull(),
});

export type Transaction = typeof transactions.$inferSelect;
export type InsertTransaction = typeof transactions.$inferInsert;

/**
 * Fraud alerts and compliance violations
 */
export const fraudAlerts = mysqlTable("fraud_alerts", {
  id: int("id").autoincrement().primaryKey(),
  transactionId: int("transaction_id").references(() => transactions.id, { onDelete: "cascade" }),
  beneficiaryId: int("beneficiary_id").notNull().references(() => beneficiaries.id),
  alertType: mysqlEnum("alert_type", ["mcc_violation", "unusual_spending", "velocity_check", "duplicate_transaction", "suspicious_merchant", "other"]).notNull(),
  severity: mysqlEnum("severity", ["low", "medium", "high", "critical"]).notNull(),
  description: text("description").notNull(),
  status: mysqlEnum("status", ["open", "investigating", "resolved", "false_positive"]).default("open").notNull(),
  assignedTo: int("assigned_to").references(() => users.id),
  resolvedBy: int("resolved_by").references(() => users.id),
  resolvedAt: timestamp("resolved_at"),
  resolution: text("resolution"),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export type FraudAlert = typeof fraudAlerts.$inferSelect;
export type InsertFraudAlert = typeof fraudAlerts.$inferInsert;

// Middleware Monitoring Tables
export const middlewareMetrics = mysqlTable("middleware_metrics", {
  id: int("id").autoincrement().primaryKey(),
  component: varchar("component", { length: 50 }).notNull(), // redis, kafka, apisix, etc.
  metricType: varchar("metric_type", { length: 100 }).notNull(), // cpu, memory, throughput, latency
  metricValue: float("metric_value").notNull(),
  unit: varchar("unit", { length: 20 }), // percent, mb, req/sec
  timestamp: timestamp("timestamp").defaultNow().notNull(),
  metadata: text("metadata"), // JSON for additional context
});

export const middlewareAlerts = mysqlTable("middleware_alerts", {
  id: int("id").autoincrement().primaryKey(),
  component: varchar("component", { length: 50 }).notNull(),
  alertType: varchar("alert_type", { length: 50 }).notNull(), // health, performance, error
  severity: mysqlEnum("severity", ["info", "warning", "critical"]).notNull(),
  message: text("message").notNull(),
  details: text("details"), // JSON
  status: mysqlEnum("status", ["active", "acknowledged", "resolved"]).default("active").notNull(),
  triggeredAt: timestamp("triggered_at").defaultNow().notNull(),
  acknowledgedAt: timestamp("acknowledged_at"),
  acknowledgedBy: int("acknowledged_by"),
  resolvedAt: timestamp("resolved_at"),
  resolvedBy: int("resolved_by"),
});

export const middlewareHealthChecks = mysqlTable("middleware_health_checks", {
  id: int("id").autoincrement().primaryKey(),
  component: varchar("component", { length: 50 }).notNull(),
  status: mysqlEnum("status", ["healthy", "degraded", "unhealthy"]).notNull(),
  responseTime: int("response_time"), // milliseconds
  errorMessage: text("error_message"),
  checkedAt: timestamp("checked_at").defaultNow().notNull(),
});

// Event Replay System Tables
export const eventSnapshots = mysqlTable("event_snapshots", {
  id: int("id").autoincrement().primaryKey(),
  snapshotName: varchar("snapshot_name", { length: 255 }).notNull(),
  description: text("description"),
  eventCount: int("event_count").notNull(),
  startOffset: varchar("start_offset", { length: 100 }).notNull(),
  endOffset: varchar("end_offset", { length: 100 }).notNull(),
  topics: text("topics").notNull(), // JSON array of topics
  createdBy: int("created_by").notNull(),
  createdAt: timestamp("created_at").defaultNow().notNull(),
});

export const eventReplays = mysqlTable("event_replays", {
  id: int("id").autoincrement().primaryKey(),
  replayName: varchar("replay_name", { length: 255 }).notNull(),
  snapshotId: int("snapshot_id"),
  topics: text("topics").notNull(), // JSON array
  startOffset: varchar("start_offset", { length: 100 }),
  endOffset: varchar("end_offset", { length: 100 }),
  eventFilter: text("event_filter"), // JSON filter criteria
  status: mysqlEnum("status", ["pending", "running", "completed", "failed", "cancelled"]).default("pending").notNull(),
  progress: int("progress").default(0).notNull(), // percentage
  eventsProcessed: int("events_processed").default(0).notNull(),
  eventsTotal: int("events_total").default(0).notNull(),
  errorMessage: text("error_message"),
  startedBy: int("started_by").notNull(),
  startedAt: timestamp("started_at").defaultNow().notNull(),
  completedAt: timestamp("completed_at"),
});

export const eventReplayLogs = mysqlTable("event_replay_logs", {
  id: int("id").autoincrement().primaryKey(),
  replayId: int("replay_id").notNull(),
  eventOffset: varchar("event_offset", { length: 100 }).notNull(),
  eventType: varchar("event_type", { length: 100 }).notNull(),
  eventData: text("event_data"), // JSON
  processedAt: timestamp("processed_at").defaultNow().notNull(),
  success: boolean("success").notNull(),
  errorMessage: text("error_message"),
});

// Multi-tenancy Tables
export const tenants = mysqlTable("tenants", {
  id: int("id").autoincrement().primaryKey(),
  tenantCode: varchar("tenant_code", { length: 50 }).notNull().unique(), // unique identifier
  tenantName: varchar("tenant_name", { length: 255 }).notNull(),
  status: mysqlEnum("status", ["active", "suspended", "inactive"]).default("active").notNull(),
  configuration: text("configuration"), // JSON tenant-specific config
  kafkaTopicPrefix: varchar("kafka_topic_prefix", { length: 50 }).notNull(), // e.g., "tenant1_"
  redisNamespace: varchar("redis_namespace", { length: 50 }).notNull(), // e.g., "tenant1:"
  permifyTenantId: varchar("permify_tenant_id", { length: 100 }),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export const tenantUsers = mysqlTable("tenant_users", {
  id: int("id").autoincrement().primaryKey(),
  tenantId: int("tenant_id").notNull(),
  userId: int("user_id").notNull(),
  role: mysqlEnum("role", ["owner", "admin", "member"]).default("member").notNull(),
  joinedAt: timestamp("joined_at").defaultNow().notNull(),
});

export const tenantUsage = mysqlTable("tenant_usage", {
  id: int("id").autoincrement().primaryKey(),
  tenantId: int("tenant_id").notNull(),
  metricType: varchar("metric_type", { length: 100 }).notNull(), // api_calls, storage_mb, events_count
  metricValue: float("metric_value").notNull(),
  period: varchar("period", { length: 20 }).notNull(), // YYYY-MM-DD
  recordedAt: timestamp("recorded_at").defaultNow().notNull(),
});

// Blockchain Integration Tables
export const blockchainTransactions = mysqlTable("blockchain_transactions", {
  id: int("id").autoincrement().primaryKey(),
  transactionId: varchar("transaction_id", { length: 100 }).notNull().unique(),
  blockHash: varchar("block_hash", { length: 255 }).notNull(),
  blockNumber: int("block_number").notNull(),
  transactionHash: varchar("transaction_hash", { length: 255 }).notNull().unique(),
  chaincodeName: varchar("chaincode_name", { length: 100 }).notNull(),
  functionName: varchar("function_name", { length: 100 }).notNull(),
  payload: text("payload").notNull(), // JSON
  status: mysqlEnum("status", ["pending", "confirmed", "failed"]).default("pending").notNull(),
  timestamp: timestamp("timestamp").defaultNow().notNull(),
});


export const blockchainIdentities = mysqlTable("blockchain_identities", {
  id: int("id").autoincrement().primaryKey(),
  userId: int("user_id").notNull().unique(),
  did: varchar("did", { length: 255 }).notNull().unique(), // Decentralized Identifier
  publicKey: text("public_key").notNull(),
  verificationMethod: text("verification_method"), // JSON
  status: mysqlEnum("status", ["active", "revoked"]).default("active").notNull(),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

// Machine Learning Pipeline Tables
export const mlModels = mysqlTable("ml_models", {
  id: int("id").autoincrement().primaryKey(),
  modelName: varchar("model_name", { length: 100 }).notNull(),
  modelType: varchar("model_type", { length: 50 }).notNull(), // fraud_detection, enrollment_prediction, budget_optimization
  version: varchar("version", { length: 20 }).notNull(),
  algorithm: varchar("algorithm", { length: 50 }).notNull(),
  hyperparameters: text("hyperparameters"), // JSON
  metrics: text("metrics"), // JSON (accuracy, precision, recall, f1)
  status: mysqlEnum("status", ["training", "active", "archived"]).default("training").notNull(),
  trainedAt: timestamp("trained_at").defaultNow().notNull(),
  createdBy: int("created_by").notNull(),
});

export const mlTrainingJobs = mysqlTable("ml_training_jobs", {
  id: int("id").autoincrement().primaryKey(),
  modelId: int("model_id").notNull(),
  jobType: varchar("job_type", { length: 50 }).notNull(), // initial, retrain, ab_test
  datasetSize: int("dataset_size").notNull(),
  trainingDuration: int("training_duration"), // seconds
  status: mysqlEnum("status", ["pending", "running", "completed", "failed"]).default("pending").notNull(),
  errorMessage: text("error_message"),
  startedAt: timestamp("started_at"),
  completedAt: timestamp("completed_at"),
  createdAt: timestamp("created_at").defaultNow().notNull(),
});

export const mlFeatures = mysqlTable("ml_features", {
  id: int("id").autoincrement().primaryKey(),
  featureName: varchar("feature_name", { length: 100 }).notNull().unique(),
  featureType: varchar("feature_type", { length: 50 }).notNull(), // numerical, categorical, boolean
  description: text("description"),
  importance: float("importance"), // Feature importance score
  modelType: varchar("model_type", { length: 50 }).notNull(),
  createdAt: timestamp("created_at").defaultNow().notNull(),
});

export const mlAbTests = mysqlTable("ml_ab_tests", {
  id: int("id").autoincrement().primaryKey(),
  testName: varchar("test_name", { length: 100 }).notNull(),
  modelAId: int("model_a_id").notNull(),
  modelBId: int("model_b_id").notNull(),
  trafficSplit: int("traffic_split").default(50).notNull(), // Percentage to model A
  status: mysqlEnum("status", ["running", "completed", "cancelled"]).default("running").notNull(),
  startedAt: timestamp("started_at").defaultNow().notNull(),
  endedAt: timestamp("ended_at"),
  results: text("results"), // JSON
});

/**
 * Workflow Monitoring Schema
 * Tracks Temporal workflow executions, activities, and performance metrics
 */

export const workflowExecutions = mysqlTable("workflow_executions", {
  id: int("id").autoincrement().primaryKey(),
  workflowId: varchar("workflow_id", { length: 255 }).notNull().unique(),
  workflowType: varchar("workflow_type", { length: 100 }).notNull(),
  runId: varchar("run_id", { length: 255 }).notNull(),
  status: mysqlEnum("status", ["running", "completed", "failed", "cancelled", "timeout"]).notNull(),
  input: json("input"),
  result: json("result"),
  error: text("error"),
  startTime: timestamp("start_time").notNull(),
  endTime: timestamp("end_time"),
  duration: int("duration"),
  initiatedBy: varchar("initiated_by", { length: 255 }),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export const activityExecutions = mysqlTable("activity_executions", {
  id: int("id").autoincrement().primaryKey(),
  workflowId: varchar("workflow_id", { length: 255 }).notNull(),
  activityId: varchar("activity_id", { length: 255 }).notNull(),
  activityType: varchar("activity_type", { length: 100 }).notNull(),
  status: mysqlEnum("status", ["scheduled", "started", "completed", "failed", "timeout", "cancelled"]).notNull(),
  input: json("input"),
  result: json("result"),
  error: text("error"),
  attempt: int("attempt").default(1).notNull(),
  startTime: timestamp("start_time").notNull(),
  endTime: timestamp("end_time"),
  duration: int("duration"),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export const workflowMetrics = mysqlTable("workflow_metrics", {
  id: int("id").autoincrement().primaryKey(),
  workflowType: varchar("workflow_type", { length: 100 }).notNull(),
  date: timestamp("date").notNull(),
  totalExecutions: int("total_executions").default(0).notNull(),
  successfulExecutions: int("successful_executions").default(0).notNull(),
  failedExecutions: int("failed_executions").default(0).notNull(),
  averageDuration: int("average_duration"),
  minDuration: int("min_duration"),
  maxDuration: int("max_duration"),
  p50Duration: int("p50_duration"),
  p95Duration: int("p95_duration"),
  p99Duration: int("p99_duration"),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export const workflowAlerts = mysqlTable("workflow_alerts", {
  id: int("id").autoincrement().primaryKey(),
  workflowId: varchar("workflow_id", { length: 255 }).notNull(),
  alertType: mysqlEnum("alert_type", ["timeout", "high_failure_rate", "slow_execution", "stuck_workflow", "resource_exhaustion"]).notNull(),
  severity: mysqlEnum("severity", ["low", "medium", "high", "critical"]).notNull(),
  message: text("message").notNull(),
  resolved: int("resolved").default(0).notNull(),
  resolvedAt: timestamp("resolved_at"),
  resolvedBy: varchar("resolved_by", { length: 255 }),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export type WorkflowExecution = typeof workflowExecutions.$inferSelect;
export type InsertWorkflowExecution = typeof workflowExecutions.$inferInsert;

export type ActivityExecution = typeof activityExecutions.$inferSelect;
export type InsertActivityExecution = typeof activityExecutions.$inferInsert;

export type WorkflowMetric = typeof workflowMetrics.$inferSelect;
export type InsertWorkflowMetric = typeof workflowMetrics.$inferInsert;

export type WorkflowAlert = typeof workflowAlerts.$inferSelect;
export type InsertWorkflowAlert = typeof workflowAlerts.$inferInsert;
