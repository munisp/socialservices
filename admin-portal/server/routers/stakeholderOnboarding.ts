import { TRPCError } from "@trpc/server";
import { z } from "zod";
import { adminProcedure, protectedProcedure, router } from "../_core/trpc";
import * as store from "../stakeholderOnboardingDb";
import { onboardingInput, onboardingStates, stakeholderPolicies, stakeholderTypes } from "../services/stakeholderOnboarding";

export const stakeholderOnboardingRouter = router({
  catalog: protectedProcedure.query(() => ({ stakeholderTypes, policies: stakeholderPolicies })),
  organizations: router({
    list: adminProcedure.query(() => store.listOrganizations()),
    create: adminProcedure.input(z.object({ legalName: z.string().trim().min(2).max(255), stakeholderType: z.enum(stakeholderTypes), registrationReference: z.string().trim().min(3).max(255) })).mutation(async ({ input }) => ({ id: await store.createOrganization(input.legalName, input.stakeholderType, input.registrationReference) })),
    verify: adminProcedure.input(z.object({ id: z.number().int().positive(), status: z.enum(["verified", "rejected", "suspended"]) })).mutation(async ({ input }) => { await store.setOrganizationVerification(input.id, input.status); return { success: true }; }),
  }),
  create: protectedProcedure.input(onboardingInput.extend({ userId: z.number().int().positive().optional() })).mutation(async ({ input, ctx }) => ({ id: await store.createOnboarding(input, input.userId ?? ctx.user.id, ctx.user.id) })),
  list: adminProcedure.query(() => store.listOnboardings()),
  get: protectedProcedure.input(z.object({ id: z.number().int().positive() })).query(async ({ input }) => { const value = await store.getOnboarding(input.id); if (!value) throw new TRPCError({ code: "NOT_FOUND" }); return value; }),
  events: protectedProcedure.input(z.object({ id: z.number().int().positive() })).query(({ input }) => store.getEvents(input.id)),
  transition: adminProcedure.input(z.object({ id: z.number().int().positive(), to: z.enum(onboardingStates), reason: z.string().trim().min(5), evidence: z.record(z.string(), z.unknown()).default({}) })).mutation(async ({ input, ctx }) => {
    try { return await store.transition(input.id, input.to, ctx.user.id, input.reason, input.evidence); }
    catch (error) { throw new TRPCError({ code: "BAD_REQUEST", message: error instanceof Error ? error.message : "Transition failed" }); }
  }),
});
