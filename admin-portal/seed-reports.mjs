import mysql from "mysql2/promise";

const DATABASE_URL = process.env.DATABASE_URL;

if (!DATABASE_URL) {
  console.error("❌ DATABASE_URL environment variable is not set");
  process.exit(1);
}

// Helper to calculate next run time
function getNextRunTime(reportType) {
  const now = new Date();
  
  if (reportType === "weekly") {
    // Next Monday at 9 AM
    const nextMonday = new Date(now);
    nextMonday.setDate(now.getDate() + ((1 + 7 - now.getDay()) % 7 || 7));
    nextMonday.setHours(9, 0, 0, 0);
    return nextMonday;
  } else {
    // First day of next month at 9 AM
    const nextMonth = new Date(now.getFullYear(), now.getMonth() + 1, 1, 9, 0, 0, 0);
    return nextMonth;
  }
}

async function seedReports() {
  console.log("🌱 Starting scheduled reports seeding...\n");

  let connection;
  try {
    connection = await mysql.createConnection(DATABASE_URL);

    // Get admin user
    const [users] = await connection.query(
      "SELECT id, name, email FROM users WHERE role = 'admin' LIMIT 1"
    );

    if (users.length === 0) {
      console.error("❌ No admin user found. Please ensure you're logged in as admin first.");
      process.exit(1);
    }

    const adminUser = users[0];
    console.log(`👤 Configuring reports for: ${adminUser.name} (${adminUser.email})\n`);

    const scheduledReports = [
      {
        name: "Weekly Platform Summary",
        description: "Comprehensive weekly summary including new users, program activity, disbursement status, and pending approvals. Delivered every Monday morning.",
        reportType: "weekly",
        recipients: JSON.stringify([adminUser.email]),
        isActive: 1,
        nextRunAt: getNextRunTime("weekly"),
        createdBy: adminUser.id,
      },
      {
        name: "Weekly Disbursement Report",
        description: "Weekly report on disbursement schedules, completed payments, failed transactions, and upcoming disbursements. Helps track payment execution and identify issues.",
        reportType: "weekly",
        recipients: JSON.stringify([adminUser.email, "finance@example.com"]),
        isActive: 1,
        nextRunAt: getNextRunTime("weekly"),
        createdBy: adminUser.id,
      },
      {
        name: "Monthly Analytics Dashboard",
        description: "Comprehensive monthly analytics including MCC usage patterns, beneficiary trends, program performance metrics, and cost analysis. Delivered on the first of each month.",
        reportType: "monthly",
        recipients: JSON.stringify([adminUser.email, "analytics@example.com"]),
        isActive: 1,
        nextRunAt: getNextRunTime("monthly"),
        createdBy: adminUser.id,
      },
      {
        name: "Monthly Compliance Report",
        description: "Monthly compliance and audit report covering all administrative actions, approval workflows, role changes, and security events. Essential for regulatory compliance.",
        reportType: "monthly",
        recipients: JSON.stringify([adminUser.email, "compliance@example.com", "audit@example.com"]),
        isActive: 1,
        nextRunAt: getNextRunTime("monthly"),
        createdBy: adminUser.id,
      },
      {
        name: "Weekly Approval Queue Digest",
        description: "Weekly digest of pending approval requests, recently approved/rejected items, and approval workflow metrics. Helps administrators stay on top of pending actions.",
        reportType: "weekly",
        recipients: JSON.stringify([adminUser.email]),
        isActive: 1,
        nextRunAt: getNextRunTime("weekly"),
        createdBy: adminUser.id,
      },
    ];

    console.log(`📊 Creating ${scheduledReports.length} scheduled reports\n`);

    let successCount = 0;
    let errorCount = 0;

    for (const report of scheduledReports) {
      try {
        await connection.query(
          `INSERT INTO scheduled_reports 
           (name, description, report_type, recipients, is_active, next_run_at, created_by) 
           VALUES (?, ?, ?, ?, ?, ?, ?)
           ON DUPLICATE KEY UPDATE 
           description = VALUES(description),
           recipients = VALUES(recipients),
           is_active = VALUES(is_active),
           next_run_at = VALUES(next_run_at)`,
          [
            report.name,
            report.description,
            report.reportType,
            report.recipients,
            report.isActive,
            report.nextRunAt,
            report.createdBy,
          ]
        );

        const recipientList = JSON.parse(report.recipients);
        const nextRun = report.nextRunAt.toLocaleDateString('en-US', { 
          weekday: 'short', 
          year: 'numeric', 
          month: 'short', 
          day: 'numeric',
          hour: '2-digit',
          minute: '2-digit'
        });

        console.log(`✓ Created report: ${report.name}`);
        console.log(`  Type: ${report.reportType.toUpperCase()}`);
        console.log(`  Recipients: ${recipientList.length} (${recipientList.slice(0, 2).join(', ')}${recipientList.length > 2 ? '...' : ''})`);
        console.log(`  Next Run: ${nextRun}\n`);
        
        successCount++;
      } catch (error) {
        console.error(`❌ Failed to create ${report.name}:`, error.message);
        errorCount++;
      }
    }

    console.log("=".repeat(60));
    console.log("📈 Seeding Summary:");
    console.log("=".repeat(60));
    console.log(`✅ Successfully created: ${successCount} reports`);
    console.log(`❌ Failed: ${errorCount} reports`);
    console.log("=".repeat(60));

    // Show breakdown by report type
    const [breakdown] = await connection.query(`
      SELECT 
        report_type,
        COUNT(*) as report_count,
        SUM(is_active) as active_count,
        GROUP_CONCAT(name SEPARATOR ' | ') as report_names
      FROM scheduled_reports
      GROUP BY report_type
    `);

    console.log("\n📊 Reports by Type:");
    console.log("=".repeat(60));
    breakdown.forEach(row => {
      console.log(`\n${row.report_type.toUpperCase()}:`);
      console.log(`  Total: ${row.report_count} reports`);
      console.log(`  Active: ${row.active_count} reports`);
    });

    // Show next scheduled runs
    const [nextRuns] = await connection.query(`
      SELECT name, report_type, next_run_at
      FROM scheduled_reports
      WHERE is_active = 1
      ORDER BY next_run_at
      LIMIT 5
    `);

    console.log("\n📅 Next Scheduled Runs:");
    console.log("=".repeat(60));
    nextRuns.forEach(row => {
      const nextRun = new Date(row.next_run_at).toLocaleDateString('en-US', { 
        weekday: 'short', 
        month: 'short', 
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
      });
      console.log(`${nextRun.padEnd(25)} - ${row.name}`);
    });

    console.log("\n💡 Usage:");
    console.log("=".repeat(60));
    console.log("1. Navigate to 'Scheduled Reports' in the Admin Portal");
    console.log("2. View configured reports and their schedules");
    console.log("3. Edit recipients or toggle active status as needed");
    console.log("4. Check 'Report History' to see past report executions");

    console.log("\n✨ Scheduled reports seeding completed!");
  } catch (error) {
    console.error("\n❌ Fatal error during seeding:", error);
    process.exit(1);
  } finally {
    if (connection) {
      await connection.end();
    }
  }
}

seedReports().catch((error) => {
  console.error("❌ Unhandled error:", error);
  process.exit(1);
});
