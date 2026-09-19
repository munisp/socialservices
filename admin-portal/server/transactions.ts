import { eq, desc, and, gte, lte, sql, between } from "drizzle-orm";
import { getDb } from "./db";
import {
  transactions,
  fraudAlerts,
  type Transaction,
  type InsertTransaction,
  type FraudAlert,
  type InsertFraudAlert,
} from "../drizzle/schema";

// Transaction operations
export async function getAllTransactions(limit = 100) {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(transactions)
    .orderBy(desc(transactions.transactionDate))
    .limit(limit);
}

export async function getTransactionById(id: number) {
  const db = await getDb();
  if (!db) return undefined;
  
  const result = await db
    .select()
    .from(transactions)
    .where(eq(transactions.id, id))
    .limit(1);
  
  return result[0];
}

export async function getBeneficiaryTransactions(beneficiaryId: number, limit = 50) {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(transactions)
    .where(eq(transactions.beneficiaryId, beneficiaryId))
    .orderBy(desc(transactions.transactionDate))
    .limit(limit);
}

export async function getProgramTransactions(programId: number, limit = 100) {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(transactions)
    .where(eq(transactions.programId, programId))
    .orderBy(desc(transactions.transactionDate))
    .limit(limit);
}

export async function getTransactionsByDateRange(startDate: Date, endDate: Date) {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(transactions)
    .where(between(transactions.transactionDate, startDate, endDate))
    .orderBy(desc(transactions.transactionDate));
}

export async function getNonCompliantTransactions(limit = 50) {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(transactions)
    .where(eq(transactions.complianceStatus, "non_compliant"))
    .orderBy(desc(transactions.transactionDate))
    .limit(limit);
}

export async function createTransaction(data: InsertTransaction) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  const [result] = await db.insert(transactions).values(data);
  return result.insertId;
}

export async function updateTransactionStatus(
  id: number,
  status: "pending" | "approved" | "declined" | "reversed",
  reason?: string
) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  const updateData: any = { status };
  if (reason) {
    updateData.declineReason = reason;
  }
  
  await db
    .update(transactions)
    .set(updateData)
    .where(eq(transactions.id, id));
}

export async function updateComplianceStatus(
  id: number,
  complianceStatus: "compliant" | "non_compliant" | "under_review"
) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  await db
    .update(transactions)
    .set({ complianceStatus })
    .where(eq(transactions.id, id));
}

// Fraud Alert operations
export async function getAllFraudAlerts(limit = 50) {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(fraudAlerts)
    .orderBy(desc(fraudAlerts.createdAt))
    .limit(limit);
}

export async function getOpenFraudAlerts() {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(fraudAlerts)
    .where(eq(fraudAlerts.status, "open"))
    .orderBy(desc(fraudAlerts.createdAt));
}

export async function getBeneficiaryFraudAlerts(beneficiaryId: number) {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(fraudAlerts)
    .where(eq(fraudAlerts.beneficiaryId, beneficiaryId))
    .orderBy(desc(fraudAlerts.createdAt));
}

export async function createFraudAlert(data: InsertFraudAlert) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  const [result] = await db.insert(fraudAlerts).values(data);
  return result.insertId;
}

export async function updateFraudAlertStatus(
  id: number,
  status: "open" | "investigating" | "resolved" | "false_positive",
  assignedTo?: number,
  resolvedBy?: number,
  resolution?: string
) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  const updateData: any = { status };
  
  if (assignedTo !== undefined) {
    updateData.assignedTo = assignedTo;
  }
  
  if (status === "resolved" || status === "false_positive") {
    updateData.resolvedBy = resolvedBy;
    updateData.resolvedAt = new Date();
    if (resolution) {
      updateData.resolution = resolution;
    }
  }
  
  await db
    .update(fraudAlerts)
    .set(updateData)
    .where(eq(fraudAlerts.id, id));
}

