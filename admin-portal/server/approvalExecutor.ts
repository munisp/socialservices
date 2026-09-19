import * as db from "./db";

/**
 * Execute an approved request based on its type
 */
export async function executeApprovedRequest(request: any): Promise<void> {
  switch (request.requestType) {
    case "ROLE_CHANGE":
      await db.updateUserRole(request.targetId, request.requestData.newRole);
      await db.logAdminAction({
        performedBy: request.requestedBy,
        action: "ROLE_CHANGE",
        targetUserId: request.targetId,
        justification: `Approved request: ${request.justification}`,
        details: request.requestData,
      });
      break;

    case "BATCH_PROGRAM_UPDATE":
      for (const programId of request.requestData.programIds) {
        await db.updateBenefitProgramStatus(programId, request.requestData.status);
      }
      await db.logAdminAction({
        performedBy: request.requestedBy,
        action: "BATCH_PROGRAM_UPDATE",
        justification: `Approved request: ${request.justification}`,
        details: request.requestData,
      });
      break;

    case "BATCH_FLAG_TOGGLE":
      for (const flagId of request.requestData.flagIds) {
        await db.updateFeatureFlag(flagId, { enabled: request.requestData.enabled });
      }
      await db.logAdminAction({
        performedBy: request.requestedBy,
        action: "BATCH_FLAG_TOGGLE",
        justification: `Approved request: ${request.justification}`,
        details: request.requestData,
      });
      break;

    case "PROGRAM_DELETE":
      // Delete program logic would go here
      await db.logAdminAction({
        performedBy: request.requestedBy,
        action: "PROGRAM_DELETE",
        targetUserId: request.targetId,
        justification: `Approved request: ${request.justification}`,
        details: request.requestData,
      });
      break;

    case "USER_DELETE":
      // Delete user logic would go here
      await db.logAdminAction({
        performedBy: request.requestedBy,
        action: "USER_DELETE",
        targetUserId: request.targetId,
        justification: `Approved request: ${request.justification}`,
        details: request.requestData,
      });
      break;

    default:
      throw new Error(`Unknown request type: ${request.requestType}`);
  }
}
