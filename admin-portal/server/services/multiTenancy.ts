import { getDb } from "../db";
import { tenants, tenantUsers, tenantUsage } from "../../drizzle/schema";
import { desc, eq, and } from "drizzle-orm";

/**
 * Multi-tenancy Support
 * Tenant isolation across all middleware components
 */

export interface Tenant {
  id: number;
  tenantCode: string;
  tenantName: string;
  status: "active" | "suspended" | "inactive";
  configuration?: any;
  kafkaTopicPrefix: string;
  redisNamespace: string;
  permifyTenantId?: string;
  createdAt: Date;
  updatedAt: Date;
}

export interface TenantUser {
  id: number;
  tenantId: number;
  userId: number;
  role: "owner" | "admin" | "member";
  joinedAt: Date;
}

/**
 * Create tenant
 */
export async function createTenant(
  tenantCode: string,
  tenantName: string,
  ownerId: number,
  configuration?: any
): Promise<number | null> {
  const db = await getDb();
  if (!db) return null;

  try {
    // Generate unique prefixes
    const kafkaTopicPrefix = `${tenantCode}_`;
    const redisNamespace = `${tenantCode}:`;
    const permifyTenantId = `tenant_${tenantCode}`;

    // Create tenant
    const result = await db.insert(tenants).values({
      tenantCode,
      tenantName,
      configuration: configuration ? JSON.stringify(configuration) : null,
      kafkaTopicPrefix,
      redisNamespace,
      permifyTenantId,
    });

    const tenantId = result[0].insertId;

    // Add owner as tenant user
    await db.insert(tenantUsers).values({
      tenantId,
      userId: ownerId,
      role: "owner",
    });

    console.log(`[MultiTenancy] Tenant created: ${tenantCode} (${tenantId})`);
    return tenantId;
  } catch (error) {
    console.error("[MultiTenancy] Failed to create tenant:", error);
    return null;
  }
}

/**
 * Get tenant by ID
 */
export async function getTenantById(tenantId: number): Promise<Tenant | null> {
  const db = await getDb();
  if (!db) return null;

  try {
    const results = await db.select().from(tenants).where(eq(tenants.id, tenantId)).limit(1);

    if (results.length === 0) {
      return null;
    }

    const r = results[0];
    return {
      id: r.id,
      tenantCode: r.tenantCode,
      tenantName: r.tenantName,
      status: r.status,
      configuration: r.configuration ? JSON.parse(r.configuration) : undefined,
      kafkaTopicPrefix: r.kafkaTopicPrefix,
      redisNamespace: r.redisNamespace,
      permifyTenantId: r.permifyTenantId || undefined,
      createdAt: r.createdAt,
      updatedAt: r.updatedAt,
    };
  } catch (error) {
    console.error("[MultiTenancy] Failed to get tenant:", error);
    return null;
  }
}

/**
 * Get tenant by code
 */
export async function getTenantByCode(tenantCode: string): Promise<Tenant | null> {
  const db = await getDb();
  if (!db) return null;

  try {
    const results = await db
      .select()
      .from(tenants)
      .where(eq(tenants.tenantCode, tenantCode))
      .limit(1);

    if (results.length === 0) {
      return null;
    }

    const r = results[0];
    return {
      id: r.id,
      tenantCode: r.tenantCode,
      tenantName: r.tenantName,
      status: r.status,
      configuration: r.configuration ? JSON.parse(r.configuration) : undefined,
      kafkaTopicPrefix: r.kafkaTopicPrefix,
      redisNamespace: r.redisNamespace,
      permifyTenantId: r.permifyTenantId || undefined,
      createdAt: r.createdAt,
      updatedAt: r.updatedAt,
    };
  } catch (error) {
    console.error("[MultiTenancy] Failed to get tenant:", error);
    return null;
  }
}

/**
 * Get all tenants
 */
export async function getAllTenants(): Promise<Tenant[]> {
  const db = await getDb();
  if (!db) return [];

  try {
    const results = await db.select().from(tenants).orderBy(desc(tenants.createdAt)).limit(100);

    return results.map((r) => ({
      id: r.id,
      tenantCode: r.tenantCode,
      tenantName: r.tenantName,
      status: r.status,
      configuration: r.configuration ? JSON.parse(r.configuration) : undefined,
      kafkaTopicPrefix: r.kafkaTopicPrefix,
      redisNamespace: r.redisNamespace,
      permifyTenantId: r.permifyTenantId || undefined,
      createdAt: r.createdAt,
      updatedAt: r.updatedAt,
    }));
  } catch (error) {
    console.error("[MultiTenancy] Failed to get tenants:", error);
    return [];
  }
}

/**
 * Update tenant status
 */
export async function updateTenantStatus(
  tenantId: number,
  status: "active" | "suspended" | "inactive"
): Promise<void> {
  const db = await getDb();
  if (!db) return;

  try {
    await db.update(tenants).set({ status }).where(eq(tenants.id, tenantId));

    console.log(`[MultiTenancy] Tenant ${tenantId} status updated to ${status}`);
  } catch (error) {
    console.error("[MultiTenancy] Failed to update tenant status:", error);
  }
}

