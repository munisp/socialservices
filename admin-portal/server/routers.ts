import { COOKIE_NAME } from "@shared/const";
import { globalSearch } from "./opensearch";
import { syncProgram, syncUser, syncMccCode, syncAuditLog } from "./opensearchSync";
import { TRPCError } from "@trpc/server";
import { z } from "zod";
import { getSessionCookieOptions } from "./_core/cookies";
import { systemRouter } from "./_core/systemRouter";
import { protectedProcedure, publicProcedure, router } from "./_core/trpc";
import * as db from "./db";
import * as beneficiaryDb from "./beneficiaries";
import * as transactionDb from "./transactions";
import * as exportUtils from "./exports";
import { sendAdminNotification, NotificationTemplates } from "./emailNotification";
import { broadcastDashboardUpdate } from "./websocket";
import { executeApprovedRequest } from "./approvalExecutor";
import { middlewareMonitoringRouter } from "./routers/middlewareMonitoring";
import { eventReplayRouter } from "./routers/eventReplay";
import { multiTenancyRouter } from "./routers/multiTenancy";
import { workflowRouter } from "./routers/workflow";
import { temporalUIRouter } from "./routers/temporalUI";
import { workflowControlRouter } from "./routers/workflowControl";
import { auditRouter } from "./routers/audit";
import { worldClassRouter } from "./routers/worldClass";
import { stakeholderOnboardingRouter } from "./routers/stakeholderOnboarding";

// Admin-only procedure
const adminProcedure = protectedProcedure.use(({ ctx, next }) => {
  if (ctx.user.role !== "admin") {
    throw new TRPCError({ code: "FORBIDDEN", message: "Admin access required" });
  }
  return next({ ctx });
});

