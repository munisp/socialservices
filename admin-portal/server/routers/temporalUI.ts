import { z } from "zod";
import { router, adminProcedure } from "../_core/trpc";

/**
 * Temporal UI Integration Router
 * Provides endpoints for Temporal UI integration and workflow linking
 */

const TEMPORAL_UI_URL = process.env.TEMPORAL_UI_URL || "http://localhost:8080";
const TEMPORAL_NAMESPACE = process.env.TEMPORAL_NAMESPACE || "default";

export const temporalUIRouter = router({
  // Get Temporal UI configuration
  getConfig: adminProcedure.query(() => {
    return {
      temporalUIUrl: TEMPORAL_UI_URL,
      namespace: TEMPORAL_NAMESPACE,
      proxyPath: "/api/temporal-ui",
    };
  }),

  // Get direct link to workflow in Temporal UI
  getWorkflowLink: adminProcedure
    .input(
      z.object({
        workflowId: z.string(),
        runId: z.string(),
      })
    )
    .query(({ input }) => {
      const url = `${TEMPORAL_UI_URL}/namespaces/${TEMPORAL_NAMESPACE}/workflows/${input.workflowId}/${input.runId}`;
      return {
        url,
        proxyUrl: `/api/temporal-ui/namespaces/${TEMPORAL_NAMESPACE}/workflows/${input.workflowId}/${input.runId}`,
      };
    }),

  // Check Temporal UI health
  checkHealth: adminProcedure.query(async () => {
    try {
      const response = await fetch(`${TEMPORAL_UI_URL}/health`, {
        signal: AbortSignal.timeout(5000),
      });

      return {
        status: response.ok ? "healthy" : "unhealthy",
        statusCode: response.status,
        temporalUIUrl: TEMPORAL_UI_URL,
      };
    } catch (error) {
      return {
        status: "unavailable",
        error: error instanceof Error ? error.message : "Unknown error",
        temporalUIUrl: TEMPORAL_UI_URL,
      };
    }
  }),

  // Get workflow execution details from Temporal
  getWorkflowDetails: adminProcedure
    .input(
      z.object({
        workflowId: z.string(),
        runId: z.string(),
      })
    )
    .query(async ({ input }) => {
      try {
        // This would typically use Temporal client SDK
        // For now, return a link to the Temporal UI
        return {
          workflowId: input.workflowId,
          runId: input.runId,
          temporalUILink: `/api/temporal-ui/namespaces/${TEMPORAL_NAMESPACE}/workflows/${input.workflowId}/${input.runId}`,
        };
      } catch (error) {
        throw new Error(`Failed to get workflow details: ${error instanceof Error ? error.message : "Unknown error"}`);
      }
    }),
});
