import { int, mysqlTable, text, timestamp, varchar, decimal, mysqlEnum, json } from "drizzle-orm/mysql-core";

/**
 * Workflow Monitoring Schema
 * Tracks Temporal workflow executions, activities, and performance metrics
 */

export const workflowExecutions = mysqlTable("workflow_executions", {
  id: int("id").autoincrement().primaryKey(),
  workflowId: varchar("workflow_id", { length: 255 }).notNull().unique(),
  workflowType: varchar("workflow_type", { length: 100 }).notNull(), // journey1_enrollment, journey2_kyc, etc.
  runId: varchar("run_id", { length: 255 }).notNull(),
  status: mysqlEnum("status", ["running", "completed", "failed", "cancelled", "timeout"]).notNull(),
  input: json("input"), // Workflow input parameters
  result: json("result"), // Workflow result
  error: text("error"), // Error message if failed
  startTime: timestamp("start_time").notNull(),
  endTime: timestamp("end_time"),
  duration: int("duration"), // Duration in milliseconds
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
  duration: int("duration"), // Duration in milliseconds
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
  averageDuration: int("average_duration"), // Average duration in milliseconds
  minDuration: int("min_duration"),
  maxDuration: int("max_duration"),
  p50Duration: int("p50_duration"), // Median
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
  resolved: int("resolved").default(0).notNull(), // 0 = not resolved, 1 = resolved
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
