import { eq, desc, asc, and, or, like, sql, gt, lt } from "drizzle-orm";
import { getDb } from "./db";
import {
  beneficiaries,
  programEnrollments,
  kycDocuments,
  benefitCards,
  type Beneficiary,
  type InsertBeneficiary,
  type ProgramEnrollment,
  type InsertProgramEnrollment,
  type KycDocument,
  type InsertKycDocument,
  type BenefitCard,
  type InsertBenefitCard,
} from "../drizzle/schema";

// Pagination types
export interface PaginationParams {
  page?: number;
  pageSize?: number;
  status?: "all" | "pending" | "approved" | "rejected" | "suspended";
  sortBy?: "createdAt" | "firstName" | "lastName" | "enrollmentStatus";
  sortOrder?: "asc" | "desc";
  cursor?: number; // For cursor-based pagination (beneficiary ID)
}

export interface PaginatedResult<T> {
  results: T[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
  hasNextPage: boolean;
  hasPreviousPage: boolean;
  nextCursor?: number;
}

// Beneficiary CRUD operations with DB-backed pagination
export async function getAllBeneficiaries() {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(beneficiaries)
    .orderBy(desc(beneficiaries.createdAt));
}

/**
 * Get beneficiaries with proper DB-backed pagination
 * Uses LIMIT/OFFSET for offset-based pagination
 * Supports filtering, sorting at the database level
 */
export async function getBeneficiariesPaginated(params: PaginationParams = {}): Promise<PaginatedResult<Beneficiary>> {
  const db = await getDb();
  if (!db) {
    return {
      results: [],
      total: 0,
      page: 1,
      pageSize: 20,
      totalPages: 0,
      hasNextPage: false,
      hasPreviousPage: false,
    };
  }
  
  const page = params.page || 1;
  const pageSize = Math.min(params.pageSize || 20, 100); // Max 100 per page
  const offset = (page - 1) * pageSize;
  const sortOrder = params.sortOrder || "desc";
  const sortBy = params.sortBy || "createdAt";
  
  // Build where conditions
  const conditions = [];
  if (params.status && params.status !== "all") {
    conditions.push(eq(beneficiaries.enrollmentStatus, params.status));
  }
  
  // Get total count with filters
  const countQuery = db
    .select({ count: sql<number>`COUNT(*)` })
    .from(beneficiaries);
  
  if (conditions.length > 0) {
    countQuery.where(and(...conditions));
  }
  
  const [countResult] = await countQuery;
  const total = countResult?.count || 0;
  
  // Build the main query with sorting
  let orderByColumn;
  switch (sortBy) {
    case "firstName":
      orderByColumn = beneficiaries.firstName;
      break;
    case "lastName":
      orderByColumn = beneficiaries.lastName;
      break;
    case "enrollmentStatus":
      orderByColumn = beneficiaries.enrollmentStatus;
      break;
    case "createdAt":
    default:
      orderByColumn = beneficiaries.createdAt;
  }
  
  const query = db
    .select()
    .from(beneficiaries)
    .orderBy(sortOrder === "asc" ? asc(orderByColumn) : desc(orderByColumn))
    .limit(pageSize)
    .offset(offset);
  
  if (conditions.length > 0) {
    query.where(and(...conditions));
  }
  
  const results = await query;
  const totalPages = Math.ceil(total / pageSize);
  
  return {
    results,
    total,
    page,
    pageSize,
    totalPages,
    hasNextPage: page < totalPages,
    hasPreviousPage: page > 1,
    nextCursor: results.length > 0 ? results[results.length - 1].id : undefined,
  };
}

/**
 * Cursor-based pagination for large datasets
 * More efficient than offset-based for deep pagination
 */
export async function getBeneficiariesByCursor(
  cursor: number | undefined,
  limit: number = 20,
  status?: "all" | "pending" | "approved" | "rejected" | "suspended"
): Promise<{ results: Beneficiary[]; nextCursor?: number; hasMore: boolean }> {
  const db = await getDb();
  if (!db) {
    return { results: [], hasMore: false };
  }
  
  const pageSize = Math.min(limit, 100);
  const conditions = [];
  
  if (cursor) {
    conditions.push(gt(beneficiaries.id, cursor));
  }
  
  if (status && status !== "all") {
    conditions.push(eq(beneficiaries.enrollmentStatus, status));
  }
  
  const query = db
    .select()
    .from(beneficiaries)
    .orderBy(asc(beneficiaries.id))
    .limit(pageSize + 1); // Fetch one extra to check if there's more
  
  if (conditions.length > 0) {
    query.where(and(...conditions));
  }
  
  const results = await query;
  const hasMore = results.length > pageSize;
  
  if (hasMore) {
    results.pop(); // Remove the extra item
  }
  
  return {
    results,
    nextCursor: results.length > 0 ? results[results.length - 1].id : undefined,
    hasMore,
  };
}

/**
 * Search beneficiaries with DB-backed pagination
 */
export async function searchBeneficiariesPaginated(
  query: string,
  params: PaginationParams = {}
): Promise<PaginatedResult<Beneficiary>> {
  const db = await getDb();
  if (!db) {
    return {
      results: [],
      total: 0,
      page: 1,
      pageSize: 20,
      totalPages: 0,
      hasNextPage: false,
      hasPreviousPage: false,
    };
  }
  
  const page = params.page || 1;
  const pageSize = Math.min(params.pageSize || 20, 100);
  const offset = (page - 1) * pageSize;
  const searchPattern = `%${query}%`;
  
  const searchCondition = or(
    like(beneficiaries.firstName, searchPattern),
    like(beneficiaries.lastName, searchPattern),
    like(beneficiaries.nationalId, searchPattern),
    like(beneficiaries.email, searchPattern),
    like(beneficiaries.phoneNumber, searchPattern)
  );
  
  // Get total count
  const [countResult] = await db
    .select({ count: sql<number>`COUNT(*)` })
    .from(beneficiaries)
    .where(searchCondition);
  
  const total = countResult?.count || 0;
  
  // Get paginated results
  const results = await db
    .select()
    .from(beneficiaries)
    .where(searchCondition)
    .orderBy(desc(beneficiaries.createdAt))
    .limit(pageSize)
    .offset(offset);
  
  const totalPages = Math.ceil(total / pageSize);
  
  return {
    results,
    total,
    page,
    pageSize,
    totalPages,
    hasNextPage: page < totalPages,
    hasPreviousPage: page > 1,
  };
}

export async function getBeneficiaryById(id: number) {
  const db = await getDb();
  if (!db) return undefined;
  
  const result = await db
    .select()
    .from(beneficiaries)
    .where(eq(beneficiaries.id, id))
    .limit(1);
  
  return result[0];
}

export async function searchBeneficiaries(query: string) {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(beneficiaries)
    .where(
      or(
        like(beneficiaries.firstName, `%${query}%`),
        like(beneficiaries.lastName, `%${query}%`),
        like(beneficiaries.nationalId, `%${query}%`),
        like(beneficiaries.email, `%${query}%`),
        like(beneficiaries.phoneNumber, `%${query}%`)
      )
    )
    .orderBy(desc(beneficiaries.createdAt));
}

export async function createBeneficiary(data: InsertBeneficiary) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  const [result] = await db.insert(beneficiaries).values(data);
  return result.insertId;
}

