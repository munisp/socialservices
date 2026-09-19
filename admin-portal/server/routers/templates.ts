import { z } from "zod";
import { router, adminProcedure, protectedProcedure } from "../_core/trpc";
import { TRPCError } from "@trpc/server";

/**
 * Workflow Templates Router
 * Provides endpoints for template management, versioning, and deployment
 */

export const templatesRouter = router({
  // List all templates (with filtering)
  list: protectedProcedure
    .input(
      z.object({
        category: z.string().optional(),
        status: z.enum(["draft", "active", "archived", "deprecated"]).optional(),
        search: z.string().optional(),
        tags: z.array(z.string()).optional(),
        limit: z.number().default(50),
        offset: z.number().default(0),
      })
    )
    .query(async ({ input, ctx }) => {
      // Mock data - replace with actual database queries
      return {
        templates: [
          {
            id: 1,
            name: "Standard Cash Transfer Program",
            description: "Standard workflow for unconditional cash transfer programs",
            workflowType: "EnrollmentWorkflow",
            category: "cash_transfer",
            version: "1.0.0",
            tags: ["cash", "unconditional", "standard"],
            usageCount: 45,
            status: "active",
            visibility: "public",
            createdAt: new Date("2024-01-15"),
          },
          {
            id: 2,
            name: "Emergency Relief Disbursement",
            description: "Fast-track disbursement for emergency situations",
            workflowType: "DisbursementProcessingWorkflow",
            category: "emergency_relief",
            version: "2.1.0",
            tags: ["emergency", "fast-track", "relief"],
            usageCount: 23,
            status: "active",
            visibility: "public",
            createdAt: new Date("2024-02-10"),
          },
        ],
        total: 2,
      };
    }),

  // Get template by ID
  getById: protectedProcedure
    .input(z.object({ id: z.number() }))
    .query(async ({ input }) => {
      // Mock data
      return {
        id: input.id,
        name: "Standard Cash Transfer Program",
        description: "Standard workflow for unconditional cash transfer programs",
        workflowType: "EnrollmentWorkflow",
        category: "cash_transfer",
        version: "1.0.0",
        tags: ["cash", "unconditional", "standard"],
        configuration: {
          eligibilityCriteria: ["age >= 18", "income < 5000"],
          disbursementSchedule: "monthly",
          benefitAmount: 1000,
        },
        defaultParameters: {
          programName: "Cash Transfer Program",
          duration: 12,
        },
        requiredParameters: ["programId", "beneficiaryId", "amount"],
        usageCount: 45,
        status: "active",
        visibility: "public",
      };
    }),

  // Create new template
  create: adminProcedure
    .input(
      z.object({
        name: z.string(),
        description: z.string().optional(),
        workflowType: z.string(),
        category: z.string().optional(),
        configuration: z.any(),
        defaultParameters: z.any().optional(),
        requiredParameters: z.array(z.string()).optional(),
        tags: z.array(z.string()).optional(),
        visibility: z.enum(["private", "team", "public"]).default("private"),
      })
    )
    .mutation(async ({ input, ctx }) => {
      console.log("[Templates] Creating template:", input);
      
      return {
        success: true,
        templateId: Date.now(),
        message: "Template created successfully",
      };
    }),

  // Update template
  update: adminProcedure
    .input(
      z.object({
        id: z.number(),
        name: z.string().optional(),
        description: z.string().optional(),
        configuration: z.any().optional(),
        defaultParameters: z.any().optional(),
        requiredParameters: z.array(z.string()).optional(),
        tags: z.array(z.string()).optional(),
        visibility: z.enum(["private", "team", "public"]).optional(),
        status: z.enum(["draft", "active", "archived", "deprecated"]).optional(),
      })
    )
    .mutation(async ({ input, ctx }) => {
      console.log("[Templates] Updating template:", input);
      
      return {
        success: true,
        message: "Template updated successfully",
      };
    }),

  // Delete template
  delete: adminProcedure
    .input(z.object({ id: z.number() }))
    .mutation(async ({ input }) => {
      console.log("[Templates] Deleting template:", input.id);
      
      return {
        success: true,
        message: "Template deleted successfully",
      };
    }),

  // Publish template (make it active)
  publish: adminProcedure
    .input(z.object({ id: z.number() }))
    .mutation(async ({ input }) => {
      console.log("[Templates] Publishing template:", input.id);
      
      return {
        success: true,
        message: "Template published successfully",
      };
    }),

  // Create new version of template
  createVersion: adminProcedure
    .input(
      z.object({
        templateId: z.number(),
        version: z.string(),
        configuration: z.any(),
        defaultParameters: z.any().optional(),
        requiredParameters: z.array(z.string()).optional(),
        changeLog: z.string(),
      })
    )
    .mutation(async ({ input, ctx }) => {
      console.log("[Templates] Creating new version:", input);
      
      return {
        success: true,
        versionId: Date.now(),
        message: "Template version created successfully",
      };
    }),

  // Get template versions
  getVersions: protectedProcedure
    .input(z.object({ templateId: z.number() }))
    .query(async ({ input }) => {
      // Mock data
      return [
        {
          id: 1,
          templateId: input.templateId,
          version: "1.0.0",
          changeLog: "Initial version",
          createdAt: new Date("2024-01-15"),
        },
        {
          id: 2,
          templateId: input.templateId,
          version: "1.1.0",
          changeLog: "Added new eligibility criteria",
          createdAt: new Date("2024-02-01"),
        },
      ];
    }),

  // Deploy template (create workflow from template)
  deploy: adminProcedure
    .input(
      z.object({
        templateId: z.number(),
        templateVersion: z.string().optional(),
        parameters: z.any(),
        deploymentNotes: z.string().optional(),
      })
    )
    .mutation(async ({ input, ctx }) => {
      console.log("[Templates] Deploying template:", input);
      
      // In real implementation:
      // 1. Load template configuration
      // 2. Merge with provided parameters
      // 3. Validate required parameters
      // 4. Create workflow execution in Temporal
      // 5. Track deployment
      
      return {
        success: true,
        workflowId: `WF-${Date.now()}`,
        deploymentId: Date.now(),
        message: "Template deployed successfully",
      };
    }),

  // Get deployment history
  getDeployments: protectedProcedure
    .input(
      z.object({
        templateId: z.number().optional(),
        status: z.enum(["pending", "active", "completed", "failed", "cancelled"]).optional(),
        limit: z.number().default(50),
      })
    )
    .query(async ({ input }) => {
      // Mock data
      return [
        {
          id: 1,
          templateId: 1,
          templateVersion: "1.0.0",
          workflowId: "WF-001",
          status: "completed",
          deployedBy: 1,
          startedAt: new Date(Date.now() - 86400000),
          completedAt: new Date(Date.now() - 3600000),
        },
      ];
    }),

  // Add template to favorites
  addToFavorites: protectedProcedure
    .input(z.object({ templateId: z.number() }))
    .mutation(async ({ input, ctx }) => {
      console.log("[Templates] Adding to favorites:", input.templateId);
      
      return {
        success: true,
        message: "Template added to favorites",
      };
    }),

  // Remove template from favorites
  removeFromFavorites: protectedProcedure
    .input(z.object({ templateId: z.number() }))
    .mutation(async ({ input, ctx }) => {
      console.log("[Templates] Removing from favorites:", input.templateId);
      
      return {
        success: true,
        message: "Template removed from favorites",
      };
    }),

  // Get user's favorite templates
  getFavorites: protectedProcedure.query(async ({ ctx }) => {
    // Mock data
    return [
      {
        id: 1,
        name: "Standard Cash Transfer Program",
        workflowType: "EnrollmentWorkflow",
        category: "cash_transfer",
      },
    ];
  }),

  // Get template categories
  getCategories: protectedProcedure.query(async () => {
    // Mock data
    return [
      {
        id: 1,
        name: "cash_transfer",
        description: "Cash transfer programs",
        icon: "dollar-sign",
        color: "#10b981",
      },
      {
        id: 2,
        name: "food_assistance",
        description: "Food assistance programs",
        icon: "shopping-cart",
        color: "#f59e0b",
      },
      {
        id: 3,
        name: "emergency_relief",
        description: "Emergency relief programs",
        icon: "alert-triangle",
        color: "#ef4444",
      },
    ];
  }),

  // Preview template (validate and show what would be deployed)
  preview: protectedProcedure
    .input(
      z.object({
        templateId: z.number(),
        parameters: z.any(),
      })
    )
    .query(async ({ input }) => {
      // Mock data
      return {
        templateName: "Standard Cash Transfer Program",
        workflowType: "EnrollmentWorkflow",
        mergedConfiguration: {
          ...input.parameters,
          eligibilityCriteria: ["age >= 18", "income < 5000"],
          disbursementSchedule: "monthly",
        },
        validationErrors: [],
        estimatedDuration: 300000, // 5 minutes
      };
    }),
});
