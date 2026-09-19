import { int, mysqlEnum, mysqlTable, text, timestamp, varchar, json } from "drizzle-orm/mysql-core";

/**
 * Workflow Templates Schema
 * Stores reusable workflow configurations for quick program deployment
 */

export const workflowTemplates = mysqlTable("workflow_templates", {
  id: int("id").autoincrement().primaryKey(),
  name: varchar("name", { length: 255 }).notNull(),
  description: text("description"),
  workflowType: varchar("workflow_type", { length: 100 }).notNull(),
  category: varchar("category", { length: 100 }), // e.g., "cash_transfer", "food_assistance", "emergency_relief"
  
  // Template configuration
  configuration: json("configuration").notNull(), // Workflow-specific configuration
  defaultParameters: json("default_parameters"), // Default input parameters
  requiredParameters: json("required_parameters"), // Required parameters for deployment
  
  // Metadata
  version: varchar("version", { length: 50 }).notNull().default("1.0.0"),
  tags: json("tags"), // Array of tags for search/filtering
  
  // Usage tracking
  usageCount: int("usage_count").default(0).notNull(),
  lastUsedAt: timestamp("last_used_at"),
  
  // Visibility and permissions
  visibility: mysqlEnum("visibility", ["private", "team", "public"]).notNull().default("private"),
  ownerId: int("owner_id").notNull(),
  sharedWith: json("shared_with"), // Array of user IDs or team IDs
  
  // Status
  status: mysqlEnum("status", ["draft", "active", "archived", "deprecated"]).notNull().default("draft"),
  publishedAt: timestamp("published_at"),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
  createdBy: int("created_by").notNull(),
  updatedBy: int("updated_by"),
});

export const workflowTemplateVersions = mysqlTable("workflow_template_versions", {
  id: int("id").autoincrement().primaryKey(),
  templateId: int("template_id").notNull(),
  version: varchar("version", { length: 50 }).notNull(),
  
  // Version content
  configuration: json("configuration").notNull(),
  defaultParameters: json("default_parameters"),
  requiredParameters: json("required_parameters"),
  
  // Change tracking
  changeLog: text("change_log"),
  changedBy: int("changed_by").notNull(),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
});

export const workflowTemplateDeployments = mysqlTable("workflow_template_deployments", {
  id: int("id").autoincrement().primaryKey(),
  templateId: int("template_id").notNull(),
  templateVersion: varchar("template_version", { length: 50 }).notNull(),
  
  // Deployment details
  workflowId: varchar("workflow_id", { length: 255 }).notNull(),
  deployedParameters: json("deployed_parameters"),
  
  // Status tracking
  status: mysqlEnum("status", ["pending", "active", "completed", "failed", "cancelled"]).notNull(),
  startedAt: timestamp("started_at"),
  completedAt: timestamp("completed_at"),
  
  // Deployment metadata
  deployedBy: int("deployed_by").notNull(),
  deploymentNotes: text("deployment_notes"),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export const workflowTemplateCategories = mysqlTable("workflow_template_categories", {
  id: int("id").autoincrement().primaryKey(),
  name: varchar("name", { length: 100 }).notNull().unique(),
  description: text("description"),
  icon: varchar("icon", { length: 50 }), // Icon name for UI
  color: varchar("color", { length: 50 }), // Color code for UI
  displayOrder: int("display_order").default(0).notNull(),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().onUpdateNow().notNull(),
});

export const workflowTemplateFavorites = mysqlTable("workflow_template_favorites", {
  id: int("id").autoincrement().primaryKey(),
  templateId: int("template_id").notNull(),
  userId: int("user_id").notNull(),
  
  createdAt: timestamp("created_at").defaultNow().notNull(),
});

export type WorkflowTemplate = typeof workflowTemplates.$inferSelect;
export type InsertWorkflowTemplate = typeof workflowTemplates.$inferInsert;

export type WorkflowTemplateVersion = typeof workflowTemplateVersions.$inferSelect;
export type InsertWorkflowTemplateVersion = typeof workflowTemplateVersions.$inferInsert;

export type WorkflowTemplateDeployment = typeof workflowTemplateDeployments.$inferSelect;
export type InsertWorkflowTemplateDeployment = typeof workflowTemplateDeployments.$inferInsert;

export type WorkflowTemplateCategory = typeof workflowTemplateCategories.$inferSelect;
export type InsertWorkflowTemplateCategory = typeof workflowTemplateCategories.$inferInsert;

export type WorkflowTemplateFavorite = typeof workflowTemplateFavorites.$inferSelect;
export type InsertWorkflowTemplateFavorite = typeof workflowTemplateFavorites.$inferInsert;
