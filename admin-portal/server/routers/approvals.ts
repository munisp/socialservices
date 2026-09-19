import { z } from "zod";
import { router, adminProcedure, protectedProcedure } from "../_core/trpc";
import { TRPCError } from "@trpc/server";

/**
 * Approval Workflows Router
 * Multi-level approval system for sensitive operations
 */

export const approvalsRouter = router({
  // List approval chains
  listChains: adminProcedure.query(async () => {
    // Mock data
    return [
      {
        id: 1,
        name: "High Value Disbursement Approval",
        description: "Required for disbursements over $10,000",
        operationType: "high_value_disbursement",
        levels: [
          { level: 1, approvers: ["supervisor@example.com"], title: "Supervisor Approval" },
          { level: 2, approvers: ["manager@example.com"], title: "Manager Approval" },
          { level: 3, approvers: ["director@example.com"], title: "Director Approval" },
        ],
        enabled: true,
      },
      {
        id: 2,
        name: "Beneficiary Deletion Approval",
        description: "Required for permanent beneficiary record deletion",
        operationType: "beneficiary_deletion",
        levels: [
          { level: 1, approvers: ["data_admin@example.com"], title: "Data Admin Review" },
          { level: 2, approvers: ["compliance@example.com"], title: "Compliance Approval" },
        ],
        enabled: true,
      },
    ];
  }),

  // Create approval chain
  createChain: adminProcedure
    .input(
      z.object({
        name: z.string(),
        description: z.string().optional(),
        operationType: z.string(),
        levels: z.array(
          z.object({
            level: z.number(),
            approvers: z.array(z.string()),
            title: z.string(),
          })
        ),
        requiresAllApprovers: z.boolean().default(false),
        allowDelegation: z.boolean().default(true),
        autoEscalationHours: z.number().default(24),
        conditions: z.any().optional(),
      })
    )
    .mutation(async ({ input, ctx }) => {
      console.log("[Approvals] Creating chain:", input);
      
      return {
        success: true,
        chainId: Date.now(),
        message: "Approval chain created successfully",
      };
    }),

  // Submit approval request
  submitRequest: protectedProcedure
    .input(
      z.object({
        operationType: z.string(),
        operationData: z.any(),
        requestReason: z.string().optional(),
        workflowId: z.string().optional(),
        relatedEntityType: z.string().optional(),
        relatedEntityId: z.string().optional(),
      })
    )
    .mutation(async ({ input, ctx }) => {
      console.log("[Approvals] Submitting request:", input);
      
      // In real implementation:
      // 1. Find matching approval chain based on operation type and conditions
      // 2. Create approval request
      // 3. Notify first level approvers
      // 4. Return request ID
      
      return {
        success: true,
        requestId: Date.now(),
        chainId: 1,
        currentLevel: 1,
        totalLevels: 3,
        nextApprovers: ["supervisor@example.com"],
        message: "Approval request submitted successfully",
      };
    }),

  // Get pending approval requests (for current user)
  getPendingRequests: protectedProcedure.query(async ({ ctx }) => {
    // Mock data - filter by current user's approval permissions
    return [
      {
        id: 1,
        operationType: "high_value_disbursement",
        operationData: {
          beneficiaryId: "BEN-001",
          amount: 15000,
          programId: "PROG-001",
        },
        requestReason: "Emergency relief payment",
        currentLevel: 1,
        totalLevels: 3,
        requestedBy: "field_officer@example.com",
        requestedAt: new Date(Date.now() - 3600000),
        expiresAt: new Date(Date.now() + 82800000), // 23 hours remaining
      },
    ];
  }),

  // Get approval request details
  getRequestDetails: protectedProcedure
    .input(z.object({ requestId: z.number() }))
    .query(async ({ input }) => {
      // Mock data
      return {
        id: input.requestId,
        chainId: 1,
        chainName: "High Value Disbursement Approval",
        operationType: "high_value_disbursement",
        operationData: {
          beneficiaryId: "BEN-001",
          beneficiaryName: "John Doe",
          amount: 15000,
          programId: "PROG-001",
          programName: "Emergency Relief Program",
        },
        requestReason: "Emergency relief payment for flood victims",
        status: "pending",
        currentLevel: 1,
        totalLevels: 3,
        requestedBy: "field_officer@example.com",
        requestedAt: new Date(Date.now() - 3600000),
        expiresAt: new Date(Date.now() + 82800000),
        approvalHistory: [
          {
            level: 1,
            status: "pending",
            approvers: ["supervisor@example.com"],
          },
          {
            level: 2,
            status: "not_started",
            approvers: ["manager@example.com"],
          },
          {
            level: 3,
            status: "not_started",
            approvers: ["director@example.com"],
          },
        ],
      };
    }),

  // Approve request
  approveRequest: protectedProcedure
    .input(
      z.object({
        requestId: z.number(),
        comments: z.string().optional(),
        attachments: z.array(z.string()).optional(),
      })
    )
    .mutation(async ({ input, ctx }) => {
      console.log("[Approvals] Approving request:", input.requestId);
      
      // In real implementation:
      // 1. Verify user has permission to approve at current level
      // 2. Record approval action
      // 3. Check if all approvals at current level are complete
      // 4. If yes, advance to next level or mark as fully approved
      // 5. Send notifications
      // 6. If fully approved, trigger the approved operation
      
      return {
        success: true,
        requestStatus: "approved",
        nextLevel: 2,
        message: "Request approved successfully",
      };
    }),

  // Reject request
  rejectRequest: protectedProcedure
    .input(
      z.object({
        requestId: z.number(),
        comments: z.string(),
        attachments: z.array(z.string()).optional(),
      })
    )
    .mutation(async ({ input, ctx }) => {
      console.log("[Approvals] Rejecting request:", input.requestId);
      
      return {
        success: true,
        requestStatus: "rejected",
        message: "Request rejected",
      };
    }),

  // Delegate approval
  delegateApproval: protectedProcedure
    .input(
      z.object({
        requestId: z.number(),
        delegateToUserId: z.number(),
        delegationReason: z.string(),
      })
    )
    .mutation(async ({ input, ctx }) => {
      console.log("[Approvals] Delegating approval:", input);
      
      return {
        success: true,
        message: "Approval delegated successfully",
      };
    }),

  // Create delegation rule
  createDelegation: protectedProcedure
    .input(
      z.object({
        delegateToUserId: z.number(),
        operationType: z.string().optional(),
        startDate: z.date(),
        endDate: z.date(),
        reason: z.string(),
      })
    )
    .mutation(async ({ input, ctx }) => {
      console.log("[Approvals] Creating delegation:", input);
      
      return {
        success: true,
        delegationId: Date.now(),
        message: "Delegation created successfully",
      };
    }),

  // Get user's delegations
  getMyDelegations: protectedProcedure.query(async ({ ctx }) => {
    // Mock data
    return {
      outgoing: [
        {
          id: 1,
          delegateToUserId: 2,
          delegateToName: "Jane Smith",
          operationType: "high_value_disbursement",
          startDate: new Date(),
          endDate: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000),
          active: true,
        },
      ],
      incoming: [
        {
          id: 2,
          delegatorId: 3,
          delegatorName: "Bob Johnson",
          operationType: null, // All types
          startDate: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000),
          endDate: new Date(Date.now() + 5 * 24 * 60 * 60 * 1000),
          active: true,
        },
      ],
    };
  }),

  // Get approval history
  getApprovalHistory: protectedProcedure
    .input(
      z.object({
        requestId: z.number().optional(),
        operationType: z.string().optional(),
        status: z.enum(["pending", "approved", "rejected", "cancelled", "expired"]).optional(),
        limit: z.number().default(50),
      })
    )
    .query(async ({ input }) => {
      // Mock data
      return [
        {
          id: 1,
          operationType: "high_value_disbursement",
          operationData: { amount: 15000, beneficiaryId: "BEN-001" },
          status: "approved",
          requestedBy: "field_officer@example.com",
          requestedAt: new Date(Date.now() - 86400000),
          completedAt: new Date(Date.now() - 43200000),
          approvalActions: [
            {
              level: 1,
              action: "approved",
              approverName: "Supervisor Smith",
              actionedAt: new Date(Date.now() - 79200000),
            },
            {
              level: 2,
              action: "approved",
              approverName: "Manager Jones",
              actionedAt: new Date(Date.now() - 43200000),
            },
          ],
        },
      ];
    }),

  // Get approval statistics
  getStatistics: adminProcedure
    .input(
      z.object({
        startDate: z.date(),
        endDate: z.date(),
      })
    )
    .query(async ({ input }) => {
      // Mock data
      return {
        totalRequests: 245,
        approved: 198,
        rejected: 32,
        pending: 15,
        expired: 0,
        averageApprovalTime: 4.5, // hours
        approvalRate: 80.8,
        byOperationType: [
          {
            operationType: "high_value_disbursement",
            total: 150,
            approved: 125,
            rejected: 20,
            pending: 5,
          },
          {
            operationType: "beneficiary_deletion",
            total: 95,
            approved: 73,
            rejected: 12,
            pending: 10,
          },
        ],
      };
    }),

  // Cancel approval request
  cancelRequest: protectedProcedure
    .input(
      z.object({
        requestId: z.number(),
        reason: z.string(),
      })
    )
    .mutation(async ({ input, ctx }) => {
      console.log("[Approvals] Cancelling request:", input);
      
      return {
        success: true,
        message: "Approval request cancelled",
      };
    }),
});
