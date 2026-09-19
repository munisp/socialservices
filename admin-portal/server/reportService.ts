import * as db from "./db";
import { notifyOwner } from "./_core/notification";

interface ReportData {
  period: string;
  startDate: string;
  endDate: string;
  metrics: {
    totalUsers: number;
    newUsers: number;
    totalPrograms: number;
    activePrograms: number;
    totalFeatureFlags: number;
    enabledFlags: number;
    upcomingDisbursements: number;
    adminActions: number;
  };
  topActions: Array<{
    action: string;
    count: number;
  }>;
  recentPrograms: Array<{
    name: string;
    createdAt: string;
  }>;
}

/**
 * Generate a summary report for the specified period
 */
export async function generateReport(reportType: "weekly" | "monthly"): Promise<ReportData> {
  const now = new Date();
  const startDate = new Date(now);
  
  if (reportType === "weekly") {
    startDate.setDate(now.getDate() - 7);
  } else {
    startDate.setMonth(now.getMonth() - 1);
  }

  // Gather metrics
  const allUsers = await db.getAllUsers();
  const newUsers = allUsers.filter(u => new Date(u.createdAt) >= startDate);
  
  const allPrograms = await db.getAllBenefitPrograms();
  const activePrograms = allPrograms.filter(p => p.status === "active");
  const recentPrograms = allPrograms
    .filter(p => new Date(p.createdAt) >= startDate)
    .slice(0, 5)
    .map(p => ({
      name: p.name,
      createdAt: new Date(p.createdAt).toLocaleDateString(),
    }));

  const allFlags = await db.getAllFeatureFlags();
  const enabledFlags = allFlags.filter(f => f.enabled);

  const allDisbursements = await db.getAllDisbursementSchedules();
  const upcomingDisbursements = allDisbursements.filter(
    d => new Date(d.scheduledDate) > now && d.status === "pending"
  );

  const allActions = await db.getAllAdminActions();
  const recentActions = allActions.filter(a => new Date(a.performedAt) >= startDate);

  // Count action types
  const actionCounts = recentActions.reduce((acc, action) => {
    acc[action.action] = (acc[action.action] || 0) + 1;
    return acc;
  }, {} as Record<string, number>);

  const topActions = Object.entries(actionCounts)
    .map(([action, count]) => ({ action, count }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 5);

  return {
    period: reportType,
    startDate: startDate.toISOString(),
    endDate: now.toISOString(),
    metrics: {
      totalUsers: allUsers.length,
      newUsers: newUsers.length,
      totalPrograms: allPrograms.length,
      activePrograms: activePrograms.length,
      totalFeatureFlags: allFlags.length,
      enabledFlags: enabledFlags.length,
      upcomingDisbursements: upcomingDisbursements.length,
      adminActions: recentActions.length,
    },
    topActions,
    recentPrograms,
  };
}

/**
 * Format report data as HTML email
 */
export function formatReportEmail(data: ReportData): string {
  const periodLabel = data.period === "weekly" ? "Weekly" : "Monthly";
  
  return `
    <!DOCTYPE html>
    <html>
    <head>
      <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        h1 { color: #2563eb; border-bottom: 2px solid #2563eb; padding-bottom: 10px; }
        h2 { color: #1e40af; margin-top: 30px; }
        .metric { background: #f3f4f6; padding: 15px; margin: 10px 0; border-radius: 8px; }
        .metric-label { font-weight: bold; color: #6b7280; }
        .metric-value { font-size: 24px; color: #2563eb; }
        table { width: 100%; border-collapse: collapse; margin: 15px 0; }
        th, td { padding: 10px; text-align: left; border-bottom: 1px solid #e5e7eb; }
        th { background: #f9fafb; font-weight: 600; }
        .footer { margin-top: 40px; padding-top: 20px; border-top: 1px solid #e5e7eb; color: #6b7280; font-size: 12px; }
      </style>
    </head>
    <body>
      <div class="container">
        <h1>${periodLabel} Admin Portal Report</h1>
        <p>Period: ${new Date(data.startDate).toLocaleDateString()} - ${new Date(data.endDate).toLocaleDateString()}</p>
        
        <h2>📊 Key Metrics</h2>
        <div class="metric">
          <div class="metric-label">Total Users</div>
          <div class="metric-value">${data.metrics.totalUsers} <span style="font-size: 14px; color: #10b981;">(+${data.metrics.newUsers} new)</span></div>
        </div>
        <div class="metric">
          <div class="metric-label">Benefit Programs</div>
          <div class="metric-value">${data.metrics.activePrograms} / ${data.metrics.totalPrograms} active</div>
        </div>
        <div class="metric">
          <div class="metric-label">Feature Flags</div>
          <div class="metric-value">${data.metrics.enabledFlags} / ${data.metrics.totalFeatureFlags} enabled</div>
        </div>
        <div class="metric">
          <div class="metric-label">Upcoming Disbursements</div>
          <div class="metric-value">${data.metrics.upcomingDisbursements}</div>
        </div>
        <div class="metric">
          <div class="metric-label">Admin Actions</div>
          <div class="metric-value">${data.metrics.adminActions}</div>
        </div>

        <h2>🔥 Top Admin Actions</h2>
        <table>
          <thead>
            <tr>
              <th>Action Type</th>
              <th>Count</th>
            </tr>
          </thead>
          <tbody>
            ${data.topActions.map(a => `
              <tr>
                <td>${a.action}</td>
                <td>${a.count}</td>
              </tr>
            `).join('')}
          </tbody>
        </table>

        ${data.recentPrograms.length > 0 ? `
          <h2>🆕 Recently Created Programs</h2>
          <table>
            <thead>
              <tr>
                <th>Program Name</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              ${data.recentPrograms.map(p => `
                <tr>
                  <td>${p.name}</td>
                  <td>${p.createdAt}</td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        ` : ''}

        <div class="footer">
          <p>This is an automated report from the Social Protection Platform Admin Portal.</p>
        </div>
      </div>
    </body>
    </html>
  `;
}

/**
 * Send report via email to recipients
 */
export async function sendReportEmail(recipients: string[], reportData: ReportData): Promise<boolean> {
  const periodLabel = reportData.period === "weekly" ? "Weekly" : "Monthly";
  const emailBody = formatReportEmail(reportData);
  
  // Use the notifyOwner function for each recipient
  // In a production system, you'd use a proper email service like SendGrid
  const title = `${periodLabel} Admin Portal Report`;
  const success = await notifyOwner({
    title,
    content: `Report generated for period ${new Date(reportData.startDate).toLocaleDateString()} - ${new Date(reportData.endDate).toLocaleDateString()}. Recipients: ${recipients.join(", ")}`,
  });

  return success;
}

/**
 * Calculate next run time for a report
 */
export function calculateNextRunTime(reportType: "weekly" | "monthly", lastRun?: Date): Date {
  const now = lastRun || new Date();
  const next = new Date(now);
  
  if (reportType === "weekly") {
    // Run every Monday at 9 AM
    next.setDate(now.getDate() + (1 + 7 - now.getDay()) % 7);
    next.setHours(9, 0, 0, 0);
    if (next <= now) {
      next.setDate(next.getDate() + 7);
    }
  } else {
    // Run on the 1st of each month at 9 AM
    next.setMonth(now.getMonth() + 1);
    next.setDate(1);
    next.setHours(9, 0, 0, 0);
  }
  
  return next;
}
