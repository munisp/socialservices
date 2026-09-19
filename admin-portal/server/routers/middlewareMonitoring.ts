import { router, protectedProcedure } from "../_core/trpc";
import { z } from "zod";
import {
  getRecentMetrics,
  getActiveAlerts,
  acknowledgeAlert,
  resolveAlert,
  checkAllMiddlewareHealth,
  getLatestHealthStatus,
  type MiddlewareComponent,
} from "../services/middlewareMonitoring";

export const middlewareMonitoringRouter = router({
  // Get health status for all middleware
  getHealthStatus: protectedProcedure.query(async () => {
    return await getLatestHealthStatus();
  }),

  // Perform health check on all middleware
  checkHealth: protectedProcedure.mutation(async () => {
    return await checkAllMiddlewareHealth();
  }),

  // Get metrics for a component
  getMetrics: protectedProcedure
    .input(
      z.object({
        component: z.enum([
          "redis",
          "apisix",
          "kafka",
          "fluvio",
          "keycloak",
          "permify",
          "dapr",
          "temporal",
        ]),
        metricType: z.string(),
        minutes: z.number().optional(),
      })
    )
    .query(async ({ input }) => {
      return await getRecentMetrics(
        input.component as MiddlewareComponent,
        input.metricType,
        input.minutes
      );
    }),

  // Get active alerts
  getActiveAlerts: protectedProcedure.query(async () => {
    return await getActiveAlerts();
  }),

  // Acknowledge alert
  acknowledgeAlert: protectedProcedure
    .input(z.object({ alertId: z.number() }))
    .mutation(async ({ input, ctx }) => {
      await acknowledgeAlert(input.alertId, ctx.user.id);
      return { success: true };
    }),

  // Resolve alert
  resolveAlert: protectedProcedure
    .input(z.object({ alertId: z.number() }))
    .mutation(async ({ input, ctx }) => {
      await resolveAlert(input.alertId, ctx.user.id);
      return { success: true };
    }),
});
