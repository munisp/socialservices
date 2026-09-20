import { router, protectedProcedure } from "../_core/trpc";
import { z } from "zod";
import { TRPCError } from "@trpc/server";

const GO_ORCHESTRATOR_URL = process.env.GO_ORCHESTRATOR_URL || "http://localhost:8090";
const PMT_SERVICE_URL = process.env.PMT_SERVICE_URL || "http://localhost:8091";

async function callGoService(endpoint: string, method: string = "GET", body?: any) {
  const response = await fetch(`${GO_ORCHESTRATOR_URL}${endpoint}`, {
    method,
    headers: { "Content-Type": "application/json" },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    throw new TRPCError({
      code: "INTERNAL_SERVER_ERROR",
      message: `Go service error: ${response.statusText}`,
    });
  }
  return response.json();
}

async function callPMTService(endpoint: string, method: string = "GET", body?: any) {
  const response = await fetch(`${PMT_SERVICE_URL}${endpoint}`, {
    method,
    headers: { "Content-Type": "application/json" },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    throw new TRPCError({
      code: "INTERNAL_SERVER_ERROR",
      message: `PMT service error: ${response.statusText}`,
    });
  }
  return response.json();
}

export const offlineSyncRouter = router({
  getStatus: protectedProcedure.query(async () => {
    return callGoService("/api/offline/status");
  }),

  sync: protectedProcedure
    .input(
      z.object({
        deviceId: z.string(),
        changes: z.array(
          z.object({
            id: z.string(),
            type: z.enum(["beneficiary", "household", "survey"]),
            action: z.enum(["create", "update", "delete"]),
            data: z.any(),
            timestamp: z.number(),
          })
        ),
      })
    )
    .mutation(async ({ input }) => {
      return callGoService("/api/offline/sync", "POST", input);
    }),

  getChanges: protectedProcedure
    .input(
      z.object({
        deviceId: z.string(),
        lastSyncTimestamp: z.number(),
      })
    )
    .query(async ({ input }) => {
      return callGoService(
        `/api/offline/changes?deviceId=${input.deviceId}&since=${input.lastSyncTimestamp}`
      );
    }),

  registerDevice: protectedProcedure
    .input(
      z.object({
        deviceId: z.string(),
        deviceType: z.string(),
        userId: z.number(),
      })
    )
    .mutation(async ({ input }) => {
      return callGoService("/api/offline/devices/register", "POST", input);
    }),

  getConflicts: protectedProcedure
    .input(z.object({ deviceId: z.string() }))
    .query(async ({ input }) => {
      return callGoService(`/api/offline/conflicts?deviceId=${input.deviceId}`);
    }),

  resolveConflict: protectedProcedure
    .input(
      z.object({
        conflictId: z.string(),
        resolution: z.enum(["client_wins", "server_wins", "merge", "manual"]),
        mergedData: z.any().optional(),
      })
    )
    .mutation(async ({ input }) => {
      return callGoService("/api/offline/conflicts/resolve", "POST", input);
    }),
});

export const federationRouter = router({
  getProviders: protectedProcedure.query(async () => {
    return callGoService("/api/federation/providers");
  }),

  sendOTP: protectedProcedure
    .input(
      z.object({
        providerId: z.string(),
        nationalId: z.string(),
        phone: z.string().optional(),
      })
    )
    .mutation(async ({ input }) => {
      return callGoService("/api/federation/send-otp", "POST", input);
    }),

  verify: protectedProcedure
    .input(
      z.object({
        providerId: z.string(),
        nationalId: z.string(),
        otp: z.string().optional(),
        biometricData: z.string().optional(),
        consent: z.object({
          dataSharing: z.boolean(),
          biometricCapture: z.boolean(),
          termsAccepted: z.boolean(),
        }),
      })
    )
    .mutation(async ({ input }) => {
      return callGoService("/api/federation/verify", "POST", input);
    }),

  getHistory: protectedProcedure
    .input(z.object({ beneficiaryId: z.string() }))
    .query(async ({ input }) => {
      return callGoService(`/api/federation/history/${input.beneficiaryId}`);
    }),

  revokeConsent: protectedProcedure
    .input(
      z.object({
        beneficiaryId: z.string(),
        providerId: z.string(),
      })
    )
    .mutation(async ({ input }) => {
      return callGoService("/api/federation/consent/revoke", "POST", input);
    }),
});

export const pmtRouter = router({
  calculate: protectedProcedure
    .input(
      z.object({
        householdId: z.string().optional(),
        characteristics: z.object({
          householdSize: z.number(),
          headAge: z.number(),
          headGender: z.enum(["male", "female"]),
          headEducation: z.string(),
          headEmployment: z.string(),
          childrenUnder5: z.number(),
          childrenUnder18: z.number(),
          elderlyOver65: z.number(),
          disabledMembers: z.number(),
          housingType: z.string(),
          wallMaterial: z.string(),
          roofMaterial: z.string(),
          floorMaterial: z.string(),
          waterSource: z.string(),
          sanitationType: z.string(),
          cookingFuel: z.string(),
          electricityAccess: z.boolean(),
          numberOfRooms: z.number(),
          landOwnership: z.number(),
          livestockOwnership: z.number(),
          vehicleOwnership: z.boolean(),
          applianceOwnership: z.array(z.string()),
          region: z.string(),
          district: z.string(),
          urbanRural: z.enum(["urban", "rural"]),
          hasBankAccount: z.boolean(),
          receivesRemittances: z.boolean(),
          hasHealthInsurance: z.boolean(),
          childrenInSchool: z.number(),
          foodSecurityScore: z.number(),
        }),
      })
    )
    .mutation(async ({ input }) => {
      return callPMTService("/calculate", "POST", input);
    }),

  batchCalculate: protectedProcedure
    .input(
      z.object({
        households: z.array(
          z.object({
            householdId: z.string(),
            characteristics: z.any(),
          })
        ),
      })
    )
    .mutation(async ({ input }) => {
      return callPMTService("/batch-calculate", "POST", input);
    }),

  getScore: protectedProcedure
    .input(z.object({ householdId: z.string() }))
    .query(async ({ input }) => {
      return callPMTService(`/score/${input.householdId}`);
    }),

  getThresholds: protectedProcedure.query(async () => {
    return callPMTService("/thresholds");
  }),

  trainModel: protectedProcedure
    .input(
      z.object({
        modelType: z.enum(["ridge", "lasso", "random_forest", "gradient_boosting"]),
        trainingDataPath: z.string(),
      })
    )
    .mutation(async ({ input }) => {
      return callPMTService("/train", "POST", input);
    }),
});

export const interopRouter = router({
  getProfile: protectedProcedure
    .input(z.object({ beneficiaryId: z.string() }))
    .query(async ({ input }) => {
      return callGoService(`/api/interop/profile/${input.beneficiaryId}`);
    }),

  getConsents: protectedProcedure
    .input(z.object({ beneficiaryId: z.string() }))
    .query(async ({ input }) => {
      return callGoService(`/api/interop/consents/${input.beneficiaryId}`);
    }),

  grantConsent: protectedProcedure
    .input(
      z.object({
        beneficiaryId: z.string(),
        sector: z.enum(["health", "education", "tax", "labor"]),
        purpose: z.string(),
        expiresAt: z.string().optional(),
      })
    )
    .mutation(async ({ input }) => {
      return callGoService("/api/interop/consent/grant", "POST", input);
    }),

  revokeConsent: protectedProcedure
    .input(
      z.object({
        beneficiaryId: z.string(),
        sector: z.enum(["health", "education", "tax", "labor"]),
      })
    )
    .mutation(async ({ input }) => {
      return callGoService("/api/interop/consent/revoke", "POST", input);
    }),

  refreshProfile: protectedProcedure
    .input(z.object({ beneficiaryId: z.string() }))
    .mutation(async ({ input }) => {
      return callGoService(`/api/interop/profile/${input.beneficiaryId}/refresh`, "POST");
    }),

  getSectors: protectedProcedure.query(async () => {
    return callGoService("/api/interop/sectors");
  }),
});

export const loadTestRouter = router({
  getHistory: protectedProcedure.query(async () => {
    return callGoService("/api/loadtest/history");
  }),

  start: protectedProcedure
    .input(
      z.object({
        name: z.string(),
        scalePreset: z.enum(["small", "medium", "large", "xlarge", "country"]),
        testType: z.enum([
          "smoke",
          "load",
          "stress",
          "spike",
          "soak",
          "breakpoint",
          "scalability",
        ]),
        scenarios: z.array(z.string()),
        duration: z.number(),
        targetRPS: z.number(),
        sloConfig: z.object({
          maxLatencyP50: z.number(),
          maxLatencyP95: z.number(),
          maxLatencyP99: z.number(),
          minSuccessRate: z.number(),
          maxErrorRate: z.number(),
        }),
      })
    )
    .mutation(async ({ input }) => {
      return callGoService("/api/loadtest/start", "POST", input);
    }),

  getStatus: protectedProcedure
    .input(z.object({ testId: z.string() }))
    .query(async ({ input }) => {
      return callGoService(`/api/loadtest/${input.testId}/status`);
    }),

  getMetrics: protectedProcedure
    .input(z.object({ testId: z.string() }))
    .query(async ({ input }) => {
      return callGoService(`/api/loadtest/${input.testId}/metrics`);
    }),

  stop: protectedProcedure
    .input(z.object({ testId: z.string() }))
    .mutation(async ({ input }) => {
      return callGoService(`/api/loadtest/${input.testId}/stop`, "POST");
    }),

  getReport: protectedProcedure
    .input(z.object({ testId: z.string() }))
    .query(async ({ input }) => {
      return callGoService(`/api/loadtest/${input.testId}/report`);
    }),
});

// Journey Orchestration Router - 30 User Journeys
export const journeyRouter = router({
  // List all journeys
  list: protectedProcedure
    .input(z.object({ category: z.string().optional() }).optional())
    .query(async ({ input }) => {
      const params = input?.category ? `?category=${input.category}` : "";
      return callGoService(`/api/journeys/${params}`);
    }),

  // Get journey categories
  getCategories: protectedProcedure.query(async () => {
    return callGoService("/api/journeys/categories");
  }),

  // Get journey contracts (full mapping from UI to Temporal)
  getContracts: protectedProcedure.query(async () => {
    return callGoService("/api/journey-contracts");
  }),

  // Get specific journey definition
  getDefinition: protectedProcedure
    .input(z.object({ journeyKey: z.string() }))
    .query(async ({ input }) => {
      return callGoService(`/api/journeys/${input.journeyKey}`);
    }),

  // Get journey status
  getStatus: protectedProcedure
    .input(z.object({ journeyKey: z.string(), journeyRunId: z.string() }))
    .query(async ({ input }) => {
      return callGoService(`/api/journeys/${input.journeyKey}/status?runId=${input.journeyRunId}`);
    }),

  // Start enrollment journeys (1-5)
  startBeneficiaryEnrollment: protectedProcedure
    .input(z.object({
      nationalId: z.string(),
      firstName: z.string(),
      lastName: z.string(),
      dateOfBirth: z.string(),
      gender: z.enum(["male", "female", "other"]),
      phone: z.string().optional(),
      email: z.string().optional(),
      address: z.object({
        region: z.string(),
        district: z.string(),
        ward: z.string(),
        village: z.string().optional(),
        street: z.string().optional(),
      }),
      programId: z.string().optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/beneficiary-enrollment/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startKYCVerification: protectedProcedure
    .input(z.object({
      beneficiaryId: z.string(),
      providerId: z.string(),
      verificationMethod: z.enum(["otp", "biometric", "document"]),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/kyc-verification/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startHouseholdRegistration: protectedProcedure
    .input(z.object({
      headBeneficiaryId: z.string(),
      members: z.array(z.object({
        nationalId: z.string().optional(),
        firstName: z.string(),
        lastName: z.string(),
        relationship: z.string(),
        dateOfBirth: z.string(),
        gender: z.enum(["male", "female", "other"]),
      })),
      address: z.object({
        region: z.string(),
        district: z.string(),
        ward: z.string(),
        village: z.string().optional(),
      }),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/household-registration/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startProgramEnrollment: protectedProcedure
    .input(z.object({
      beneficiaryId: z.string(),
      programId: z.string(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/program-enrollment/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startCardIssuance: protectedProcedure
    .input(z.object({
      beneficiaryId: z.string(),
      cardType: z.enum(["physical", "virtual"]),
      deliveryAddress: z.object({
        region: z.string(),
        district: z.string(),
        ward: z.string(),
        street: z.string().optional(),
      }).optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/card-issuance/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  // Start payment journeys (6-10)
  startDisbursementSchedule: protectedProcedure
    .input(z.object({
      programId: z.string(),
      disbursementDate: z.string(),
      amount: z.number(),
      currency: z.string().default("TZS"),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/disbursement-schedule/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startDisbursementExecute: protectedProcedure
    .input(z.object({
      disbursementId: z.string(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/disbursement-execute/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startRetryDisbursement: protectedProcedure
    .input(z.object({
      disbursementId: z.string(),
      itemIds: z.array(z.string()).optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/retry-disbursement/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startReconciliation: protectedProcedure
    .input(z.object({
      disbursementId: z.string(),
      externalStatementPath: z.string().optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/reconciliation/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startDisputeResolution: protectedProcedure
    .input(z.object({
      reconciliationId: z.string(),
      discrepancyIds: z.array(z.string()),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/dispute-resolution/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  // Start grievance journeys (11-13)
  startGrievanceSubmission: protectedProcedure
    .input(z.object({
      beneficiaryId: z.string(),
      category: z.enum(["payment", "enrollment", "service", "other"]),
      description: z.string(),
      priority: z.enum(["low", "medium", "high", "critical"]).default("medium"),
      attachments: z.array(z.string()).optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/grievance-submission/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startGrievanceResolution: protectedProcedure
    .input(z.object({
      grievanceId: z.string(),
      resolution: z.string(),
      compensationAmount: z.number().optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/grievance-resolution/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startGrievanceEscalation: protectedProcedure
    .input(z.object({
      grievanceId: z.string(),
      escalationReason: z.string(),
      targetLevel: z.number(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/grievance-escalation/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  // Start lifecycle journeys (14-18)
  startProfileUpdate: protectedProcedure
    .input(z.object({
      beneficiaryId: z.string(),
      updates: z.record(z.string(), z.any()),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/profile-update/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startBeneficiarySuspension: protectedProcedure
    .input(z.object({
      beneficiaryId: z.string(),
      reason: z.string(),
      suspensionType: z.enum(["temporary", "permanent"]),
      endDate: z.string().optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/beneficiary-suspension/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startBeneficiaryReactivation: protectedProcedure
    .input(z.object({
      beneficiaryId: z.string(),
      reason: z.string(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/beneficiary-reactivation/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startBeneficiaryExit: protectedProcedure
    .input(z.object({
      beneficiaryId: z.string(),
      exitType: z.enum(["graduation", "voluntary", "death", "ineligible"]),
      reason: z.string(),
      finalPaymentAmount: z.number().optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/beneficiary-exit/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startDeathRegistration: protectedProcedure
    .input(z.object({
      beneficiaryId: z.string(),
      dateOfDeath: z.string(),
      deathCertificateNumber: z.string().optional(),
      survivorBeneficiaryId: z.string().optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/death-registration/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  // Start admin journeys (19-22)
  startApprovalDelegation: protectedProcedure
    .input(z.object({
      delegatorId: z.string(),
      delegateeId: z.string(),
      permissions: z.array(z.string()),
      startDate: z.string(),
      endDate: z.string(),
      reason: z.string(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/approval-delegation/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startBreakGlassAccess: protectedProcedure
    .input(z.object({
      resourceType: z.string(),
      resourceId: z.string(),
      reason: z.string(),
      durationMinutes: z.number(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/break-glass-access/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startBulkOperation: protectedProcedure
    .input(z.object({
      operationType: z.enum(["suspend", "reactivate", "enroll", "update", "export"]),
      entityType: z.enum(["beneficiary", "household", "program"]),
      entityIds: z.array(z.string()),
      parameters: z.record(z.string(), z.any()).optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/bulk-operation/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startOfflineSync: protectedProcedure
    .input(z.object({
      deviceId: z.string(),
      changes: z.array(z.object({
        entityType: z.string(),
        entityId: z.string(),
        action: z.enum(["create", "update", "delete"]),
        data: z.any(),
        clientTimestamp: z.number(),
        clientVersion: z.number(),
      })),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/offline-sync/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  // Start reporting journeys (23-26)
  startMonthlyReporting: protectedProcedure
    .input(z.object({
      reportType: z.enum(["monthly", "quarterly", "annual"]),
      programId: z.string().optional(),
      startDate: z.string(),
      endDate: z.string(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/monthly-reporting/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startProgramAnalytics: protectedProcedure
    .input(z.object({
      programId: z.string(),
      analysisType: z.enum(["performance", "trends", "comparison"]),
      dateRange: z.object({
        start: z.string(),
        end: z.string(),
      }),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/program-analytics/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startDashboardRefresh: protectedProcedure
    .input(z.object({
      dashboardId: z.string(),
      widgets: z.array(z.string()).optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/dashboard-refresh/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startDataExport: protectedProcedure
    .input(z.object({
      exportType: z.enum(["beneficiaries", "disbursements", "grievances", "programs"]),
      format: z.enum(["csv", "excel", "json"]),
      filters: z.record(z.string(), z.any()).optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/data-export/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  // Start fraud/compliance journeys (27-30)
  startFraudInvestigation: protectedProcedure
    .input(z.object({
      beneficiaryId: z.string(),
      allegationType: z.enum(["duplicate_identity", "false_information", "unauthorized_access", "payment_fraud"]),
      description: z.string(),
      evidence: z.array(z.string()).optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/fraud-investigation/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startFraudPrediction: protectedProcedure
    .input(z.object({
      scope: z.enum(["all", "program", "region"]),
      scopeId: z.string().optional(),
      threshold: z.number().default(0.7),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/fraud-prediction/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startNationalIDVerification: protectedProcedure
    .input(z.object({
      beneficiaryId: z.string(),
      providerId: z.string(),
      verificationLevel: z.enum(["basic", "enhanced", "full"]),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/national-id-verification/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),

  startCrossSectorInterop: protectedProcedure
    .input(z.object({
      beneficiaryId: z.string(),
      sector: z.enum(["health", "education", "tax", "labor"]),
      operation: z.enum(["query", "share", "verify"]),
      dataFields: z.array(z.string()).optional(),
    }))
    .mutation(async ({ input, ctx }) => {
      return callGoService("/api/journeys/cross-sector-interop/start", "POST", {
        ...input,
        actorId: ctx.session?.user?.id,
        tenantId: ctx.session?.user?.tenantId || "default",
      });
    }),
});

export const worldClassRouter = router({
  offlineSync: offlineSyncRouter,
  federation: federationRouter,
  pmt: pmtRouter,
  interop: interopRouter,
  loadTest: loadTestRouter,
  journeys: journeyRouter,
});