export const appRouter = router({
  savedFilters: router({
    list: protectedProcedure.query(async ({ ctx }) => {
      return db.getSavedSearchFiltersByUserId(ctx.user.id);
    }),
    create: protectedProcedure
      .input(
        z.object({
          name: z.string().min(1).max(255),
          query: z.string(),
          entityType: z.string(),
        })
      )
      .mutation(async ({ ctx, input }) => {
        const filterId = await db.createSavedSearchFilter({
          userId: ctx.user.id,
          ...input,
        });
        return { filterId };
      }),
    delete: protectedProcedure
      .input(z.object({ id: z.number() }))
      .mutation(async ({ input }) => {
        await db.deleteSavedSearchFilter(input.id);
        return { success: true };
      }),
  }),
  approvals: router({
    getPending: protectedProcedure.query(async () => {
      return db.getAllPendingApprovalRequests();
    }),
    getById: protectedProcedure
      .input(z.object({ id: z.number() }))
      .query(async ({ input }) => {
        const request = await db.getApprovalRequestById(input.id);
        if (!request) throw new TRPCError({ code: "NOT_FOUND", message: "Approval request not found" });
        const approvals = await db.getApprovalsForRequest(input.id);
        return { ...request, approvals };
      }),
    createRequest: protectedProcedure
      .input(
        z.object({
          requestType: z.enum(["ROLE_CHANGE", "BATCH_PROGRAM_UPDATE", "BATCH_FLAG_TOGGLE", "PROGRAM_DELETE", "USER_DELETE"]),
          targetId: z.number().optional(),
          requestData: z.any(),
          justification: z.string().min(10),
        })
      )
      .mutation(async ({ ctx, input }) => {
        const requestId = await db.createApprovalRequest({
          requestType: input.requestType,
          requestedBy: ctx.user.id,
          targetId: input.targetId,
          requestData: input.requestData,
          justification: input.justification,
        });
        // Broadcast will be handled by WebSocket listeners
        return { requestId };
      }),
    approve: protectedProcedure
      .input(
        z.object({
          requestId: z.number(),
          decision: z.enum(["approved", "rejected"]),
          comment: z.string().optional(),
        })
      )
      .mutation(async ({ ctx, input }) => {
        const approvalId = await db.createApproval({
          requestId: input.requestId,
          approvedBy: ctx.user.id,
          decision: input.decision,
          comment: input.comment,
        });
        
        // Check if request is now fully approved
        const request = await db.getApprovalRequestById(input.requestId);
        if (request?.status === "approved") {
          // Execute the approved action
          await executeApprovedRequest(request);
        }
        
        // Broadcast will be handled by WebSocket listeners
        return { approvalId };
      }),
  }),
  search: router({
    global: protectedProcedure
      .input(
        z.object({
          query: z.string().min(1),
          types: z.array(z.enum(["PROGRAMS", "USERS", "MCC_CODES", "AUDIT_LOGS"])).optional(),
          dateRange: z
            .object({
              from: z.string().optional(),
              to: z.string().optional(),
            })
            .optional(),
        })
      )
      .query(async ({ input }) => {
        const results = await globalSearch(input.query, {
          types: input.types,
          dateRange: input.dateRange,
        });
        return results;
      }),
  }),
  system: systemRouter,
  auth: router({
    me: publicProcedure.query((opts) => opts.ctx.user),
    logout: publicProcedure.mutation(({ ctx }) => {
      const cookieOptions = getSessionCookieOptions(ctx.req);
      ctx.res.clearCookie(COOKIE_NAME, { ...cookieOptions, maxAge: -1 });
      return {
        success: true,
      } as const;
    }),
  }),

  // ===== Benefit Programs =====
  programs: router({
    list: adminProcedure.query(async () => {
      return await db.getAllBenefitPrograms();
    }),

    getById: adminProcedure.input(z.object({ id: z.number() })).query(async ({ input }) => {
      const program = await db.getBenefitProgramById(input.id);
      if (!program) {
        throw new TRPCError({ code: "NOT_FOUND", message: "Program not found" });
      }
      return program;
    }),

    create: adminProcedure
      .input(
        z.object({
          name: z.string().min(1),
          accountType: z.string().min(1),
          description: z.string().optional(),
          status: z.enum(["active", "inactive", "pending_review"]).default("active"),
        })
      )
      .mutation(async ({ input, ctx }) => {
        return await db.createBenefitProgram({
          ...input,
          createdBy: ctx.user.id,
        });
      }),

    update: adminProcedure
      .input(
        z.object({
          id: z.number(),
          name: z.string().min(1).optional(),
          description: z.string().optional(),
          status: z.enum(["active", "inactive", "pending_review"]).optional(),
        })
      )
      .mutation(async ({ input }) => {
        const { id, ...updates } = input;
        await db.updateBenefitProgram(id, updates);
        return { success: true };
      }),

    batchUpdateStatus: adminProcedure
      .input(
        z.object({
          programIds: z.array(z.number()),
          isActive: z.boolean(),
          justification: z.string().min(10),
        })
      )
      .mutation(async ({ input, ctx }) => {
        for (const programId of input.programIds) {
          await db.updateBenefitProgramStatus(programId, input.isActive);
        }

        await db.logAdminAction({
          action: "batch_update_programs",
          targetUserId: null,
          details: { programIds: input.programIds, isActive: input.isActive },
          justification: input.justification,
          performedBy: ctx.user.id,
        });

        // Send email notification
        await sendAdminNotification(
          NotificationTemplates.batchProgramUpdate({
            count: input.programIds.length,
            action: input.isActive ? "activated" : "deactivated",
            performedBy: ctx.user.name || ctx.user.email || "Unknown Admin",
          })
        );

        return { success: true, count: input.programIds.length };
      }),

    // Soft delete - marks program as deleted using deletedAt timestamp (not status)
    softDelete: adminProcedure
      .input(
        z.object({
          id: z.number(),
          justification: z.string().min(10),
        })
      )
      .mutation(async ({ input, ctx }) => {
        const program = await db.getBenefitProgramById(input.id);
        if (!program) {
          throw new TRPCError({ code: "NOT_FOUND", message: "Program not found" });
        }

        // Check if already deleted
        if (program.deletedAt) {
          throw new TRPCError({ code: "BAD_REQUEST", message: "Program is already deleted" });
        }

        // Soft delete by setting deletedAt timestamp (status remains unchanged for audit)
        await db.updateBenefitProgram(input.id, {
          deletedAt: new Date(),
          deletedBy: ctx.user.id,
        });

        await db.logAdminAction({
          action: "soft_delete_program",
          targetUserId: null,
          details: { 
            programId: input.id, 
            programName: program.name,
            deletedAt: new Date().toISOString(),
          },
          justification: input.justification,
          performedBy: ctx.user.id,
        });

        return { success: true };
      }),

    // Restore soft-deleted program
    restore: adminProcedure
      .input(
        z.object({
          id: z.number(),
          justification: z.string().min(10),
        })
      )
      .mutation(async ({ input, ctx }) => {
        const program = await db.getBenefitProgramById(input.id);
        if (!program) {
          throw new TRPCError({ code: "NOT_FOUND", message: "Program not found" });
        }

        // Check if actually deleted
        if (!program.deletedAt) {
          throw new TRPCError({ code: "BAD_REQUEST", message: "Program is not deleted" });
        }

        // Restore by clearing deletedAt timestamp
        await db.updateBenefitProgram(input.id, {
          deletedAt: null,
          deletedBy: null,
        });

        await db.logAdminAction({
          action: "restore_program",
          targetUserId: null,
          details: { 
            programId: input.id, 
            programName: program.name,
            restoredAt: new Date().toISOString(),
          },
          justification: input.justification,
          performedBy: ctx.user.id,
        });

        return { success: true };
      }),

    // List deleted programs (for admin recovery) - filters by deletedAt not null
    listDeleted: adminProcedure.query(async () => {
      const allPrograms = await db.getAllBenefitPrograms();
      return allPrograms.filter((p: { deletedAt: Date | null }) => p.deletedAt !== null);
    }),
  }),

  // ===== MCC Rules =====
  mccRules: router({
    getByProgramId: adminProcedure.input(z.object({ programId: z.number() })).query(async ({ input }) => {
      return await db.getMccRulesByProgramId(input.programId);
    }),

    add: adminProcedure
      .input(
        z.object({
          programId: z.number(),
          mccCode: z.string().min(1),
          mccDescription: z.string().optional(),
          justification: z.string().min(1),
        })
      )
      .mutation(async ({ input, ctx }) => {
        await db.addMccRule({
          programId: input.programId,
          mccCode: input.mccCode,
          mccDescription: input.mccDescription,
          createdBy: ctx.user.id,
        });

        // Log audit trail
        await db.logMccRuleAudit({
          programId: input.programId,
          action: "add_mcc",
          mccCode: input.mccCode,
          justification: input.justification,
          performedBy: ctx.user.id,
        });

        return { success: true };
      }),

    remove: adminProcedure
      .input(
        z.object({
          programId: z.number(),
          mccCode: z.string(),
          justification: z.string().min(1),
        })
      )
      .mutation(async ({ input, ctx }) => {
        await db.removeMccRule(input.programId, input.mccCode);

        // Log audit trail
        await db.logMccRuleAudit({
          programId: input.programId,
          action: "remove_mcc",
          mccCode: input.mccCode,
          justification: input.justification,
          performedBy: ctx.user.id,
        });

        return { success: true };
      }),

    getAudit: adminProcedure.input(z.object({ programId: z.number() })).query(async ({ input }) => {
      return await db.getMccRuleAuditByProgramId(input.programId);
      }),
  }),

  // ===== Feature Flags =====
  featureFlags: router({
    list: adminProcedure.query(async () => {
      return await db.getAllFeatureFlags();
    }),

    create: adminProcedure
      .input(
        z.object({
          name: z.string().min(1),
          description: z.string().optional(),
          enabled: z.boolean().default(false),
          environment: z.enum(["all", "production", "staging", "development"]).default("all"),
        })
      )
      .mutation(async ({ input, ctx }) => {
        await db.createFeatureFlag({
          ...input,
          updatedBy: ctx.user.id,
        });
        return { success: true };
      }),

    toggle: adminProcedure
      .input(
        z.object({
          id: z.number(),
          enabled: z.boolean(),
        })
      )
      .mutation(async ({ input, ctx }) => {
        await db.updateFeatureFlag(input.id, {
          enabled: input.enabled,
          updatedBy: ctx.user.id,
        });
        return { success: true };
      }),

    update: adminProcedure
      .input(
        z.object({
          id: z.number(),
          name: z.string().optional(),
          description: z.string().optional(),
          environment: z.enum(["all", "production", "staging", "development"]).optional(),
        })
      )
      .mutation(async ({ input, ctx }) => {
        const { id, ...updates } = input;
        await db.updateFeatureFlag(id, {
          ...updates,
          updatedBy: ctx.user.id,
        });
        return { success: true };
      }),

    batchToggle: adminProcedure
      .input(
        z.object({
          flagIds: z.array(z.number()),
          enabled: z.boolean(),
          justification: z.string().min(10),
        })
      )
      .mutation(async ({ input, ctx }) => {
        for (const flagId of input.flagIds) {
          await db.toggleFeatureFlag(flagId, input.enabled);
        }

        await db.logAdminAction({
          action: "batch_toggle_flags",
          targetUserId: null,
          details: { flagIds: input.flagIds, enabled: input.enabled },
          justification: input.justification,
          performedBy: ctx.user.id,
        });

        // Send email notification
        await sendAdminNotification(
          NotificationTemplates.batchFlagToggle({
            count: input.flagIds.length,
            action: input.enabled ? "enabled" : "disabled",
            performedBy: ctx.user.name || ctx.user.email || "Unknown Admin",
          })
        );

        return { success: true, count: input.flagIds.length };
      }),
  }),

  // ===== User Management =====
  users: router({
    list: adminProcedure.query(async () => {
      return await db.getAllUsers();
    }),

    updateRole: adminProcedure
      .input(
        z.object({
          userId: z.number(),
          role: z.enum(["admin", "user"]),
          justification: z.string().min(1),
        })
      )
      .mutation(async ({ input, ctx }) => {
        // Get the target user's current role
        const targetUser = (await db.getAllUsers()).find((u) => u.id === input.userId);
        if (!targetUser) {
          throw new TRPCError({ code: "NOT_FOUND", message: "User not found" });
        }

        // Update the role
        await db.updateUserRole(input.userId, input.role);

        // Log the action
        await db.logAdminAction({
          action: "role_change",
          targetUserId: input.userId,
          details: {
            oldRole: targetUser.role,
            newRole: input.role,
            targetUserName: targetUser.name,
            targetUserEmail: targetUser.email,
          },
          justification: input.justification,
          performedBy: ctx.user.id,
        });

        // Send email notification
        await sendAdminNotification(
          NotificationTemplates.roleChange({
            userName: targetUser.name || targetUser.email || "Unknown User",
            oldRole: targetUser.role,
            newRole: input.role,
            performedBy: ctx.user.name || ctx.user.email || "Unknown Admin",
          })
        );

        return { success: true };
      }),
  }),

  // ===== Admin Action Audit =====
  adminAudit: router({
    list: adminProcedure
      .input(
        z
          .object({
            startDate: z.string().optional(),
            endDate: z.string().optional(),
            actionType: z.string().optional(),
          })
          .optional()
      )
      .query(async ({ input }) => {
        const filters = input
          ? {
              startDate: input.startDate ? new Date(input.startDate) : undefined,
              endDate: input.endDate ? new Date(input.endDate) : undefined,
              actionType: input.actionType || undefined,
            }
          : undefined;
        return await db.getAllAdminActions(filters);
      }),

    getByUser: adminProcedure.input(z.object({ userId: z.number() })).query(async ({ input }) => {
      return await db.getAdminActionsByUser(input.userId);
      }),
  }),

  // ===== Disbursement Schedules =====
  disbursements: router({
    list: adminProcedure.query(async () => {
      return await db.getAllDisbursementSchedules();
    }),

    getById: adminProcedure.input(z.object({ id: z.number() })).query(async ({ input }) => {
      const schedule = await db.getDisbursementScheduleById(input.id);
      if (!schedule) {
        throw new TRPCError({ code: "NOT_FOUND", message: "Schedule not found" });
      }
      return schedule;
    }),

    create: adminProcedure
      .input(
        z.object({
          programId: z.number(),
          scheduledDate: z.date(),
          amount: z.number().positive(),
          beneficiaryCount: z.number().positive().optional(),
          metadata: z.any().optional(),
        })
      )
      .mutation(async ({ input, ctx }) => {
        await db.createDisbursementSchedule({
          ...input,
          createdBy: ctx.user.id,
        });
        return { success: true };
      }),

    updateStatus: adminProcedure
      .input(
        z.object({
          id: z.number(),
          status: z.enum(["pending", "processing", "completed", "failed"]),
        })
      )
      .mutation(async ({ input }) => {
        await db.updateDisbursementSchedule(input.id, {
          status: input.status,
        });
        return { success: true };
      }),
  }),

  // ===== Program Templates =====
  programTemplates: router({
    list: adminProcedure.query(async () => {
      return db.getAllProgramTemplates();
    }),

    getById: adminProcedure
      .input(z.object({ id: z.number() }))
      .query(async ({ input }) => {
        return db.getProgramTemplateById(input.id);
      }),

    update: adminProcedure
      .input(
        z.object({
          id: z.number(),
          name: z.string().min(1).optional(),
          description: z.string().optional(),
        })
      )
      .mutation(async ({ input }) => {
        const updateData: any = {};
        if (input.name) updateData.name = input.name;
        if (input.description !== undefined) updateData.description = input.description || null;

        await db.updateProgramTemplate(input.id, updateData);
        return { success: true };
      }),

    delete: adminProcedure
      .input(z.object({ id: z.number() }))
      .mutation(async ({ input }) => {
        await db.deleteProgramTemplate(input.id);
        return { success: true };
      }),

    create: adminProcedure
      .input(
        z.object({
          programId: z.number(),
          templateName: z.string().min(1),
          templateDescription: z.string().optional(),
        })
      )
      .mutation(async ({ ctx, input }) => {
        // Get the program and its MCC rules
        const program = await db.getBenefitProgramById(input.programId);
        if (!program) {
          throw new Error("Program not found");
        }

        const mccRulesList = await db.getMccRulesByProgramId(input.programId);

        // Create the template
        const templateId = await db.createProgramTemplate({
          name: input.templateName,
          description: input.templateDescription || program.description,
          accountType: program.accountType,
          mccRules: mccRulesList.map((rule) => ({
            mccCode: rule.mccCode,
            mccDescription: rule.mccDescription,
          })),
          createdBy: ctx.user.id,
        });

        return { success: true, templateId };
      }),

    createProgramFromTemplate: adminProcedure
      .input(
        z.object({
          templateId: z.number(),
          programName: z.string().min(1),
          programDescription: z.string().optional(),
        })
      )
      .mutation(async ({ ctx, input }) => {
        const template = await db.getProgramTemplateById(input.templateId);
        if (!template) {
          throw new Error("Template not found");
        }

        // Create the program
        const newProgram = await db.createBenefitProgram({
          name: input.programName,
          description: input.programDescription || template.description || "",
          accountType: template.accountType,
          status: "pending_review",
          createdBy: ctx.user.id,
        });

        // Create MCC rules from template
        const mccRulesArray = template.mccRules as Array<{ mccCode: string; mccDescription: string | null }>;
        for (const rule of mccRulesArray) {
          await db.addMccRule({
            programId: newProgram.id,
            mccCode: rule.mccCode,
            mccDescription: rule.mccDescription,
            createdBy: ctx.user.id,
          });
        }

        return { success: true, programId: newProgram.id };
      }),
  }),

  // ===== Scheduled Reports =====
  scheduledReports: router({
    list: adminProcedure.query(async () => {
      return db.getAllScheduledReports();
    }),

    create: adminProcedure
      .input(
        z.object({
          name: z.string().min(1),
          description: z.string().optional(),
          reportType: z.enum(["weekly", "monthly"]),
          recipients: z.array(z.string().email()),
        })
      )
      .mutation(async ({ ctx, input }) => {
        const { calculateNextRunTime } = await import("./reportService");
        const nextRunAt = calculateNextRunTime(input.reportType);

        const reportId = await db.createScheduledReport({
          name: input.name,
          description: input.description || null,
          reportType: input.reportType,
          recipients: JSON.stringify(input.recipients),
          isActive: 1,
          nextRunAt,
          createdBy: ctx.user.id,
        });

        return { success: true, reportId };
      }),

    update: adminProcedure
      .input(
        z.object({
          id: z.number(),
          name: z.string().min(1).optional(),
          description: z.string().optional(),
          recipients: z.array(z.string().email()).optional(),
          isActive: z.boolean().optional(),
        })
      )
      .mutation(async ({ input }) => {
        const updateData: any = {};
        if (input.name) updateData.name = input.name;
        if (input.description !== undefined) updateData.description = input.description || null;
        if (input.recipients) updateData.recipients = JSON.stringify(input.recipients);
        if (input.isActive !== undefined) updateData.isActive = input.isActive ? 1 : 0;

        await db.updateScheduledReport(input.id, updateData);
        return { success: true };
      }),

    delete: adminProcedure
      .input(z.object({ id: z.number() }))
      .mutation(async ({ input }) => {
        await db.deleteScheduledReport(input.id);
        return { success: true };
      }),

    generateNow: adminProcedure
      .input(z.object({ id: z.number() }))
      .mutation(async ({ input }) => {
        const report = await db.getScheduledReportById(input.id);
        if (!report) {
          throw new Error("Report not found");
        }

        const { generateReport, sendReportEmail } = await import("./reportService");
        
        try {
          const reportData = await generateReport(report.reportType);
          const recipients = JSON.parse(report.recipients) as string[];
          const emailSent = await sendReportEmail(recipients, reportData);

          await db.createReportHistory({
            reportId: report.id,
            reportData: JSON.stringify(reportData),
            status: emailSent ? "success" : "failed",
            errorMessage: emailSent ? null : "Failed to send email",
          });

          await db.updateScheduledReport(report.id, {
            lastRunAt: new Date(),
          });

          return { success: true, reportData };
        } catch (error: any) {
          await db.createReportHistory({
            reportId: report.id,
            reportData: "{}",
            status: "failed",
            errorMessage: error.message,
          });
          throw error;
        }
      }),

    getHistory: adminProcedure
      .input(z.object({ reportId: z.number() }))
      .query(async ({ input }) => {
        return db.getReportHistoryByReportId(input.reportId);
      }),
  }),

  // ===== Notification Preferences =====
  notificationPreferences: router({
    get: protectedProcedure.query(async ({ ctx }) => {
      const prefs = await db.getNotificationPreferencesByUserId(ctx.user.id);
      return prefs || {
        userId: ctx.user.id,
        notifyOnRoleChange: true,
        notifyOnBatchOperations: true,
        notifyOnProgramChanges: true,
        notifyOnMccRuleChanges: false,
        emailAddress: null,
      };
    }),

    update: protectedProcedure
      .input(
        z.object({
          notifyOnRoleChange: z.boolean(),
          notifyOnBatchOperations: z.boolean(),
          notifyOnProgramChanges: z.boolean(),
          notifyOnMccRuleChanges: z.boolean(),
          emailAddress: z.string().email().optional(),
        })
      )
      .mutation(async ({ ctx, input }) => {
        await db.createNotificationPreference({
          userId: ctx.user.id,
          ...input,
          emailAddress: input.emailAddress || null,
        });
        return { success: true };
      }),
  }),

  // ===== MCC Analytics =====
  mccAnalytics: router({
    getUsageStats: adminProcedure.query(async () => {
      const allRules = await db.getAllMccRules();
      const allPrograms = await db.getAllBenefitPrograms();
      
      // Count MCC usage across programs
      const mccUsageMap = new Map<string, { code: string; description: string; count: number; programs: string[] }>();
      
      for (const rule of allRules) {
        const program = allPrograms.find(p => p.id === rule.programId);
        if (!program) continue;
        
        const existing = mccUsageMap.get(rule.mccCode);
        if (existing) {
          existing.count++;
          existing.programs.push(program.name);
        } else {
          mccUsageMap.set(rule.mccCode, {
            code: rule.mccCode,
            description: rule.mccDescription || "Unknown",
            count: 1,
            programs: [program.name],
          });
        }
      }
      
      // Convert to array and sort by usage count
      const usageStats = Array.from(mccUsageMap.values())
        .sort((a, b) => b.count - a.count)
        .slice(0, 20); // Top 20
      
      return {
        totalMccCodes: mccUsageMap.size,
        totalRules: allRules.length,
        totalPrograms: allPrograms.length,
        topMccCodes: usageStats,
      };
    }),
  }),

  // ===== MCC Database =====
  mccDatabase: router({
    list: adminProcedure
      .input(
        z
          .object({
            query: z.string().optional(),
            page: z.number().default(1),
            pageSize: z.number().default(50),
          })
          .optional()
      )
      .query(async ({ input }) => {
        const query = input?.query || "";
        const page = input?.page || 1;
        const pageSize = input?.pageSize || 50;
        const offset = (page - 1) * pageSize;

        const allResults = await db.searchMccDatabase(query);
        const total = allResults.length;
        const results = allResults.slice(offset, offset + pageSize);

        return {
          results,
          total,
          page,
          pageSize,
          totalPages: Math.ceil(total / pageSize),
        };
      }),

    update: adminProcedure
      .input(
        z.object({
          id: z.number(),
          mccCode: z.string(),
          description: z.string(),
          category: z.string().optional(),
        })
      )
      .mutation(async ({ input, ctx }) => {
        await db.updateMccEntry(input.id, {
          mccCode: input.mccCode,
          description: input.description,
          category: input.category,
        });

        await db.logAdminAction({
          action: "update_mcc",
          targetUserId: null,
          details: { mccId: input.id, mccCode: input.mccCode },
          justification: `Updated MCC ${input.mccCode}`,
          performedBy: ctx.user.id,
        });

        return { success: true };
      }),

    delete: adminProcedure.input(z.object({ id: z.number() })).mutation(async ({ input, ctx }) => {
      await db.deleteMccEntry(input.id);

      await db.logAdminAction({
        action: "delete_mcc",
        targetUserId: null,
        details: { mccId: input.id },
        justification: `Deleted MCC entry ${input.id}`,
        performedBy: ctx.user.id,
      });

      return { success: true };
    }),

    bulkImport: adminProcedure
      .input(
        z.object({
          entries: z.array(
            z.object({
              mccCode: z.string(),
              description: z.string(),
              category: z.string().optional(),
            })
          ),
        })
      )
      .mutation(async ({ input, ctx }) => {
        try {
          await db.seedMccDatabase(input.entries);
          
          // Log the bulk import action
          await db.logAdminAction({
            action: "bulk_mcc_import",
            targetUserId: null,
            details: {
              count: input.entries.length,
              performedBy: ctx.user.name || ctx.user.email,
            },
            justification: `Bulk imported ${input.entries.length} MCC entries`,
            performedBy: ctx.user.id,
          });

          return { success: true, count: input.entries.length };
        } catch (error) {
          throw new TRPCError({
            code: "INTERNAL_SERVER_ERROR",
            message: `Failed to import MCCs: ${error instanceof Error ? error.message : "Unknown error"}`,
          });
        }
      }),
  }),

  // ===== Beneficiary Management =====
  beneficiaries: router({
    // DB-backed pagination - queries are executed at the database level
    list: adminProcedure
      .input(
        z.object({
          page: z.number().min(1).default(1),
          pageSize: z.number().min(1).max(100).default(20),
          status: z.enum(["all", "pending", "approved", "rejected", "suspended"]).default("all"),
          sortBy: z.enum(["createdAt", "firstName", "lastName", "enrollmentStatus"]).default("createdAt"),
          sortOrder: z.enum(["asc", "desc"]).default("desc"),
        }).optional()
      )
      .query(async ({ input }) => {
        // Use DB-backed pagination - filtering, sorting, and pagination happen at database level
        return beneficiaryDb.getBeneficiariesPaginated({
          page: input?.page || 1,
          pageSize: input?.pageSize || 20,
          status: input?.status as "all" | "pending" | "approved" | "rejected" | "suspended" | undefined,
          sortBy: input?.sortBy as "createdAt" | "firstName" | "lastName" | "enrollmentStatus" | undefined,
          sortOrder: input?.sortOrder as "asc" | "desc" | undefined,
        });
      }),

    // Cursor-based pagination for large datasets (more efficient for deep pagination)
    listByCursor: adminProcedure
      .input(
        z.object({
          cursor: z.number().optional(),
          limit: z.number().min(1).max(100).default(20),
          status: z.enum(["all", "pending", "approved", "rejected", "suspended"]).default("all"),
        })
      )
      .query(async ({ input }) => {
        return beneficiaryDb.getBeneficiariesByCursor(
          input.cursor,
          input.limit,
          input.status as "all" | "pending" | "approved" | "rejected" | "suspended"
        );
      }),

    getById: adminProcedure
      .input(z.object({ id: z.number() }))
      .query(async ({ input }) => {
        return beneficiaryDb.getBeneficiaryById(input.id);
      }),

    // DB-backed search with pagination
    search: adminProcedure
      .input(z.object({ 
        query: z.string(),
        page: z.number().min(1).default(1),
        pageSize: z.number().min(1).max(100).default(20),
      }))
      .query(async ({ input }) => {
        // Use DB-backed search pagination
        return beneficiaryDb.searchBeneficiariesPaginated(input.query, {
          page: input.page,
          pageSize: input.pageSize,
        });
      }),

    create: adminProcedure
      .input(
        z.object({
          firstName: z.string().min(1),
          lastName: z.string().min(1),
          dateOfBirth: z.date(),
          nationalId: z.string().optional(),
          phoneNumber: z.string().optional(),
          email: z.string().email().optional(),
          address: z.string().optional(),
          city: z.string().optional(),
          state: z.string().optional(),
          postalCode: z.string().optional(),
        })
      )
      .mutation(async ({ input, ctx }) => {
        const beneficiaryId = await beneficiaryDb.createBeneficiary(input);
        
        await db.logAdminAction({
          action: "create_beneficiary",
          targetUserId: null,
          details: { beneficiaryId, name: `${input.firstName} ${input.lastName}` },
          justification: "Created new beneficiary",
          performedBy: ctx.user.id,
        });
        
        return { beneficiaryId };
      }),

    update: adminProcedure
      .input(
        z.object({
          id: z.number(),
          firstName: z.string().min(1).optional(),
          lastName: z.string().min(1).optional(),
          phoneNumber: z.string().optional(),
          email: z.string().email().optional(),
          address: z.string().optional(),
          city: z.string().optional(),
          state: z.string().optional(),
          postalCode: z.string().optional(),
        })
      )
      .mutation(async ({ input, ctx }) => {
        const { id, ...updateData } = input;
        await beneficiaryDb.updateBeneficiary(id, updateData);
        
        await db.logAdminAction({
          action: "update_beneficiary",
          targetUserId: null,
          details: { beneficiaryId: id },
          justification: "Updated beneficiary information",
          performedBy: ctx.user.id,
        });
        
        return { success: true };
      }),

    approve: adminProcedure
      .input(z.object({ id: z.number() }))
      .mutation(async ({ input, ctx }) => {
        await beneficiaryDb.approveBeneficiary(input.id, ctx.user.id);
        
        await db.logAdminAction({
          action: "approve_beneficiary",
          targetUserId: null,
          details: { beneficiaryId: input.id },
          justification: "Approved beneficiary enrollment",
          performedBy: ctx.user.id,
        });
        
        return { success: true };
      }),

    reject: adminProcedure
      .input(z.object({ id: z.number() }))
      .mutation(async ({ input, ctx }) => {
        await beneficiaryDb.rejectBeneficiary(input.id);
        
        await db.logAdminAction({
          action: "reject_beneficiary",
          targetUserId: null,
          details: { beneficiaryId: input.id },
          justification: "Rejected beneficiary enrollment",
          performedBy: ctx.user.id,
        });
        
        return { success: true };
      }),

    suspend: adminProcedure
      .input(z.object({ id: z.number() }))
      .mutation(async ({ input, ctx }) => {
        await beneficiaryDb.suspendBeneficiary(input.id);
        await db.logAdminAction({ action: "suspend_beneficiary", targetUserId: null, details: { beneficiaryId: input.id }, justification: "Suspended beneficiary enrollment", performedBy: ctx.user.id });
        return { success: true };
      }),

    delete: adminProcedure
      .input(z.object({ id: z.number() }))
      .mutation(async ({ input, ctx }) => {
        await beneficiaryDb.deleteBeneficiary(input.id);
        
        await db.logAdminAction({
          action: "delete_beneficiary",
          targetUserId: null,
          details: { beneficiaryId: input.id },
          justification: "Deleted beneficiary",
          performedBy: ctx.user.id,
        });
        
        return { success: true };
      }),

    getStats: adminProcedure.query(async () => {
      return beneficiaryDb.getBeneficiaryStats();
    }),

    // Enrollments
    getEnrollments: adminProcedure
      .input(z.object({ beneficiaryId: z.number() }))
      .query(async ({ input }) => {
        return beneficiaryDb.getBeneficiaryEnrollments(input.beneficiaryId);
      }),

    createEnrollment: adminProcedure
      .input(
        z.object({
          beneficiaryId: z.number(),
          programId: z.number(),
          monthlyAllocation: z.number(),
        })
      )
      .mutation(async ({ input, ctx }) => {
        const enrollmentId = await beneficiaryDb.createEnrollment(input);
        
        await db.logAdminAction({
          action: "create_enrollment",
          targetUserId: null,
          details: { enrollmentId, beneficiaryId: input.beneficiaryId, programId: input.programId },
          justification: "Enrolled beneficiary in program",
          performedBy: ctx.user.id,
        });
        
        return { enrollmentId };
      }),

    // KYC Documents
    getDocuments: adminProcedure
      .input(z.object({ beneficiaryId: z.number() }))
      .query(async ({ input }) => {
        return beneficiaryDb.getBeneficiaryDocuments(input.beneficiaryId);
      }),

    uploadDocument: adminProcedure
      .input(
        z.object({
          beneficiaryId: z.number(),
          documentType: z.enum(["national_id", "passport", "drivers_license", "birth_certificate", "proof_of_address", "photo", "other"]),
          documentUrl: z.string(),
          fileKey: z.string(),
          fileName: z.string(),
          mimeType: z.string().optional(),
          fileSize: z.number().optional(),
        })
      )
      .mutation(async ({ input }) => {
        const documentId = await beneficiaryDb.createKycDocument(input);
        return { documentId };
      }),

    verifyDocument: adminProcedure
      .input(
        z.object({
          id: z.number(),
          status: z.enum(["verified", "rejected"]),
          reason: z.string().optional(),
        })
      )
      .mutation(async ({ input, ctx }) => {
        await beneficiaryDb.verifyDocument(input.id, ctx.user.id, input.status, input.reason);
        
        await db.logAdminAction({
          action: "verify_document",
          targetUserId: null,
          details: { documentId: input.id, status: input.status },
          justification: `Document ${input.status}`,
          performedBy: ctx.user.id,
        });
        
        return { success: true };
      }),

    // Benefit Cards
    getCards: adminProcedure
      .input(z.object({ beneficiaryId: z.number() }))
      .query(async ({ input }) => {
        return beneficiaryDb.getBeneficiaryCards(input.beneficiaryId);
      }),

    issueCard: adminProcedure
      .input(
        z.object({
          beneficiaryId: z.number(),
          cardNumber: z.string(),
          cardType: z.enum(["physical", "virtual"]),
          expiresAt: z.date(),
        })
      )
      .mutation(async ({ input, ctx }) => {
        const cardId = await beneficiaryDb.createBenefitCard(input);
        
        await db.logAdminAction({
          action: "issue_card",
          targetUserId: null,
          details: { cardId, beneficiaryId: input.beneficiaryId, cardType: input.cardType },
          justification: "Issued benefit card",
          performedBy: ctx.user.id,
        });
        
        return { cardId };
      }),

    updateCardStatus: adminProcedure
      .input(
        z.object({
          id: z.number(),
          status: z.enum(["pending", "active", "blocked", "expired", "lost", "stolen"]),
          reason: z.string().optional(),
        })
      )
      .mutation(async ({ input, ctx }) => {
        await beneficiaryDb.updateCardStatus(input.id, input.status, input.reason);
        
        await db.logAdminAction({
          action: "update_card_status",
          targetUserId: null,
          details: { cardId: input.id, status: input.status },
          justification: `Card status changed to ${input.status}`,
          performedBy: ctx.user.id,
        });
        
        return { success: true };
      }),
  }),

  // ===== Transaction Monitoring =====
  transactions: router({    list: adminProcedure
      .input(z.object({ limit: z.number().optional() }).optional())
      .query(async ({ input }) => {
        return transactionDb.getAllTransactions(input?.limit);
      }),

    getById: adminProcedure
      .input(z.object({ id: z.number() }))
      .query(async ({ input }) => {
        return transactionDb.getTransactionById(input.id);
      }),

    getByBeneficiary: adminProcedure
      .input(z.object({ beneficiaryId: z.number(), limit: z.number().optional() }))
      .query(async ({ input }) => {
        return transactionDb.getBeneficiaryTransactions(input.beneficiaryId, input.limit);
      }),

    getByProgram: adminProcedure
      .input(z.object({ programId: z.number(), limit: z.number().optional() }))
      .query(async ({ input }) => {
        return transactionDb.getProgramTransactions(input.programId, input.limit);
      }),

    getNonCompliant: adminProcedure
      .input(z.object({ limit: z.number().optional() }).optional())
      .query(async ({ input }) => {
        return transactionDb.getNonCompliantTransactions(input?.limit);
      }),

    getStats: adminProcedure
      .input(z.object({ programId: z.number().optional() }).optional())
      .query(async ({ input }) => {
        return transactionDb.getTransactionStats(input?.programId);
      }),

    getMccUsage: adminProcedure
      .input(z.object({ programId: z.number().optional(), limit: z.number().optional() }).optional())
      .query(async ({ input }) => {
        return transactionDb.getMccUsageStats(input?.programId, input?.limit);
      }),

    getDailyVolume: adminProcedure
      .input(z.object({ days: z.number().optional() }).optional())
      .query(async ({ input }) => {
        return transactionDb.getDailyTransactionVolume(input?.days);
      }),
  }),

  // ===== Fraud Alerts =====
  fraudAlerts: router({
    list: adminProcedure
      .input(z.object({ limit: z.number().optional() }).optional())
      .query(async ({ input }) => {
        return transactionDb.getAllFraudAlerts(input?.limit);
      }),

    getOpen: adminProcedure.query(async () => {
      return transactionDb.getOpenFraudAlerts();
    }),

    getByBeneficiary: adminProcedure
      .input(z.object({ beneficiaryId: z.number() }))
      .query(async ({ input }) => {
        return transactionDb.getBeneficiaryFraudAlerts(input.beneficiaryId);
      }),

    updateStatus: adminProcedure
      .input(
        z.object({
          id: z.number(),
          status: z.enum(["open", "investigating", "resolved", "false_positive"]),
          assignedTo: z.number().optional(),
          resolution: z.string().optional(),
        })
      )
      .mutation(async ({ input, ctx }) => {
        await transactionDb.updateFraudAlertStatus(
          input.id,
          input.status,
          input.assignedTo,
          ctx.user.id,
          input.resolution
        );
        
        await db.logAdminAction({
          action: "update_fraud_alert",
          targetUserId: null,
          details: { alertId: input.id, status: input.status },
          justification: `Fraud alert status changed to ${input.status}`,
          performedBy: ctx.user.id,
        });
        
        return { success: true };
      }),

    getStats: adminProcedure.query(async () => {
      return transactionDb.getFraudAlertStats();
    }),

    getBySeverity: adminProcedure.query(async () => {
      return transactionDb.getAlertsBySeverity();
    }),
  }),

  // ===== Data Export =====
  exports: router({
    beneficiaries: adminProcedure
      .input(z.object({ format: z.enum(["csv", "excel"]) }).optional())
      .mutation(async () => {
        const data = await beneficiaryDb.getAllBeneficiaries();
        const csv = exportUtils.generateCSV({
          filename: "beneficiaries",
          columns: exportUtils.BENEFICIARY_EXPORT_COLUMNS,
          data,
        });
        return { csv, filename: exportUtils.generateFilename("beneficiaries", "csv") };
      }),

    transactions: adminProcedure
      .input(z.object({ format: z.enum(["csv", "excel"]) }).optional())
      .mutation(async () => {
        const data = await transactionDb.getAllTransactions(1000);
        const csv = exportUtils.generateCSV({
          filename: "transactions",
          columns: exportUtils.TRANSACTION_EXPORT_COLUMNS,
          data,
        });
        return { csv, filename: exportUtils.generateFilename("transactions", "csv") };
      }),

    auditLogs: adminProcedure
      .input(z.object({ format: z.enum(["csv", "excel"]) }).optional())
      .mutation(async () => {
        const data = await db.getAllAdminActions();
        const csv = exportUtils.generateCSV({
          filename: "audit_logs",
          columns: exportUtils.AUDIT_LOG_EXPORT_COLUMNS,
          data,
        });
        return { csv, filename: exportUtils.generateFilename("audit_logs", "csv") };
      }),

    fraudAlerts: adminProcedure
      .input(z.object({ format: z.enum(["csv", "excel"]) }).optional())
      .mutation(async () => {
        const data = await transactionDb.getAllFraudAlerts(1000);
        const csv = exportUtils.generateCSV({
          filename: "fraud_alerts",
          columns: exportUtils.FRAUD_ALERT_EXPORT_COLUMNS,
          data,
        });
        return { csv, filename: exportUtils.generateFilename("fraud_alerts", "csv") };
      }),
  }),

  // ===== Transaction Simulation =====
  transactionSimulator: router({
    generateTransactions: adminProcedure
      .input(
        z.object({
          beneficiaryId: z.number(),
          count: z.number().min(1).max(100),
          pattern: z.enum(["compliant", "mcc_violation", "fraud_pattern", "mixed"]),
          amountRange: z.object({
            min: z.number(),
            max: z.number(),
          }).optional(),
        })
      )
      .mutation(async ({ input, ctx }) => {
        const transactions = await transactionDb.generateSimulatedTransactions(
          input.beneficiaryId,
          input.count,
          input.pattern,
          input.amountRange
        );
        
        await db.logAdminAction({
          action: "simulate_transactions",
          targetUserId: null,
          details: { beneficiaryId: input.beneficiaryId, count: input.count, pattern: input.pattern },
          justification: `Generated ${input.count} ${input.pattern} test transactions`,
          performedBy: ctx.user.id,
        });
        
        return { transactions, count: transactions.length };
      }),
  }),

  // ===== Analytics =====
  analytics: router({
    getDashboardMetrics: adminProcedure.query(async () => {
      const [users, programs, flags, schedules, actions] = await Promise.all([
        db.getAllUsers(),
        db.getAllBenefitPrograms(),
        db.getAllFeatureFlags(),
        db.getAllDisbursementSchedules(),
        db.getAllAdminActions(),
      ]);

      const totalUsers = users.length;
      const adminCount = users.filter((u: any) => u.role === "admin").length;
      const totalPrograms = programs.length;
      const totalFlags = flags.length;
      const upcomingDisbursements = schedules.filter(
        (s: any) => s.status === "pending" && new Date(s.scheduledDate) > new Date()
      ).length;
      const recentActions = actions.slice(0, 5);

      return {
        totalUsers,
        adminCount,
        totalPrograms,
        totalFlags,
        upcomingDisbursements,
        recentActions,
      };
    }),
  }),

  // Middleware monitoring
  middleware: middlewareMonitoringRouter,

  // Event replay system
  eventReplay: eventReplayRouter,

  // Multi-tenancy support
  multiTenancy: multiTenancyRouter,
  workflow: workflowRouter,
  temporalUI: temporalUIRouter,
  workflowControl: workflowControlRouter,
  audit: auditRouter,
  stakeholderOnboarding: stakeholderOnboardingRouter,
  worldClass: worldClassRouter,
});

export type AppRouter = typeof appRouter;