export async function updateBeneficiary(id: number, data: Partial<InsertBeneficiary>) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  await db
    .update(beneficiaries)
    .set(data)
    .where(eq(beneficiaries.id, id));
}

export async function deleteBeneficiary(id: number) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  await db.delete(beneficiaries).where(eq(beneficiaries.id, id));
}

export async function approveBeneficiary(id: number, approvedBy: number) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  await db
    .update(beneficiaries)
    .set({
      enrollmentStatus: "approved",
      approvedAt: new Date(),
      approvedBy,
    })
    .where(eq(beneficiaries.id, id));
}

export async function rejectBeneficiary(id: number) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  await db
    .update(beneficiaries)
    .set({ enrollmentStatus: "rejected" })
    .where(eq(beneficiaries.id, id));
}

export async function suspendBeneficiary(id: number) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  await db.update(beneficiaries).set({ enrollmentStatus: "suspended" }).where(eq(beneficiaries.id, id));
}

// Program Enrollment operations
export async function getBeneficiaryEnrollments(beneficiaryId: number) {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(programEnrollments)
    .where(eq(programEnrollments.beneficiaryId, beneficiaryId))
    .orderBy(desc(programEnrollments.enrollmentDate));
}

export async function createEnrollment(data: InsertProgramEnrollment) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  const [result] = await db.insert(programEnrollments).values(data);
  return result.insertId;
}

