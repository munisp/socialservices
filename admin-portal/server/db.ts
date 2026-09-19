import { eq, and, desc, sql, gte, lte } from "drizzle-orm";
import { drizzle } from "drizzle-orm/mysql2";
import {
  InsertUser,
  users,
  User,
  benefitPrograms,
  InsertBenefitProgram,
  BenefitProgram,
  mccRules,
  InsertMccRule,
  MccRule,
  mccRuleAudit,
  InsertMccRuleAudit,
  featureFlags,
  InsertFeatureFlag,
  FeatureFlag,
  disbursementSchedules,
  InsertDisbursementSchedule,
  DisbursementSchedule,
  mccDatabase,
  InsertMccDatabaseEntry,
  MccDatabaseEntry,
  adminActionAudit,
  InsertAdminActionAudit,
  AdminActionAudit,
  programTemplates,
  InsertProgramTemplate,
  ProgramTemplate,
  scheduledReports,
  InsertScheduledReport,
  ScheduledReport,
  reportHistory,
  InsertReportHistory,
  ReportHistory,
  notificationPreferences,
  InsertNotificationPreferences,
  NotificationPreference,
  savedSearchFilters,
  InsertSavedSearchFilter,
  SavedSearchFilter,
} from "../drizzle/schema";
import { ENV } from "./_core/env";

let _db: ReturnType<typeof drizzle> | null = null;

export async function getDb() {
  if (!_db && process.env.DATABASE_URL) {
    try {
      _db = drizzle(process.env.DATABASE_URL);
    } catch (error) {
      console.warn("[Database] Failed to connect:", error);
      _db = null;
    }
  }
  return _db;
}

// ===== User Management =====

export async function upsertUser(user: InsertUser): Promise<void> {
  if (!user.openId) {
    throw new Error("User openId is required for upsert");
  }

  const db = await getDb();
  if (!db) {
    console.warn("[Database] Cannot upsert user: database not available");
    return;
  }

  try {
    const values: InsertUser = {
      openId: user.openId,
    };
    const updateSet: Record<string, unknown> = {};

    const textFields = ["name", "email", "loginMethod"] as const;
    type TextField = (typeof textFields)[number];

    const assignNullable = (field: TextField) => {
      const value = user[field];
      if (value === undefined) return;
      const normalized = value ?? null;
      values[field] = normalized;
      updateSet[field] = normalized;
    };

    textFields.forEach(assignNullable);

    if (user.lastSignedIn !== undefined) {
      values.lastSignedIn = user.lastSignedIn;
      updateSet.lastSignedIn = user.lastSignedIn;
    }
    if (user.role !== undefined) {
      values.role = user.role;
      updateSet.role = user.role;
    } else if (user.openId === ENV.ownerOpenId) {
      values.role = "admin";
      updateSet.role = "admin";
    }

    if (!values.lastSignedIn) {
      values.lastSignedIn = new Date();
    }

    if (Object.keys(updateSet).length === 0) {
      updateSet.lastSignedIn = new Date();
    }

    await db.insert(users).values(values).onDuplicateKeyUpdate({
      set: updateSet,
    });
  } catch (error) {
    console.error("[Database] Failed to upsert user:", error);
    throw error;
  }
}

export async function getUserByOpenId(openId: string) {
  const db = await getDb();
  if (!db) {
    console.warn("[Database] Cannot get user: database not available");
    return undefined;
  }

  const result = await db.select().from(users).where(eq(users.openId, openId)).limit(1);

  return result.length > 0 ? result[0] : undefined;
}

// ===== Benefit Programs =====

export async function getAllBenefitPrograms(): Promise<BenefitProgram[]> {
  const db = await getDb();
  if (!db) return [];
  return db.select().from(benefitPrograms).orderBy(desc(benefitPrograms.createdAt));
}

export async function getBenefitProgramById(id: number): Promise<BenefitProgram | undefined> {
  const db = await getDb();
  if (!db) return undefined;
  const result = await db.select().from(benefitPrograms).where(eq(benefitPrograms.id, id)).limit(1);
  return result[0];
}

export async function createBenefitProgram(program: InsertBenefitProgram): Promise<BenefitProgram> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  const result = await db.insert(benefitPrograms).values(program);
  return getBenefitProgramById(Number(result[0].insertId)) as Promise<BenefitProgram>;
}

export async function updateBenefitProgram(id: number, data: Partial<InsertBenefitProgram>): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.update(benefitPrograms).set(data).where(eq(benefitPrograms.id, id));
}

export async function updateBenefitProgramStatus(id: number, isActive: boolean): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  const status = isActive ? "active" : "inactive";
  await db.update(benefitPrograms).set({ status }).where(eq(benefitPrograms.id, id));
}

