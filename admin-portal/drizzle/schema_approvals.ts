import { int, mysqlEnum, mysqlTable, text, timestamp, varchar, json } from "drizzle-orm/mysql-core";

/**
 * Approval Workflows Schema
 * Multi-level approval system for sensitive operations
 */

export const approvalChains = mysqlTable("approval_chains", {
  id: int("id").autoincrement().primaryKey(),
  name: varchar("name", { length: 255 }).notNull(),
  description: text("description"),
  operationType: varchar("operation_type", { length: 100 }).notNull(), // e.g., "high_value_disbursement", "beneficiary_deletion"
  
  // Chain configuration
  levels: json("levels").notNull(), // Array of approval levels with approvers
  requiresAllApprovers: int("requires_all_approvers").default(0).notNull(), // vs any approver
  allowDelegation: int("allow_delegation").default(1).notNull(),
  autoEscalationHours: int("auto_escalation_hours").default(24),
  
  // Conditions for triggering
  conditions: json("conditions"), // e.g., { "amount": { "gt": 10000 } }
  
  // Status
  enabled: int("enabled").default(1).notNull(),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
  createdBy: int("created_by").notNull(),
});

export const approvalRequests = mysqlTable("approval_requests", {
  id: int("id").autoincrement().primaryKey(),
  chainId: int("chain_id").notNull(),
  
  // Request details
  operationType: varchar("operation_type", { length: 100 }).notNull(),
  operationData: json("operation_data").notNull(), // Data for the operation being approved
  requestReason: text("request_reason"),
  
  // Workflow context
  workflowId: varchar("workflow_id", { length: 255 }),
  relatedEntityType: varchar("related_entity_type", { length: 100 }), // e.g., "disbursement", "beneficiary"
  relatedEntityId: varchar("related_entity_id", { length: 255 }),
  
  // Status tracking
  status: mysqlEnum("status", ["pending", "approved", "rejected", "cancelled", "expired"]).notNull().default("pending"),
  currentLevel: int("current_level").default(1).notNull(),
  totalLevels: int("total_levels").notNull(),
  
  // Timestamps
  requestedAt: timestamp("requested_at").defaultNow().notNull(),
  completedAt: timestamp("completed_at"),
  expiresAt: timestamp("expires_at"),
  
  // Requester
  requestedBy: int("requested_by").notNull(),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export const approvalActions = mysqlTable("approval_actions", {
  id: int("id").autoincrement().primaryKey(),
  requestId: int("request_id").notNull(),
  level: int("level").notNull(),
  
  // Action details
  action: mysqlEnum("action", ["approved", "rejected", "delegated", "escalated"]).notNull(),
  comments: text("comments"),
  attachments: json("attachments"), // Array of file URLs
  
  // Approver
  approverId: int("approver_id").notNull(),
  approverName: varchar("approver_name", { length: 255 }),
  approverRole: varchar("approver_role", { length: 100 }),
  
  // Delegation (if applicable)
  delegatedTo: int("delegated_to"),
  delegationReason: text("delegation_reason"),
  
  // Timestamps
  actionedAt: timestamp("actioned_at").defaultNow().notNull(),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
});

export const approvalDelegations = mysqlTable("approval_delegations", {
  id: int("id").autoincrement().primaryKey(),
  delegatorId: int("delegator_id").notNull(),
  delegateId: int("delegate_id").notNull(),
  
  // Delegation scope
  operationType: varchar("operation_type", { length: 100 }), // null = all types
  startDate: timestamp("start_date").notNull(),
  endDate: timestamp("end_date").notNull(),
  
  // Status
  active: int("active").default(1).notNull(),
  reason: text("reason"),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export const approvalNotifications = mysqlTable("approval_notifications", {
  id: int("id").autoincrement().primaryKey(),
  requestId: int("request_id").notNull(),
  recipientId: int("recipient_id").notNull(),
  
  // Notification details
  notificationType: mysqlEnum("notification_type", ["approval_required", "approved", "rejected", "escalated", "expired"]).notNull(),
  message: text("message").notNull(),
  
  // Status
  sent: int("sent").default(0).notNull(),
  sentAt: timestamp("sent_at"),
  read: int("read").default(0).notNull(),
  readAt: timestamp("read_at"),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
});

export const approvalRules = mysqlTable("approval_rules", {
  id: int("id").autoincrement().primaryKey(),
  name: varchar("name", { length: 255 }).notNull(),
  description: text("description"),
  
  // Rule definition
  operationType: varchar("operation_type", { length: 100 }).notNull(),
  conditions: json("conditions").notNull(), // JSON rules engine format
  chainId: int("chain_id").notNull(), // Which approval chain to trigger
  
  // Priority (higher number = higher priority)
  priority: int("priority").default(0).notNull(),
  
  // Status
  enabled: int("enabled").default(1).notNull(),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
  createdBy: int("created_by").notNull(),
});

export type ApprovalChain = typeof approvalChains.$inferSelect;
export type InsertApprovalChain = typeof approvalChains.$inferInsert;

export type ApprovalRequest = typeof approvalRequests.$inferSelect;
export type InsertApprovalRequest = typeof approvalRequests.$inferInsert;

export type ApprovalAction = typeof approvalActions.$inferSelect;
export type InsertApprovalAction = typeof approvalActions.$inferInsert;

export type ApprovalDelegation = typeof approvalDelegations.$inferSelect;
export type InsertApprovalDelegation = typeof approvalDelegations.$inferInsert;

export type ApprovalNotification = typeof approvalNotifications.$inferSelect;
export type InsertApprovalNotification = typeof approvalNotifications.$inferInsert;

export type ApprovalRule = typeof approvalRules.$inferSelect;
export type InsertApprovalRule = typeof approvalRules.$inferInsert;