// Analytics and Statistics
export async function getTransactionStats(programId?: number) {
  const db = await getDb();
  if (!db) return {
    total: 0,
    approved: 0,
    declined: 0,
    pending: 0,
    totalAmount: 0,
    avgAmount: 0,
  };
  
  const conditions = programId ? eq(transactions.programId, programId) : undefined;
  
  const [stats] = await db
    .select({
      total: sql<number>`COUNT(*)`,
      approved: sql<number>`SUM(CASE WHEN status = 'approved' THEN 1 ELSE 0 END)`,
      declined: sql<number>`SUM(CASE WHEN status = 'declined' THEN 1 ELSE 0 END)`,
      pending: sql<number>`SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END)`,
      totalAmount: sql<number>`SUM(amount)`,
      avgAmount: sql<number>`AVG(amount)`,
    })
    .from(transactions)
    .where(conditions);
  
  return stats;
}

export async function getMccUsageStats(programId?: number, limit = 10) {
  const db = await getDb();
  if (!db) return [];
  
  const conditions = programId ? eq(transactions.programId, programId) : undefined;
  
  return await db
    .select({
      mccCode: transactions.mccCode,
      mccDescription: transactions.mccDescription,
      transactionCount: sql<number>`COUNT(*)`,
      totalAmount: sql<number>`SUM(amount)`,
      avgAmount: sql<number>`AVG(amount)`,
    })
    .from(transactions)
    .where(conditions)
    .groupBy(transactions.mccCode, transactions.mccDescription)
    .orderBy(desc(sql`COUNT(*)`))
    .limit(limit);
}

export async function getDailyTransactionVolume(days = 30) {
  const db = await getDb();
  if (!db) return [];
  
  const startDate = new Date();
  startDate.setDate(startDate.getDate() - days);
  
  return await db
    .select({
      date: sql<string>`DATE(transaction_date)`,
      transactionCount: sql<number>`COUNT(*)`,
      totalAmount: sql<number>`SUM(amount)`,
      approvedCount: sql<number>`SUM(CASE WHEN status = 'approved' THEN 1 ELSE 0 END)`,
      declinedCount: sql<number>`SUM(CASE WHEN status = 'declined' THEN 1 ELSE 0 END)`,
    })
    .from(transactions)
    .where(gte(transactions.transactionDate, startDate))
    .groupBy(sql`DATE(transaction_date)`)
    .orderBy(sql`DATE(transaction_date)`);
}

export async function getFraudAlertStats() {
  const db = await getDb();
  if (!db) return {
    total: 0,
    open: 0,
    investigating: 0,
    resolved: 0,
    falsePositive: 0,
  };
  
  const [stats] = await db
    .select({
      total: sql<number>`COUNT(*)`,
      open: sql<number>`SUM(CASE WHEN status = 'open' THEN 1 ELSE 0 END)`,
      investigating: sql<number>`SUM(CASE WHEN status = 'investigating' THEN 1 ELSE 0 END)`,
      resolved: sql<number>`SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END)`,
      falsePositive: sql<number>`SUM(CASE WHEN status = 'false_positive' THEN 1 ELSE 0 END)`,
    })
    .from(fraudAlerts);
  
  return stats;
}

export async function getAlertsBySeverity() {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select({
      severity: fraudAlerts.severity,
      count: sql<number>`COUNT(*)`,
      openCount: sql<number>`SUM(CASE WHEN status = 'open' THEN 1 ELSE 0 END)`,
    })
    .from(fraudAlerts)
    .groupBy(fraudAlerts.severity)
    .orderBy(sql`FIELD(severity, 'critical', 'high', 'medium', 'low')`);
}

/**
 * Generate simulated transactions for testing
 */