// ===== MCC Rules =====

export async function getMccRulesByProgramId(programId: number): Promise<MccRule[]> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  return await db.select().from(mccRules).where(eq(mccRules.programId, programId));
}

export async function getAllMccRules(): Promise<MccRule[]> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  return await db.select().from(mccRules);
}

export async function addMccRule(rule: InsertMccRule): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.insert(mccRules).values(rule);
}

export async function removeMccRule(programId: number, mccCode: string): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.delete(mccRules).where(and(eq(mccRules.programId, programId), eq(mccRules.mccCode, mccCode)));
}

// ===== MCC Rule Audit =====

export async function logMccRuleAudit(audit: InsertMccRuleAudit): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.insert(mccRuleAudit).values(audit);
}

export async function getMccRuleAuditByProgramId(programId: number): Promise<typeof mccRuleAudit.$inferSelect[]> {
  const db = await getDb();
  if (!db) return [];
  return db.select().from(mccRuleAudit).where(eq(mccRuleAudit.programId, programId)).orderBy(desc(mccRuleAudit.performedAt));
}

// ===== Feature Flags =====

export async function getAllFeatureFlags(): Promise<FeatureFlag[]> {
  const db = await getDb();
  if (!db) return [];
  return db.select().from(featureFlags).orderBy(featureFlags.name);
}

export async function getFeatureFlagByName(name: string): Promise<FeatureFlag | undefined> {
  const db = await getDb();
  if (!db) return undefined;
  const result = await db.select().from(featureFlags).where(eq(featureFlags.name, name)).limit(1);
  return result[0];
}

export async function createFeatureFlag(flag: InsertFeatureFlag): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.insert(featureFlags).values(flag);
}

export async function updateFeatureFlag(id: number, data: Partial<InsertFeatureFlag>): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.update(featureFlags).set(data).where(eq(featureFlags.id, id));
}

export async function toggleFeatureFlag(id: number, enabled: boolean): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.update(featureFlags).set({ enabled }).where(eq(featureFlags.id, id));
}

// ===== Disbursement Schedules =====

export async function getAllDisbursementSchedules(): Promise<DisbursementSchedule[]> {
  const db = await getDb();
  if (!db) return [];
  return db.select().from(disbursementSchedules).orderBy(desc(disbursementSchedules.scheduledDate));
}

export async function getDisbursementScheduleById(id: number): Promise<DisbursementSchedule | undefined> {
  const db = await getDb();
  if (!db) return undefined;
  const result = await db.select().from(disbursementSchedules).where(eq(disbursementSchedules.id, id)).limit(1);
  return result[0];
}

export async function createDisbursementSchedule(schedule: InsertDisbursementSchedule): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.insert(disbursementSchedules).values(schedule);
}

export async function updateDisbursementSchedule(id: number, updates: Partial<InsertDisbursementSchedule>): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.update(disbursementSchedules).set(updates).where(eq(disbursementSchedules.id, id));
}

// ===== MCC Database (Reference) =====

export async function searchMccDatabase(query: string): Promise<MccDatabaseEntry[]> {
  const db = await getDb();
  if (!db) return [];
  // Simple search by description or code
  return db
    .select()
    .from(mccDatabase)
    .where(eq(mccDatabase.description, query))
    .limit(20);
}

export async function seedMccDatabase(entries: InsertMccDatabaseEntry[]): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.insert(mccDatabase).values(entries).onDuplicateKeyUpdate({ set: { id: sql`id` } });
}

export async function updateMccEntry(
  id: number,
  updates: { mccCode?: string; description?: string; category?: string | null }
): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.update(mccDatabase).set(updates).where(eq(mccDatabase.id, id));
}

export async function deleteMccEntry(id: number): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.delete(mccDatabase).where(eq(mccDatabase.id, id));
}

// ===== User Management =====

export async function getAllUsers(): Promise<User[]> {
  const db = await getDb();
  if (!db) return [];
  return db.select().from(users).orderBy(desc(users.createdAt));
}

export async function updateUserRole(userId: number, role: "admin" | "user"): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.update(users).set({ role }).where(eq(users.id, userId));
}

// ===== Admin Action Audit =====

export async function logAdminAction(action: InsertAdminActionAudit): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.insert(adminActionAudit).values(action);
}

export async function getAllAdminActions(filters?: {
  startDate?: Date;
  endDate?: Date;
  actionType?: string;
}): Promise<AdminActionAudit[]> {
  const db = await getDb();
  if (!db) return [];

  // Build where conditions
  const conditions = [];
  if (filters?.startDate) {
    conditions.push(gte(adminActionAudit.performedAt, filters.startDate));
  }
  if (filters?.endDate) {
    conditions.push(lte(adminActionAudit.performedAt, filters.endDate));
  }
  if (filters?.actionType) {
    conditions.push(eq(adminActionAudit.action, filters.actionType));
  }

  if (conditions.length > 0) {
    return db
      .select()
      .from(adminActionAudit)
      .where(and(...conditions))
      .orderBy(desc(adminActionAudit.performedAt))
      .limit(200);
  }

  return db.select().from(adminActionAudit).orderBy(desc(adminActionAudit.performedAt)).limit(200);
}