/**
 * Add user to tenant
 */
export async function addUserToTenant(
  tenantId: number,
  userId: number,
  role: "owner" | "admin" | "member" = "member"
): Promise<void> {
  const db = await getDb();
  if (!db) return;

  try {
    await db.insert(tenantUsers).values({
      tenantId,
      userId,
      role,
    });

    console.log(`[MultiTenancy] User ${userId} added to tenant ${tenantId} as ${role}`);
  } catch (error) {
    console.error("[MultiTenancy] Failed to add user to tenant:", error);
  }
}

/**
 * Remove user from tenant
 */
export async function removeUserFromTenant(tenantId: number, userId: number): Promise<void> {
  const db = await getDb();
  if (!db) return;

  try {
    await db
      .delete(tenantUsers)
      .where(and(eq(tenantUsers.tenantId, tenantId), eq(tenantUsers.userId, userId)));

    console.log(`[MultiTenancy] User ${userId} removed from tenant ${tenantId}`);
  } catch (error) {
    console.error("[MultiTenancy] Failed to remove user from tenant:", error);
  }
}

/**
 * Get user's tenants
 */
export async function getUserTenants(userId: number): Promise<Tenant[]> {
  const db = await getDb();
  if (!db) return [];

  try {
    const userTenants = await db
      .select()
      .from(tenantUsers)
      .where(eq(tenantUsers.userId, userId));

    const tenantIds = userTenants.map((ut) => ut.tenantId);

    if (tenantIds.length === 0) {
      return [];
    }

    const tenantResults = await db.select().from(tenants);

    const filteredTenants = tenantResults.filter((t) => tenantIds.includes(t.id));

    return filteredTenants.map((r) => ({
      id: r.id,
      tenantCode: r.tenantCode,
      tenantName: r.tenantName,
      status: r.status,
      configuration: r.configuration ? JSON.parse(r.configuration) : undefined,
      kafkaTopicPrefix: r.kafkaTopicPrefix,
      redisNamespace: r.redisNamespace,
      permifyTenantId: r.permifyTenantId || undefined,
      createdAt: r.createdAt,
      updatedAt: r.updatedAt,
    }));
  } catch (error) {
    console.error("[MultiTenancy] Failed to get user tenants:", error);
    return [];
  }
}

/**
 * Record tenant usage
 */
export async function recordTenantUsage(
  tenantId: number,
  metricType: string,
  metricValue: number,
  period: string
): Promise<void> {
  const db = await getDb();
  if (!db) return;

  try {
    await db.insert(tenantUsage).values({
      tenantId,
      metricType,
      metricValue,
      period,
    });
  } catch (error) {
    console.error("[MultiTenancy] Failed to record tenant usage:", error);
  }
}

/**
 * Get tenant usage
 */
export async function getTenantUsage(
  tenantId: number,
  metricType: string,
  limit: number = 30
): Promise<any[]> {
  const db = await getDb();
  if (!db) return [];

  try {
    const results = await db
      .select()
      .from(tenantUsage)
      .where(and(eq(tenantUsage.tenantId, tenantId), eq(tenantUsage.metricType, metricType)))
      .orderBy(desc(tenantUsage.recordedAt))
      .limit(limit);

    return results.map((r) => ({
      id: r.id,
      tenantId: r.tenantId,
      metricType: r.metricType,
      metricValue: r.metricValue,
      period: r.period,
      recordedAt: r.recordedAt,
    }));
  } catch (error) {
    console.error("[MultiTenancy] Failed to get tenant usage:", error);
    return [];
  }
}

/**
 * Tenant context helpers for middleware isolation
 */

export function getKafkaTopic(tenant: Tenant, baseTopic: string): string {
  return `${tenant.kafkaTopicPrefix}${baseTopic}`;
}

export function getRedisKey(tenant: Tenant, baseKey: string): string {
  return `${tenant.redisNamespace}${baseKey}`;
}

export function getPermifyTenantId(tenant: Tenant): string {
  return tenant.permifyTenantId || `tenant_${tenant.tenantCode}`;
}

/**
 * Middleware isolation examples
 */

// Kafka topic isolation
export async function publishTenantEvent(tenant: Tenant, baseTopic: string, event: any): Promise<void> {
  const topic = getKafkaTopic(tenant, baseTopic);
  console.log(`[MultiTenancy] Publishing to tenant topic: ${topic}`);
  // Call Kafka publishEvent with tenant-specific topic
}

// Redis namespace isolation
export async function getTenantCache(tenant: Tenant, baseKey: string): Promise<any> {
  const key = getRedisKey(tenant, baseKey);
  console.log(`[MultiTenancy] Getting from tenant Redis key: ${key}`);
  // Call Redis getCache with tenant-specific key
  return null;
}

// Permify tenant isolation
export async function checkTenantPermission(
  tenant: Tenant,
  userId: number,
  resource: string,
  action: string
): Promise<boolean> {
  const tenantId = getPermifyTenantId(tenant);
  console.log(`[MultiTenancy] Checking permission for tenant: ${tenantId}`);
  // Call Permify checkPermission with tenant-specific context
  return true;
}
