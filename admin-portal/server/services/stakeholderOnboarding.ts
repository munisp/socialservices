import { z } from "zod";

export const stakeholderTypes = ["beneficiary", "caregiver", "case_worker", "program_admin", "government_agency", "ngo_partner", "dfsp", "agent", "merchant", "auditor", "grievance_officer", "ml_governance"] as const;
export type StakeholderType = (typeof stakeholderTypes)[number];
export const onboardingStates = ["invited", "identity_pending", "organization_pending", "consent_pending", "scope_pending", "training_pending", "mfa_pending", "active", "suspended", "offboarded", "rejected"] as const;
export type OnboardingState = (typeof onboardingStates)[number];

export interface StakeholderPolicy { organization: boolean; training: boolean; mfa: boolean; programmeScope: boolean; safeguarding: boolean; paymentCertification: boolean; }
export const stakeholderPolicies: Record<StakeholderType, StakeholderPolicy> = {
  beneficiary: { organization: false, training: false, mfa: false, programmeScope: true, safeguarding: false, paymentCertification: false },
  caregiver: { organization: false, training: true, mfa: true, programmeScope: true, safeguarding: true, paymentCertification: false },
  case_worker: { organization: true, training: true, mfa: true, programmeScope: true, safeguarding: true, paymentCertification: false },
  program_admin: { organization: true, training: true, mfa: true, programmeScope: true, safeguarding: false, paymentCertification: false },
  government_agency: { organization: true, training: true, mfa: true, programmeScope: true, safeguarding: false, paymentCertification: false },
  ngo_partner: { organization: true, training: true, mfa: true, programmeScope: true, safeguarding: true, paymentCertification: false },
  dfsp: { organization: true, training: true, mfa: true, programmeScope: true, safeguarding: false, paymentCertification: true },
  agent: { organization: true, training: true, mfa: true, programmeScope: true, safeguarding: true, paymentCertification: true },
  merchant: { organization: true, training: true, mfa: true, programmeScope: true, safeguarding: false, paymentCertification: true },
  auditor: { organization: true, training: true, mfa: true, programmeScope: true, safeguarding: false, paymentCertification: false },
  grievance_officer: { organization: true, training: true, mfa: true, programmeScope: true, safeguarding: true, paymentCertification: false },
  ml_governance: { organization: true, training: true, mfa: true, programmeScope: true, safeguarding: false, paymentCertification: false },
};

export const onboardingInput = z.object({
  stakeholderType: z.enum(stakeholderTypes), displayName: z.string().trim().min(2).max(255), email: z.string().email().optional(), phone: z.string().trim().min(7).max(32).optional(),
  organizationId: z.number().int().positive().optional(), regionCodes: z.array(z.string().trim().min(1)).default([]), programIds: z.array(z.string().trim().min(1)).default([]),
  consentVersion: z.string().trim().min(1).optional(), identityEvidenceRefs: z.array(z.string().trim().min(1)).default([]), requiresMfa: z.boolean().optional(),
});
export type OnboardingInput = z.infer<typeof onboardingInput>;

export interface ActivationEvidence { identityVerified: boolean; organizationVerified: boolean; consentAccepted: boolean; scopeApproved: boolean; trainingCompleted: boolean; mfaEnrolled: boolean; safeguardingAttested: boolean; paymentCertified: boolean; }

export function activationGaps(type: StakeholderType, input: OnboardingInput, evidence: ActivationEvidence): string[] {
  const policy = stakeholderPolicies[type]; const gaps: string[] = [];
  if (!evidence.identityVerified || input.identityEvidenceRefs.length === 0) gaps.push("identity_verification");
  if (policy.organization && (!input.organizationId || !evidence.organizationVerified)) gaps.push("organization_verification");
  if (!input.consentVersion || !evidence.consentAccepted) gaps.push("consent_and_legal_basis");
  if (policy.programmeScope && (!input.regionCodes.length || !evidence.scopeApproved)) gaps.push("region_and_programme_scope");
  if (policy.training && !evidence.trainingCompleted) gaps.push("role_training");
  if ((input.requiresMfa ?? policy.mfa) && !evidence.mfaEnrolled) gaps.push("mfa_enrollment");
  if (policy.safeguarding && !evidence.safeguardingAttested) gaps.push("safeguarding_attestation");
  if (policy.paymentCertification && !evidence.paymentCertified) gaps.push("payment_and_settlement_certification");
  return gaps;
}

export function nextRequiredState(input: OnboardingInput): OnboardingState {
  const policy = stakeholderPolicies[input.stakeholderType];
  if (!input.identityEvidenceRefs.length) return "identity_pending";
  if (policy.organization && !input.organizationId) return "organization_pending";
  if (!input.consentVersion) return "consent_pending";
  if (policy.programmeScope && !input.regionCodes.length) return "scope_pending";
  if (policy.training) return "training_pending";
  if (input.requiresMfa ?? policy.mfa) return "mfa_pending";
  return "active";
}

export function canTransition(from: OnboardingState, to: OnboardingState): boolean {
  const transitions: Record<OnboardingState, OnboardingState[]> = {
    invited: ["identity_pending", "organization_pending", "rejected"], identity_pending: ["organization_pending", "consent_pending", "rejected"], organization_pending: ["consent_pending", "rejected"], consent_pending: ["scope_pending", "rejected"], scope_pending: ["training_pending", "mfa_pending", "active", "rejected"], training_pending: ["mfa_pending", "active", "rejected"], mfa_pending: ["active", "rejected"], active: ["suspended", "offboarded"], suspended: ["active", "offboarded"], offboarded: [], rejected: [],
  };
  return transitions[from].includes(to);
}
