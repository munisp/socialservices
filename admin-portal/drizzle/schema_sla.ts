import { int, mysqlEnum, mysqlTable, text, timestamp, varchar, json } from "drizzle-orm/mysql-core";

/**
 * SLA (Service Level Agreement) Monitoring Schema
 * Tracks workflow SLA configurations, breaches, and escalations
 */

export const slaConfigurations = mysqlTable("sla_configurations", {
  id: int("id").autoincrement().primaryKey(),
  workflowType: varchar("workflow_type", { length: 100 }).notNull().unique(),
  name: varchar("name", { length: 255 }).notNull(),
  description: text("description"),
  
  // Thresholds
  warningThresholdMs: int("warning_threshold_ms").notNull(), // Warning after X milliseconds
  criticalThresholdMs: int("critical_threshold_ms").notNull(), // Critical after X milliseconds
  maxDurationMs: int("max_duration_ms").notNull(), // Maximum allowed duration
  
  // Escalation settings
  escalationEnabled: int("escalation_enabled").default(1).notNull(),
  escalationLevel1: varchar("escalation_level1", { length: 255 }), // Email/user ID for level 1
  escalationLevel2: varchar("escalation_level2", { length: 255 }), // Email/user ID for level 2
  escalationLevel3: varchar("escalation_level3", { length: 255 }), // Email/user ID for level 3
  
  // Notification settings
  notifyOnWarning: int("notify_on_warning").default(1).notNull(),
  notifyOnCritical: int("notify_on_critical").default(1).notNull(),
  notifyOnBreach: int("notify_on_breach").default(1).notNull(),
  
  // Status
  enabled: int("enabled").default(1).notNull(),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
  createdBy: varchar("created_by", { length: 255 }),
});

export const slaBreaches = mysqlTable("sla_breaches", {
  id: int("id").autoincrement().primaryKey(),
  workflowId: varchar("workflow_id", { length: 255 }).notNull(),
  workflowType: varchar("workflow_type", { length: 100 }).notNull(),
  slaConfigId: int("sla_config_id").notNull(),
  
  // Breach details
  breachType: mysqlEnum("breach_type", ["warning", "critical", "exceeded"]).notNull(),
  breachTime: timestamp("breach_time").notNull(),
  expectedCompletionTime: timestamp("expected_completion_time").notNull(),
  actualDuration: int("actual_duration"), // Milliseconds
  thresholdDuration: int("threshold_duration").notNull(), // Milliseconds
  
  // Status
  status: mysqlEnum("status", ["active", "resolved", "escalated", "acknowledged"]).notNull().default("active"),
  resolvedAt: timestamp("resolved_at"),
  resolvedBy: varchar("resolved_by", { length: 255 }),
  resolution: text("resolution"),
  
  // Escalation tracking
  escalationLevel: int("escalation_level").default(0).notNull(),
  escalatedAt: timestamp("escalated_at"),
  escalatedTo: varchar("escalated_to", { length: 255 }),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export const slaEscalations = mysqlTable("sla_escalations", {
  id: int("id").autoincrement().primaryKey(),
  slaBreachId: int("sla_breach_id").notNull(),
  workflowId: varchar("workflow_id", { length: 255 }).notNull(),
  
  // Escalation details
  escalationLevel: int("escalation_level").notNull(),
  escalatedTo: varchar("escalated_to", { length: 255 }).notNull(),
  escalatedBy: varchar("escalated_by", { length: 255 }),
  escalationReason: text("escalation_reason"),
  
  // Status
  status: mysqlEnum("status", ["pending", "acknowledged", "resolved", "failed"]).notNull().default("pending"),
  acknowledgedAt: timestamp("acknowledged_at"),
  acknowledgedBy: varchar("acknowledged_by", { length: 255 }),
  resolvedAt: timestamp("resolved_at"),
  resolvedBy: varchar("resolved_by", { length: 255 }),
  
  // Notification tracking
  notificationSent: int("notification_sent").default(0).notNull(),
  notificationSentAt: timestamp("notification_sent_at"),
  notificationMethod: varchar("notification_method", { length: 50 }), // email, sms, slack, etc.
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export const slaMetrics = mysqlTable("sla_metrics", {
  id: int("id").autoincrement().primaryKey(),
  workflowType: varchar("workflow_type", { length: 100 }).notNull(),
  date: timestamp("date").notNull(),
  
  // Compliance metrics
  totalWorkflows: int("total_workflows").default(0).notNull(),
  withinSLA: int("within_sla").default(0).notNull(),
  warningBreaches: int("warning_breaches").default(0).notNull(),
  criticalBreaches: int("critical_breaches").default(0).notNull(),
  exceededBreaches: int("exceeded_breaches").default(0).notNull(),
  
  // Performance metrics
  averageDuration: int("average_duration"),
  p50Duration: int("p50_duration"),
  p95Duration: int("p95_duration"),
  p99Duration: int("p99_duration"),
  
  // Compliance rate
  complianceRate: int("compliance_rate"), // Percentage (0-100)
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export type SLAConfiguration = typeof slaConfigurations.$inferSelect;
export type InsertSLAConfiguration = typeof slaConfigurations.$inferInsert;

export type SLABreach = typeof slaBreaches.$inferSelect;
export type InsertSLABreach = typeof slaBreaches.$inferInsert;

export type SLAEscalation = typeof slaEscalations.$inferSelect;
export type InsertSLAEscalation = typeof slaEscalations.$inferInsert;

export type SLAMetric = typeof slaMetrics.$inferSelect;
export type InsertSLAMetric = typeof slaMetrics.$inferInsert;
