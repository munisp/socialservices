import { describe, expect, it } from "vitest";
import { activationGaps, canTransition, stakeholderPolicies, stakeholderTypes, type OnboardingInput } from "./stakeholderOnboarding";

const completeEvidence = { identityVerified: true, organizationVerified: true, consentAccepted: true, scopeApproved: true, trainingCompleted: true, mfaEnrolled: true, safeguardingAttested: true, paymentCertified: true };

describe("stakeholder onboarding policies", () => {
  it("defines a complete policy for all twelve stakeholder classes", () => {
    expect(stakeholderTypes).toHaveLength(12);
    expect(Object.keys(stakeholderPolicies).sort()).toEqual([...stakeholderTypes].sort());
  });

  it.each(stakeholderTypes)("allows %s activation only after its evidence gates", (stakeholderType) => {
    const input: OnboardingInput = { stakeholderType, displayName: "Verified stakeholder", organizationId: stakeholderPolicies[stakeholderType].organization ? 1 : undefined, regionCodes: ["NG-LA"], programIds: ["programme-1"], consentVersion: "v1", identityEvidenceRefs: ["evidence-1"], requiresMfa: stakeholderPolicies[stakeholderType].mfa };
    expect(activationGaps(stakeholderType, input, completeEvidence)).toEqual([]);
    expect(activationGaps(stakeholderType, input, { ...completeEvidence, identityVerified: false })).toContain("identity_verification");
  });

  it("prevents activation from early states and terminal-state resurrection", () => {
    expect(canTransition("invited", "active")).toBe(false);
    expect(canTransition("offboarded", "active")).toBe(false);
    expect(canTransition("rejected", "active")).toBe(false);
    expect(canTransition("suspended", "active")).toBe(true);
  });
});