export async function getAdminActionsByUser(userId: number): Promise<AdminActionAudit[]> {
  const db = await getDb();
  if (!db) return [];
  return db.select().from(adminActionAudit).where(eq(adminActionAudit.targetUserId, userId)).orderBy(desc(adminActionAudit.performedAt));
}


// ===== Program Templates =====

export async function createProgramTemplate(template: InsertProgramTemplate): Promise<number> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  const result = await db.insert(programTemplates).values(template);
  return Number(result[0].insertId);
}

export async function getAllProgramTemplates(): Promise<ProgramTemplate[]> {
  const db = await getDb();
  if (!db) return [];
  return db.select().from(programTemplates).orderBy(desc(programTemplates.createdAt));
}

export async function getProgramTemplateById(id: number): Promise<ProgramTemplate | undefined> {
  const db = await getDb();
  if (!db) return undefined;
  const result = await db.select().from(programTemplates).where(eq(programTemplates.id, id)).limit(1);
  return result[0];
}

export async function deleteProgramTemplate(id: number): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.delete(programTemplates).where(eq(programTemplates.id, id));
}


// ===== Scheduled Reports =====

export async function createScheduledReport(report: InsertScheduledReport): Promise<number> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  const result = await db.insert(scheduledReports).values(report);
  return Number(result[0].insertId);
}

export async function getAllScheduledReports(): Promise<ScheduledReport[]> {
  const db = await getDb();
  if (!db) return [];
  return db.select().from(scheduledReports).orderBy(desc(scheduledReports.createdAt));
}

export async function getScheduledReportById(id: number): Promise<ScheduledReport | undefined> {
  const db = await getDb();
  if (!db) return undefined;
  const result = await db.select().from(scheduledReports).where(eq(scheduledReports.id, id)).limit(1);
  return result[0];
}

export async function updateScheduledReport(id: number, data: Partial<InsertScheduledReport>): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.update(scheduledReports).set(data).where(eq(scheduledReports.id, id));
}

export async function deleteScheduledReport(id: number): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.delete(scheduledReports).where(eq(scheduledReports.id, id));
}

export async function getActiveScheduledReports(): Promise<ScheduledReport[]> {
  const db = await getDb();
  if (!db) return [];
  return db.select().from(scheduledReports).where(eq(scheduledReports.isActive, 1));
}

// ===== Report History =====

export async function createReportHistory(history: InsertReportHistory): Promise<number> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  const result = await db.insert(reportHistory).values(history);
  return Number(result[0].insertId);
}

export async function getReportHistoryByReportId(reportId: number): Promise<ReportHistory[]> {
  const db = await getDb();
  if (!db) return [];
  return db.select().from(reportHistory).where(eq(reportHistory.reportId, reportId)).orderBy(desc(reportHistory.generatedAt)).limit(20);
}


export async function updateProgramTemplate(id: number, data: Partial<InsertProgramTemplate>): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.update(programTemplates).set(data).where(eq(programTemplates.id, id));
}


// ===== Notification Preferences =====

export async function getNotificationPreferencesByUserId(userId: number): Promise<NotificationPreference | undefined> {
  const db = await getDb();
  if (!db) return undefined;
  const result = await db.select().from(notificationPreferences).where(eq(notificationPreferences.userId, userId)).limit(1);
  return result[0];
}

export async function createNotificationPreference(data: InsertNotificationPreferences): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  const existing = await getNotificationPreferencesByUserId(data.userId);
  if (existing) {
    await db.update(notificationPreferences).set(data).where(eq(notificationPreferences.userId, data.userId));
  } else {
    await db.insert(notificationPreferences).values(data);
  }
}


export async function getUserById(userId: number): Promise<User | undefined> {
  const db = await getDb();
  if (!db) return undefined;
  const result = await db.select().from(users).where(eq(users.id, userId)).limit(1);
  return result[0];
}

export async function getAllMccEntries(): Promise<MccDatabaseEntry[]> {
  const db = await getDb();
  if (!db) return [];
  return db.select().from(mccDatabase);
}

export async function getMccEntryById(id: number): Promise<MccDatabaseEntry | undefined> {
  const db = await getDb();
  if (!db) return undefined;
  const result = await db.select().from(mccDatabase).where(eq(mccDatabase.id, id)).limit(1);
  return result[0];
}

