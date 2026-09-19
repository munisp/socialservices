import { z } from "zod";
import { router, adminProcedure } from "../_core/trpc";
import { getDb } from "../db";
import { workflowExecutions, activityExecutions, workflowMetrics, workflowAlerts } from "../../drizzle/schema";
import { eq, desc, and, gte, lte, sql } from "drizzle-orm";

export const workflowRouter = router({
  // Get all workflow executions with filtering and pagination
  list: adminProcedure
    .input(
      z.object({
        workflowType: z.string().optional(),
        status: z.enum(["running", "completed", "failed", "cancelled", "timeout"]).optional(),
        startDate: z.string().optional(),
        endDate: z.string().optional(),
        limit: z.number().default(50),
        offset: z.number().default(0),
      })
    )
    .query(async ({ input }) => {
      const db = await getDb();
      if (!db) throw new Error("Database not available");

      const conditions = [];
      if (input.workflowType) {
        conditions.push(eq(workflowExecutions.workflowType, input.workflowType));
      }
      if (input.status) {
        conditions.push(eq(workflowExecutions.status, input.status));
      }
      if (input.startDate) {
        conditions.push(gte(workflowExecutions.startTime, new Date(input.startDate)));
      }
      if (input.endDate) {
        conditions.push(lte(workflowExecutions.startTime, new Date(input.endDate)));
      }

      const where = conditions.length > 0 ? and(...conditions) : undefined;

      const workflows = await db
        .select()
        .from(workflowExecutions)
        .where(where)
        .orderBy(desc(workflowExecutions.startTime))
        .limit(input.limit)
        .offset(input.offset);

      const total = await db
        .select({ count: sql<number>`count(*)` })
        .from(workflowExecutions)
        .where(where);

      return {
        workflows,
        total: total[0]?.count || 0,
      };
    }),

  // Get workflow details by ID
  getById: adminProcedure
    .input(z.object({ workflowId: z.string() }))
    .query(async ({ input }) => {
      const db = await getDb();
      if (!db) throw new Error("Database not available");

      const workflow = await db
        .select()
        .from(workflowExecutions)
        .where(eq(workflowExecutions.workflowId, input.workflowId))
        .limit(1);

      if (workflow.length === 0) {
        throw new Error("Workflow not found");
      }

      const activities = await db
        .select()
        .from(activityExecutions)
        .where(eq(activityExecutions.workflowId, input.workflowId))
        .orderBy(activityExecutions.startTime);

      return {
        workflow: workflow[0],
        activities,
      };
    }),

  // Get workflow metrics
  getMetrics: adminProcedure
    .input(
      z.object({
        workflowType: z.string().optional(),
        startDate: z.string().optional(),
        endDate: z.string().optional(),
      })
    )
    .query(async ({ input }) => {
      const db = await getDb();
      if (!db) throw new Error("Database not available");

      const conditions = [];
      if (input.workflowType) {
        conditions.push(eq(workflowMetrics.workflowType, input.workflowType));
      }
      if (input.startDate) {
        conditions.push(gte(workflowMetrics.date, new Date(input.startDate)));
      }
      if (input.endDate) {
        conditions.push(lte(workflowMetrics.date, new Date(input.endDate)));
      }

      const where = conditions.length > 0 ? and(...conditions) : undefined;

      const metrics = await db
        .select()
        .from(workflowMetrics)
        .where(where)
        .orderBy(desc(workflowMetrics.date));

      return metrics;
    }),

  // Get workflow statistics summary
  getStatistics: adminProcedure.query(async () => {
    const db = await getDb();
    if (!db) throw new Error("Database not available");

    const stats = await db
      .select({
        status: workflowExecutions.status,
        count: sql<number>`count(*)`,
        avgDuration: sql<number>`avg(${workflowExecutions.duration})`,
      })
      .from(workflowExecutions)
      .groupBy(workflowExecutions.status);

    const totalWorkflows = await db
      .select({ count: sql<number>`count(*)` })
      .from(workflowExecutions);

    const recentFailures = await db
      .select()
      .from(workflowExecutions)
      .where(eq(workflowExecutions.status, "failed"))
      .orderBy(desc(workflowExecutions.startTime))
      .limit(10);

    return {
      stats,
      totalWorkflows: totalWorkflows[0]?.count || 0,
      recentFailures,
    };
  }),

  // Get workflow alerts
  getAlerts: adminProcedure
    .input(
      z.object({
        resolved: z.boolean().optional(),
        severity: z.enum(["low", "medium", "high", "critical"]).optional(),
        limit: z.number().default(50),
      })
    )
    .query(async ({ input }) => {
      const db = await getDb();
      if (!db) throw new Error("Database not available");

      const conditions = [];
      if (input.resolved !== undefined) {
        conditions.push(eq(workflowAlerts.resolved, input.resolved ? 1 : 0));
      }
      if (input.severity) {
        conditions.push(eq(workflowAlerts.severity, input.severity));
      }

      const where = conditions.length > 0 ? and(...conditions) : undefined;

      const alerts = await db
        .select()
        .from(workflowAlerts)
        .where(where)
        .orderBy(desc(workflowAlerts.createdAt))
        .limit(input.limit);

      return alerts;
    }),

  // Resolve workflow alert
  resolveAlert: adminProcedure
    .input(
      z.object({
        alertId: z.number(),
        resolvedBy: z.string(),
      })
    )
    .mutation(async ({ input }) => {
      const db = await getDb();
      if (!db) throw new Error("Database not available");

      await db
        .update(workflowAlerts)
        .set({
          resolved: 1,
          resolvedAt: new Date(),
          resolvedBy: input.resolvedBy,
        })
        .where(eq(workflowAlerts.id, input.alertId));

      return { success: true };
    }),

  // Get workflow types summary
  getWorkflowTypes: adminProcedure.query(async () => {
    const db = await getDb();
    if (!db) throw new Error("Database not available");

    const types = await db
      .select({
        workflowType: workflowExecutions.workflowType,
        count: sql<number>`count(*)`,
        successRate: sql<number>`sum(case when status = 'completed' then 1 else 0 end) * 100.0 / count(*)`,
      })
      .from(workflowExecutions)
      .groupBy(workflowExecutions.workflowType);

    return types;
  }),
});
