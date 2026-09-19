import { z } from "zod";

export const stakeholderTypes = ["beneficiary", "caregiver", "case_worker", "program_admin", "government_agency", "ngo_partner", "dfsp", "agent", "merchant", "auditor", "grievance_officer", "ml_governance"] as const;
export type StakeholderType = (typeof stakeholderTypes)[number];
export const onboardingStates = ["invited", "identity_pending", "organization_pending", "consent_pending", "scope_pending", "training_pending", "mfa_pending", "active", "suspended", "offboarded", "rejected"] as const;
export type OnboardingState = (typeof onboardingStates)[number];

export const onboardingInput = z.object({
  stakeholderType: z.enum(stakeholderTypes),
  displayName: z.string().trim().min(2).max(255),
  email: z.string().email().optional(),
  phone: z.string().trim().min(7).max(32).optional(),
  organizationId: z.string().trim().min(1).optional(),
  regionCodes: z.array(z.string().trim().min(1)).min(1),
  programIds: z.array(z.string().trim().min(1)).default([]),
  consentVersion: z.string().trim().min(1),
  identityEvidenceRefs: z.array(z.string().trim().min(1)).min(1),
  requiresMfa: z.boolean().default(true),
});
export type OnboardingInput = z.infer<typeof onboardingInput>;

export function nextRequiredState(input: OnboardingInput): OnboardingState {
  if (input.stakeholderType !== "beneficiary" && !input.organizationId) return "organization_pending";
  if (!input.identityEvidenceRefs.length) return "identity_pending";
  if (!input.consentVersion) return "consent_pending";
  if (!input.regionCodes.length) return "scope_pending";
  return input.requiresMfa ? "mfa_pending" : "active";
}

export function canTransition(from: OnboardingState, to: OnboardingState): boolean {
  const transitions: Record<OnboardingState, OnboardingState[]> = {
    invited: ["identity_pending", "organization_pending", "rejected"], identity_pending: ["consent_pending", "rejected"], organization_pending: ["identity_pending", "rejected"], consent_pending: ["scope_pending", "rejected"], scope_pending: ["training_pending", "mfa_pending", "active", "rejected"], training_pending: ["mfa_pending", "active", "rejected"], mfa_pending: ["active", "rejected"], active: ["suspended", "offboarded"], suspended: ["active", "offboarded"], offboarded: [], rejected: [],
  };
  return transitions[from].includes(to);
}
