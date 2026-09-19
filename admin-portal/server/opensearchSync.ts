import * as db from "./db";
import { indexDocument, deleteDocument, INDICES, initializeIndices } from "./opensearch";

/**
 * Sync all data from MySQL to OpenSearch
 * Call this on server startup or when OpenSearch is first set up
 */
export async function syncAllData(): Promise<void> {
  console.log("[OpenSearch Sync] Starting full data synchronization...");

  // Initialize indices first
  await initializeIndices();

  // Sync all entities
  await syncAllPrograms();
  await syncAllUsers();
  await syncAllMccCodes();
  await syncAllAuditLogs();

  console.log("[OpenSearch Sync] Full synchronization complete");
}

/**
 * Sync all benefit programs
 */
export async function syncAllPrograms(): Promise<void> {
  const programs = await db.getAllBenefitPrograms();
  
  for (const program of programs) {
    const mccRules = await db.getMccRulesByProgramId(program.id);
    
    await indexDocument(INDICES.PROGRAMS, program.id, {
      id: program.id,
      name: program.name,
      description: program.description,
      accountType: program.accountType,
      status: program.status,
      createdAt: program.createdAt,
      updatedAt: program.updatedAt,
      mccRules: mccRules.map((rule) => ({
        mccCode: rule.mccCode,
        mccDescription: rule.mccDescription,
      })),
    });
  }
  
  console.log(`[OpenSearch Sync] Synced ${programs.length} programs`);
}

/**
 * Sync a single program
 */
export async function syncProgram(programId: number): Promise<void> {
  const program = await db.getBenefitProgramById(programId);
  if (!program) {
    await deleteDocument(INDICES.PROGRAMS, programId);
    return;
  }

  const mccRules = await db.getMccRulesByProgramId(programId);
  
  await indexDocument(INDICES.PROGRAMS, program.id, {
    id: program.id,
    name: program.name,
    description: program.description,
    accountType: program.accountType,
    status: program.status,
    createdAt: program.createdAt,
    updatedAt: program.updatedAt,
    mccRules: mccRules.map((rule) => ({
      mccCode: rule.mccCode,
      mccDescription: rule.mccDescription,
    })),
  });
}

/**
 * Sync all users
 */
export async function syncAllUsers(): Promise<void> {
  const users = await db.getAllUsers();
  
  for (const user of users) {
    await indexDocument(INDICES.USERS, user.id, {
      id: user.id,
      name: user.name,
      email: user.email,
      role: user.role,
      loginMethod: user.loginMethod,
      createdAt: user.createdAt,
      lastSignedIn: user.lastSignedIn,
    });
  }
  
  console.log(`[OpenSearch Sync] Synced ${users.length} users`);
}

/**
 * Sync a single user
 */
export async function syncUser(userId: number): Promise<void> {
  const user = await db.getUserById(userId);
  if (!user) {
    await deleteDocument(INDICES.USERS, userId);
    return;
  }

  await indexDocument(INDICES.USERS, user.id, {
    id: user.id,
    name: user.name,
    email: user.email,
    role: user.role,
    loginMethod: user.loginMethod,
    createdAt: user.createdAt,
    lastSignedIn: user.lastSignedIn,
  });
}

/**
 * Sync all MCC codes
 */
export async function syncAllMccCodes(): Promise<void> {
  const mccCodes = await db.getAllMccEntries();
  
  for (const mcc of mccCodes) {
    await indexDocument(INDICES.MCC_CODES, mcc.id, {
      id: mcc.id,
      mccCode: mcc.mccCode,
      description: mcc.description,
      category: mcc.category,

    });
  }
  
  console.log(`[OpenSearch Sync] Synced ${mccCodes.length} MCC codes`);
}

/**
 * Sync a single MCC code
 */
export async function syncMccCode(mccId: number): Promise<void> {
  const mcc = await db.getMccEntryById(mccId);
  if (!mcc) {
    await deleteDocument(INDICES.MCC_CODES, mccId);
    return;
  }

  await indexDocument(INDICES.MCC_CODES, mcc.id, {
    id: mcc.id,
    mccCode: mcc.mccCode,
    description: mcc.description,
    category: mcc.category,
  });
}

/**
 * Sync all audit logs
 */
export async function syncAllAuditLogs(): Promise<void> {
  const logs = await db.getAllAdminActionAudit();
  
  for (const log of logs) {
    const performer = await db.getUserById(log.performedBy);
    
    await indexDocument(INDICES.AUDIT_LOGS, log.id, {
      id: log.id,
      action: log.action,
      targetUserId: log.targetUserId,
      performedBy: log.performedBy,
      performedByName: performer?.name || "Unknown",
      justification: log.justification,
      timestamp: log.performedAt,
      details: log.details,
    });
  }
  
  console.log(`[OpenSearch Sync] Synced ${logs.length} audit logs`);
}

/**
 * Sync a single audit log
 */
export async function syncAuditLog(logId: number): Promise<void> {
  const log = await db.getAdminActionAuditById(logId);
  if (!log) {
    await deleteDocument(INDICES.AUDIT_LOGS, logId);
    return;
  }

  const performer = await db.getUserById(log.performedBy);
  
  await indexDocument(INDICES.AUDIT_LOGS, log.id, {
    id: log.id,
    action: log.action,
    targetUserId: log.targetUserId,
    performedBy: log.performedBy,
    performedByName: performer?.name || "Unknown",
    justification: log.justification,
    timestamp: log.performedAt,
    details: log.details,
  });
}