export async function updateEnrollment(id: number, data: Partial<InsertProgramEnrollment>) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  await db
    .update(programEnrollments)
    .set(data)
    .where(eq(programEnrollments.id, id));
}

export async function getEnrollmentsByProgram(programId: number) {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(programEnrollments)
    .where(eq(programEnrollments.programId, programId))
    .orderBy(desc(programEnrollments.enrollmentDate));
}

// KYC Document operations
export async function getBeneficiaryDocuments(beneficiaryId: number) {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(kycDocuments)
    .where(eq(kycDocuments.beneficiaryId, beneficiaryId))
    .orderBy(desc(kycDocuments.uploadedAt));
}

export async function createKycDocument(data: InsertKycDocument) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  const [result] = await db.insert(kycDocuments).values(data);
  return result.insertId;
}

export async function verifyDocument(id: number, verifiedBy: number, status: "verified" | "rejected", reason?: string) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  await db
    .update(kycDocuments)
    .set({
      verificationStatus: status,
      verifiedBy,
      verifiedAt: new Date(),
      rejectionReason: reason || null,
    })
    .where(eq(kycDocuments.id, id));
}

export async function deleteDocument(id: number) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  await db.delete(kycDocuments).where(eq(kycDocuments.id, id));
}

// Benefit Card operations
export async function getBeneficiaryCards(beneficiaryId: number) {
  const db = await getDb();
  if (!db) return [];
  
  return await db
    .select()
    .from(benefitCards)
    .where(eq(benefitCards.beneficiaryId, beneficiaryId))
    .orderBy(desc(benefitCards.issuedAt));
}

export async function createBenefitCard(data: InsertBenefitCard) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  const [result] = await db.insert(benefitCards).values(data);
  return result.insertId;
}

export async function updateCardStatus(
  id: number,
  status: "pending" | "active" | "blocked" | "expired" | "lost" | "stolen",
  reason?: string
) {
  const db = await getDb();
  if (!db) throw new Error("Database not available");
  
  const updateData: any = { status };
  
  if (status === "active") {
    updateData.activatedAt = new Date();
  } else if (status === "blocked" || status === "lost" || status === "stolen") {
    updateData.blockedAt = new Date();
    updateData.blockReason = reason || null;
  }
  
  await db
    .update(benefitCards)
    .set(updateData)
    .where(eq(benefitCards.id, id));
}

export async function getCardByNumber(cardNumber: string) {
  const db = await getDb();
  if (!db) return undefined;
  
  const result = await db
    .select()
    .from(benefitCards)
    .where(eq(benefitCards.cardNumber, cardNumber))
    .limit(1);
  
  return result[0];
}

// Statistics
export async function getBeneficiaryStats() {
  const db = await getDb();
  if (!db) return {
    total: 0,
    pending: 0,
    approved: 0,
    rejected: 0,
    suspended: 0,
  };
  
  const [stats] = await db
    .select({
      total: sql<number>`COUNT(*)`,
      pending: sql<number>`SUM(CASE WHEN enrollment_status = 'pending' THEN 1 ELSE 0 END)`,
      approved: sql<number>`SUM(CASE WHEN enrollment_status = 'approved' THEN 1 ELSE 0 END)`,
      rejected: sql<number>`SUM(CASE WHEN enrollment_status = 'rejected' THEN 1 ELSE 0 END)`,
      suspended: sql<number>`SUM(CASE WHEN enrollment_status = 'suspended' THEN 1 ELSE 0 END)`,
    })
    .from(beneficiaries);
  
  return stats;
}