export async function generateSimulatedTransactions(
  beneficiaryId: number,
  count: number,
  pattern: "compliant" | "mcc_violation" | "fraud_pattern" | "mixed",
  amountRange?: { min: number; max: number }
) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");

  const generatedTransactions = [];
  const now = new Date();
  
  // Sample MCC codes
  const compliantMccs = ["5411", "5812", "5912", "5499"]; // Grocery, restaurant, pharmacy, food
  const violationMccs = ["5813", "5921", "7995", "5993"]; // Bars, liquor stores, gambling, cigars
  const fraudMccs = ["5999", "5732", "4829"]; // Misc, electronics, wire transfer
  
  const merchants = {
    compliant: ["Local Grocery Store", "Fresh Market", "Food Mart", "Community Shop"],
    violation: ["Wine & Spirits", "Liquor Store", "Casino", "Betting Shop"],
    fraud: ["Unknown Merchant", "Overseas Transfer", "Suspicious Vendor"],
  };
  
  for (let i = 0; i < count; i++) {
    let mccCode: string;
    let merchantName: string;
    let complianceStatus: "compliant" | "non_compliant" | "under_review";
    let isSuspicious: boolean = false;
    
    // Determine transaction type based on pattern
    if (pattern === "compliant") {
      mccCode = compliantMccs[Math.floor(Math.random() * compliantMccs.length)];
      merchantName = merchants.compliant[Math.floor(Math.random() * merchants.compliant.length)];
      complianceStatus = "compliant";
    } else if (pattern === "mcc_violation") {
      mccCode = violationMccs[Math.floor(Math.random() * violationMccs.length)];
      merchantName = merchants.violation[Math.floor(Math.random() * merchants.violation.length)];
      complianceStatus = "non_compliant";
    } else if (pattern === "fraud_pattern") {
      mccCode = fraudMccs[Math.floor(Math.random() * fraudMccs.length)];
      merchantName = merchants.fraud[Math.floor(Math.random() * merchants.fraud.length)];
      complianceStatus = "under_review";
      isSuspicious = true;
    } else { // mixed
      const rand = Math.random();
      if (rand < 0.7) {
        mccCode = compliantMccs[Math.floor(Math.random() * compliantMccs.length)];
        merchantName = merchants.compliant[Math.floor(Math.random() * merchants.compliant.length)];
        complianceStatus = "compliant";
      } else if (rand < 0.9) {
        mccCode = violationMccs[Math.floor(Math.random() * violationMccs.length)];
        merchantName = merchants.violation[Math.floor(Math.random() * merchants.violation.length)];
        complianceStatus = "non_compliant";
      } else {
        mccCode = fraudMccs[Math.floor(Math.random() * fraudMccs.length)];
        merchantName = merchants.fraud[Math.floor(Math.random() * merchants.fraud.length)];
        complianceStatus = "under_review";
        isSuspicious = true;
      }
    }
    
    // Generate amount
    const minAmount = amountRange?.min || 500;
    const maxAmount = amountRange?.max || 5000;
    const amount = Math.floor(Math.random() * (maxAmount - minAmount) + minAmount);
    
    // Generate timestamp (within last 30 days)
    const daysAgo = Math.floor(Math.random() * 30);
    const transactionDate = new Date(now.getTime() - daysAgo * 24 * 60 * 60 * 1000);
    
    // Generate transaction ID
    const transactionId = `SIM${Date.now()}${i}${Math.floor(Math.random() * 1000)}`;
    
    // Get beneficiary's first active program enrollment
    // Default to program ID 1 if no enrollment found
    let programId = 1;
    try {
      const enrollments: any = await db.execute(
        sql.raw(`SELECT program_id FROM program_enrollments WHERE beneficiary_id = ${beneficiaryId} AND status = 'active' LIMIT 1`)
      );
      if (enrollments && enrollments.length > 0 && enrollments[0].program_id) {
        programId = enrollments[0].program_id;
      }
    } catch (e) {
      // Use default programId if query fails
    }
    
    // Create transaction
    const transactionData: InsertTransaction = {
      transactionId,
      beneficiaryId,
      programId,
      amount,
      currency: "NGN",
      merchantName,
      mccCode,
      mccDescription: `Simulated ${merchantName}`,
      transactionDate,
      transactionType: "purchase",
      status: "approved",
      complianceStatus,
    };
    
    const txId = await createTransaction(transactionData);
    const transaction = await getTransactionById(txId);
    
    if (transaction) {
      generatedTransactions.push(transaction);
    }
    
    // Create fraud alert if suspicious
    if (isSuspicious && transaction) {
      const alertType = Math.random() > 0.5 ? "unusual_spending" : "suspicious_merchant";
      const severity = Math.random() > 0.5 ? "high" : "critical";
      const riskScore = Math.floor(Math.random() * 30) + 70; // 70-100
      
      await createFraudAlert({
        beneficiaryId,
        transactionId: transaction.id,
        alertType,
        severity,
        status: "open",
        description: `Simulated ${alertType} alert for testing (Risk Score: ${riskScore})`,
      });
    }
  }
  
  return generatedTransactions;
}
