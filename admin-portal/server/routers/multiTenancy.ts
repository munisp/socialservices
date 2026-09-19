import { router, protectedProcedure } from "../_core/trpc";
import { z } from "zod";
import {
  createTenant,
  getTenantById,
  getTenantByCode,
  getAllTenants,
  updateTenantStatus,
  addUserToTenant,
  removeUserFromTenant,
  getUserTenants,
  getTenantUsage,
  recordTenantUsage,
} from "../services/multiTenancy";

export const multiTenancyRouter = router({
  // Create tenant
  createTenant: protectedProcedure
    .input(
      z.object({
        tenantCode: z.string(),
        tenantName: z.string(),
        configuration: z.any().optional(),
      })
    )
    .mutation(async ({ input, ctx }) => {
      const tenantId = await createTenant(
        input.tenantCode,
        input.tenantName,
        ctx.user.id,
        input.configuration
      );
      return { tenantId };
    }),

  // Get tenant by ID
  getTenantById: protectedProcedure
    .input(z.object({ tenantId: z.number() }))
    .query(async ({ input }) => {
      return await getTenantById(input.tenantId);
    }),

  // Get tenant by code
  getTenantByCode: protectedProcedure
    .input(z.object({ tenantCode: z.string() }))
    .query(async ({ input }) => {
      return await getTenantByCode(input.tenantCode);
    }),

  // Get all tenants
  getAllTenants: protectedProcedure.query(async () => {
    return await getAllTenants();
  }),

  // Update tenant status
  updateTenantStatus: protectedProcedure
    .input(
      z.object({
        tenantId: z.number(),
        status: z.enum(["active", "suspended", "inactive"]),
      })
    )
    .mutation(async ({ input }) => {
      await updateTenantStatus(input.tenantId, input.status);
      return { success: true };
    }),

  // Add user to tenant
  addUserToTenant: protectedProcedure
    .input(
      z.object({
        tenantId: z.number(),
        userId: z.number(),
        role: z.enum(["owner", "admin", "member"]).optional(),
      })
    )
    .mutation(async ({ input }) => {
      await addUserToTenant(input.tenantId, input.userId, input.role);
      return { success: true };
    }),

  // Remove user from tenant
  removeUserFromTenant: protectedProcedure
    .input(
      z.object({
        tenantId: z.number(),
        userId: z.number(),
      })
    )
    .mutation(async ({ input }) => {
      await removeUserFromTenant(input.tenantId, input.userId);
      return { success: true };
    }),

  // Get user's tenants
  getUserTenants: protectedProcedure.query(async ({ ctx }) => {
    return await getUserTenants(ctx.user.id);
  }),

  // Get tenant usage
  getTenantUsage: protectedProcedure
    .input(
      z.object({
        tenantId: z.number(),
        metricType: z.string(),
        limit: z.number().optional(),
      })
    )
    .query(async ({ input }) => {
      return await getTenantUsage(input.tenantId, input.metricType, input.limit);
    }),

  // Record tenant usage
  recordTenantUsage: protectedProcedure
    .input(
      z.object({
        tenantId: z.number(),
        metricType: z.string(),
        metricValue: z.number(),
        period: z.string(),
      })
    )
    .mutation(async ({ input }) => {
      await recordTenantUsage(
        input.tenantId,
        input.metricType,
        input.metricValue,
        input.period
      );
      return { success: true };
    }),
});
