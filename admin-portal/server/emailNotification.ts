import { notifyOwner } from "./_core/notification";

/**
 * Email notification service for admin actions
 * Uses the built-in notifyOwner helper to send notifications
 */

export interface NotificationPayload {
  title: string;
  content: string;
  actionType: string;
  performedBy: string;
  details?: Record<string, any>;
}

/**
 * Send email notification for critical admin actions
 */
export async function sendAdminNotification(payload: NotificationPayload): Promise<boolean> {
  try {
    const formattedContent = `
**Action**: ${payload.actionType}
**Performed By**: ${payload.performedBy}

${payload.content}

${payload.details ? `\n**Details**:\n${JSON.stringify(payload.details, null, 2)}` : ""}
    `.trim();

    const success = await notifyOwner({
      title: `[Admin Portal] ${payload.title}`,
      content: formattedContent,
    });

    return success;
  } catch (error) {
    console.error("[EmailNotification] Failed to send notification:", error);
    return false;
  }
}

/**
 * Notification templates for different action types
 */
export const NotificationTemplates = {
  roleChange: (data: { userName: string; oldRole: string; newRole: string; performedBy: string }) => ({
    title: "User Role Changed",
    content: `User **${data.userName}** role has been changed from **${data.oldRole}** to **${data.newRole}**.`,
    actionType: "role_change",
    performedBy: data.performedBy,
    details: { userName: data.userName, oldRole: data.oldRole, newRole: data.newRole },
  }),

  batchProgramUpdate: (data: { count: number; action: string; performedBy: string }) => ({
    title: "Batch Program Update",
    content: `**${data.count}** benefit programs have been ${data.action}.`,
    actionType: "batch_update_programs",
    performedBy: data.performedBy,
    details: { count: data.count, action: data.action },
  }),

  batchFlagToggle: (data: { count: number; action: string; performedBy: string }) => ({
    title: "Batch Feature Flag Update",
    content: `**${data.count}** feature flags have been ${data.action}.`,
    actionType: "batch_toggle_flags",
    performedBy: data.performedBy,
    details: { count: data.count, action: data.action },
  }),

  mccRuleChange: (data: { programName: string; mccCode: string; action: string; performedBy: string }) => ({
    title: "MCC Rule Modified",
    content: `MCC code **${data.mccCode}** has been ${data.action} for program **${data.programName}**.`,
    actionType: `${data.action}_mcc`,
    performedBy: data.performedBy,
    details: { programName: data.programName, mccCode: data.mccCode, action: data.action },
  }),

  programCreated: (data: { programName: string; performedBy: string }) => ({
    title: "New Benefit Program Created",
    content: `A new benefit program **${data.programName}** has been created.`,
    actionType: "create_program",
    performedBy: data.performedBy,
    details: { programName: data.programName },
  }),
};