export async function getAllAdminActionAudit(): Promise<AdminActionAudit[]> {
  const db = await getDb();
  if (!db) return [];
  return db.select().from(adminActionAudit).orderBy(desc(adminActionAudit.performedAt)).limit(1000);
}

export async function getAdminActionAuditById(id: number): Promise<AdminActionAudit | undefined> {
  const db = await getDb();
  if (!db) return undefined;
  const result = await db.select().from(adminActionAudit).where(eq(adminActionAudit.id, id)).limit(1);
  return result[0];
}


// ============================================================================
// Approval Workflows
// ============================================================================

export async function createApprovalRequest(request: {
  requestType: "ROLE_CHANGE" | "BATCH_PROGRAM_UPDATE" | "BATCH_FLAG_TOGGLE" | "PROGRAM_DELETE" | "USER_DELETE";
  requestedBy: number;
  targetId?: number;
  requestData: any;
  justification: string;
  requiredApprovals?: number;
}): Promise<number> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");

  const { approvalRequests } = await import("../drizzle/schema");
  
  const result = await db.insert(approvalRequests).values({
    requestType: request.requestType,
    requestedBy: request.requestedBy,
    targetId: request.targetId,
    requestData: JSON.stringify(request.requestData),
    justification: request.justification,
    requiredApprovals: request.requiredApprovals || 2,
    currentApprovals: 0,
    status: "pending",
  });

  return Number((result as any).insertId || result[0]?.insertId || 0);
}

export async function getAllPendingApprovalRequests(): Promise<any[]> {
  const db = await getDb();
  if (!db) return [];

  const { approvalRequests } = await import("../drizzle/schema");
  
  const requests = await db
    .select()
    .from(approvalRequests)
    .where(eq(approvalRequests.status, "pending"))
    .orderBy(desc(approvalRequests.createdAt));

  return requests.map(r => ({
    ...r,
    requestData: JSON.parse(r.requestData),
  }));
}

export async function getApprovalRequestById(id: number): Promise<any | undefined> {
  const db = await getDb();
  if (!db) return undefined;

  const { approvalRequests } = await import("../drizzle/schema");
  
  const result = await db
    .select()
    .from(approvalRequests)
    .where(eq(approvalRequests.id, id))
    .limit(1);

  if (result.length === 0) return undefined;

  return {
    ...result[0],
    requestData: JSON.parse(result[0].requestData),
  };
}

export async function createApproval(approval: {
  requestId: number;
  approvedBy: number;
  decision: "approved" | "rejected";
  comment?: string;
}): Promise<number> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");

  const { approvals, approvalRequests } = await import("../drizzle/schema");
  
  // Insert approval
  const result = await db.insert(approvals).values({
    requestId: approval.requestId,
    approvedBy: approval.approvedBy,
    decision: approval.decision,
    comment: approval.comment,
  });

  // Update request status
  const request = await getApprovalRequestById(approval.requestId);
  if (!request) throw new Error("Approval request not found");

  if (approval.decision === "rejected") {
    // If rejected, mark request as rejected
    await db
      .update(approvalRequests)
      .set({
        status: "rejected",
        completedAt: new Date(),
      })
      .where(eq(approvalRequests.id, approval.requestId));
  } else {
    // If approved, increment approval count
    const newCount = request.currentApprovals + 1;
    const updates: any = {
      currentApprovals: newCount,
    };

    // If reached required approvals, mark as approved
    if (newCount >= request.requiredApprovals) {
      updates.status = "approved";
      updates.completedAt = new Date();
    }

    await db
      .update(approvalRequests)
      .set(updates)
      .where(eq(approvalRequests.id, approval.requestId));
  }

  return Number((result as any).insertId || result[0]?.insertId || 0);
}

export async function getApprovalsForRequest(requestId: number): Promise<any[]> {
  const db = await getDb();
  if (!db) return [];

  const { approvals } = await import("../drizzle/schema");
  
  return db
    .select()
    .from(approvals)
    .where(eq(approvals.requestId, requestId))
    .orderBy(desc(approvals.createdAt));
}

// Saved Search Filters
export async function createSavedSearchFilter(data: InsertSavedSearchFilter): Promise<number> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  const result = await db.insert(savedSearchFilters).values(data);
  return Number((result as any).insertId || result[0]?.insertId || 0);
}

export async function getSavedSearchFiltersByUserId(userId: number): Promise<SavedSearchFilter[]> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  const result = await db.select().from(savedSearchFilters).where(eq(savedSearchFilters.userId, userId));
  return result;
}

export async function deleteSavedSearchFilter(filterId: number): Promise<void> {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  await db.delete(savedSearchFilters).where(eq(savedSearchFilters.id, filterId));
}
