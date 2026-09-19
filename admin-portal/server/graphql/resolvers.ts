import { GraphQLError } from "graphql";
import { PubSub } from "graphql-subscriptions";
import { getDb } from "../db";
import { users, beneficiaries, benefitPrograms, transactions } from "../../drizzle/schema";
import { eq, desc } from "drizzle-orm";
import { getAllTenants, getUserTenants, createTenant, updateTenantStatus } from "../services/multiTenancy";

const pubsub = new PubSub();

// Event names for subscriptions
export const EVENTS = {
  DASHBOARD_UPDATED: "DASHBOARD_UPDATED",
  TRANSACTION_CREATED: "TRANSACTION_CREATED",
  TRANSACTION_UPDATED: "TRANSACTION_UPDATED",
  FRAUD_ALERT_CREATED: "FRAUD_ALERT_CREATED",
  MIDDLEWARE_HEALTH_UPDATED: "MIDDLEWARE_HEALTH_UPDATED",
};

/**
 * GraphQL Resolvers
 */
export const resolvers = {
  Query: {
    // User queries
    me: async (_: any, __: any, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }
      return context.user;
    },

    getUser: async (_: any, { id }: { id: number }, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const db = await getDb();
      if (!db) return null;

      const results = await db.select().from(users).where(eq(users.id, id)).limit(1);
      return results.length > 0 ? results[0] : null;
    },

    getAllUsers: async (_: any, __: any, context: any) => {
      if (!context.user || context.user.role !== "admin") {
        throw new GraphQLError("Not authorized", {
          extensions: { code: "FORBIDDEN" },
        });
      }

      const db = await getDb();
      if (!db) return [];

      return await db.select().from(users).limit(100);
    },

    // Beneficiary queries
    getBeneficiary: async (_: any, { id }: { id: number }, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const db = await getDb();
      if (!db) return null;

      const results = await db.select().from(beneficiaries).where(eq(beneficiaries.id, id)).limit(1);
      return results.length > 0 ? results[0] : null;
    },

    getAllBeneficiaries: async (_: any, { limit = 50, offset = 0 }: { limit?: number; offset?: number }, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const db = await getDb();
      if (!db) return [];

      return await db.select().from(beneficiaries).limit(limit).offset(offset);
    },

    searchBeneficiaries: async (_: any, { query }: { query: string }, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const db = await getDb();
      if (!db) return [];

      // Simple search implementation (can be enhanced with full-text search)
      const results = await db.select().from(beneficiaries).limit(50);
      return results.filter(
        (b) =>
          b.firstName.toLowerCase().includes(query.toLowerCase()) ||
          b.lastName.toLowerCase().includes(query.toLowerCase()) ||
          (b.nationalId && b.nationalId.includes(query))
      );
    },

    // Program queries
    getProgram: async (_: any, { id }: { id: number }, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const db = await getDb();
      if (!db) return null;

      const results = await db.select().from(benefitPrograms).where(eq(benefitPrograms.id, id)).limit(1);
      return results.length > 0 ? results[0] : null;
    },

    getAllPrograms: async (_: any, __: any, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const db = await getDb();
      if (!db) return [];

      return await db.select().from(benefitPrograms).limit(100);
    },

    // Transaction queries
    getTransaction: async (_: any, { id }: { id: number }, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const db = await getDb();
      if (!db) return null;

      const results = await db.select().from(transactions).where(eq(transactions.id, id)).limit(1);
      return results.length > 0 ? results[0] : null;
    },

    getTransactionsByBeneficiary: async (_: any, { beneficiaryId }: { beneficiaryId: number }, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const db = await getDb();
      if (!db) return [];

      return await db
        .select()
        .from(transactions)
        .where(eq(transactions.beneficiaryId, beneficiaryId))
        .orderBy(desc(transactions.transactionDate))
        .limit(100);
    },

    getAllTransactions: async (_: any, { limit = 50, offset = 0 }: { limit?: number; offset?: number }, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const db = await getDb();
      if (!db) return [];

      return await db.select().from(transactions).orderBy(desc(transactions.transactionDate)).limit(limit).offset(offset);
    },

    // Analytics queries
    getDashboardMetrics: async (_: any, __: any, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const db = await getDb();
      if (!db) {
        return {
          totalUsers: 0,
          totalBeneficiaries: 0,
          activePrograms: 0,
          totalDisbursements: 0,
          pendingApprovals: 0,
        };
      }

      const userCount = await db.select().from(users);
      const beneficiaryCount = await db.select().from(beneficiaries);
      const programCount = await db.select().from(benefitPrograms).where(eq(benefitPrograms.status, "active"));

      return {
        totalUsers: userCount.length,
        totalBeneficiaries: beneficiaryCount.length,
        activePrograms: programCount.length,
        totalDisbursements: 0, // Placeholder
        pendingApprovals: 0, // Placeholder
      };
    },

    getFraudAlerts: async (_: any, { limit = 20 }: { limit?: number }, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      // Placeholder - would query fraud_alerts table
      return [];
    },

    // Middleware queries
    getMiddlewareHealth: async (_: any, __: any, context: any) => {
      if (!context.user || context.user.role !== "admin") {
        throw new GraphQLError("Not authorized", {
          extensions: { code: "FORBIDDEN" },
        });
      }

      // Placeholder - would query middleware health
      return [
        { component: "Redis", status: "healthy", lastCheck: new Date().toISOString(), responseTime: 5.2, errorRate: 0 },
        { component: "Kafka", status: "healthy", lastCheck: new Date().toISOString(), responseTime: 12.1, errorRate: 0 },
      ];
    },

    getMiddlewareMetrics: async (_: any, { component, hours = 24 }: { component: string; hours?: number }, context: any) => {
      if (!context.user || context.user.role !== "admin") {
        throw new GraphQLError("Not authorized", {
          extensions: { code: "FORBIDDEN" },
        });
      }

      // Placeholder - would query middleware metrics
      return [];
    },

    // Tenant queries
    getTenant: async (_: any, { id }: { id: number }, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const tenants = await getAllTenants();
      return tenants.find((t) => t.id === id) || null;
    },

    getAllTenants: async (_: any, __: any, context: any) => {
      if (!context.user || context.user.role !== "admin") {
        throw new GraphQLError("Not authorized", {
          extensions: { code: "FORBIDDEN" },
        });
      }

      return await getAllTenants();
    },

    getMyTenants: async (_: any, __: any, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      return await getUserTenants(context.user.id);
    },
  },

  Mutation: {
    // Beneficiary mutations
    createBeneficiary: async (_: any, { input }: any, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const db = await getDb();
      if (!db) throw new GraphQLError("Database unavailable");

      const result = await db.insert(beneficiaries).values({
        ...input,
        status: "active",
        kycStatus: "pending",
      });

      const newBeneficiary = await db
        .select()
        .from(beneficiaries)
        .where(eq(beneficiaries.id, result[0].insertId))
        .limit(1);

      return newBeneficiary[0];
    },

    updateBeneficiary: async (_: any, { id, input }: any, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const db = await getDb();
      if (!db) throw new GraphQLError("Database unavailable");

      await db.update(beneficiaries).set(input).where(eq(beneficiaries.id, id));

      const updated = await db.select().from(beneficiaries).where(eq(beneficiaries.id, id)).limit(1);

      return updated[0];
    },

    deleteBeneficiary: async (_: any, { id }: { id: number }, context: any) => {
      if (!context.user || context.user.role !== "admin") {
        throw new GraphQLError("Not authorized", {
          extensions: { code: "FORBIDDEN" },
        });
      }

      const db = await getDb();
      if (!db) throw new GraphQLError("Database unavailable");

      await db.delete(beneficiaries).where(eq(beneficiaries.id, id));
      return true;
    },

    // Program mutations
    createProgram: async (_: any, { input }: any, context: any) => {
      if (!context.user || context.user.role !== "admin") {
        throw new GraphQLError("Not authorized", {
          extensions: { code: "FORBIDDEN" },
        });
      }

      const db = await getDb();
      if (!db) throw new GraphQLError("Database unavailable");

      const result = await db.insert(benefitPrograms).values({
        ...input,
        status: "draft",
        createdBy: context.user.id,
      });

      const newProgram = await db
        .select()
        .from(benefitPrograms)
        .where(eq(benefitPrograms.id, result[0].insertId))
        .limit(1);

      return newProgram[0];
    },

    updateProgram: async (_: any, { id, input }: any, context: any) => {
      if (!context.user || context.user.role !== "admin") {
        throw new GraphQLError("Not authorized", {
          extensions: { code: "FORBIDDEN" },
        });
      }

      const db = await getDb();
      if (!db) throw new GraphQLError("Database unavailable");

      await db.update(benefitPrograms).set(input).where(eq(benefitPrograms.id, id));

      const updated = await db.select().from(benefitPrograms).where(eq(benefitPrograms.id, id)).limit(1);

      return updated[0];
    },

    deleteProgram: async (_: any, { id }: { id: number }, context: any) => {
      if (!context.user || context.user.role !== "admin") {
        throw new GraphQLError("Not authorized", {
          extensions: { code: "FORBIDDEN" },
        });
      }

      const db = await getDb();
      if (!db) throw new GraphQLError("Database unavailable");

      await db.delete(benefitPrograms).where(eq(benefitPrograms.id, id));
      return true;
    },

    // Transaction mutations
    createDisbursement: async (_: any, { beneficiaryId, amount, programId }: any, context: any) => {
      if (!context.user) {
        throw new GraphQLError("Not authenticated", {
          extensions: { code: "UNAUTHENTICATED" },
        });
      }

      const db = await getDb();
      if (!db) throw new GraphQLError("Database unavailable");

      const result = await db.insert(transactions).values({
        beneficiaryId,
        programId: programId || null,
        amount,
        currency: "USD",
        transactionType: "purchase", // Using 'purchase' as valid enum value
        status: "pending",
        transactionId: `TXN-${Date.now()}`,
        mccCode: "0000",
        transactionDate: new Date(),
      });

      const newTransaction = await db
        .select()
        .from(transactions)
        .where(eq(transactions.id, result[0].insertId))
        .limit(1);

      // Publish subscription event
      pubsub.publish(EVENTS.TRANSACTION_CREATED, {
        transactionCreated: newTransaction[0],
      });

      return newTransaction[0];
    },

    // Tenant mutations
    createTenant: async (_: any, { tenantCode, tenantName }: any, context: any) => {
      if (!context.user || context.user.role !== "admin") {
        throw new GraphQLError("Not authorized", {
          extensions: { code: "FORBIDDEN" },
        });
      }

      const tenantId = await createTenant(tenantCode, tenantName, context.user.id);
      if (!tenantId) throw new GraphQLError("Failed to create tenant");

      const tenants = await getAllTenants();
      return tenants.find((t) => t.id === tenantId)!;
    },

    updateTenantStatus: async (_: any, { id, status }: any, context: any) => {
      if (!context.user || context.user.role !== "admin") {
        throw new GraphQLError("Not authorized", {
          extensions: { code: "FORBIDDEN" },
        });
      }

      await updateTenantStatus(id, status);

      const tenants = await getAllTenants();
      return tenants.find((t) => t.id === id)!;
    },
  },

  Subscription: {
    dashboardUpdated: {
      subscribe: () => pubsub.asyncIterableIterator([EVENTS.DASHBOARD_UPDATED]),
    },

    transactionCreated: {
      subscribe: () => pubsub.asyncIterableIterator([EVENTS.TRANSACTION_CREATED]),
    },

    transactionUpdated: {
      subscribe: (_: any, { beneficiaryId }: { beneficiaryId?: number }) => {
        // Can filter by beneficiaryId if provided
        return pubsub.asyncIterableIterator([EVENTS.TRANSACTION_UPDATED]);
      },
    },

    fraudAlertCreated: {
      subscribe: () => pubsub.asyncIterableIterator([EVENTS.FRAUD_ALERT_CREATED]),
    },

    middlewareHealthUpdated: {
      subscribe: () => pubsub.asyncIterableIterator([EVENTS.MIDDLEWARE_HEALTH_UPDATED]),
    },
  },
};

// Helper function to publish dashboard updates
export function publishDashboardUpdate(metrics: any) {
  pubsub.publish(EVENTS.DASHBOARD_UPDATED, {
    dashboardUpdated: metrics,
  });
}

// Helper function to publish fraud alerts
export function publishFraudAlert(alert: any) {
  pubsub.publish(EVENTS.FRAUD_ALERT_CREATED, {
    fraudAlertCreated: alert,
  });
}
