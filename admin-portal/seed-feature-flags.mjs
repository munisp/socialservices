import mysql from "mysql2/promise";

const DATABASE_URL = process.env.DATABASE_URL;

if (!DATABASE_URL) {
  console.error("❌ DATABASE_URL environment variable is not set");
  process.exit(1);
}

const featureFlags = [
  {
    name: "enable_bulk_mcc_import",
    description: "Allows administrators to import multiple MCC codes at once via CSV upload. When disabled, MCC codes must be added individually through the UI.",
    enabled: true,
    environment: "all",
  },
  {
    name: "require_approval_for_mcc_changes",
    description: "Requires multi-level approval workflow for adding or removing MCC codes from benefit programs. Critical for compliance and fraud prevention.",
    enabled: true,
    environment: "production",
  },
  {
    name: "enable_realtime_notifications",
    description: "Sends real-time notifications to administrators for critical events such as failed disbursements, approval requests, and security alerts.",
    enabled: true,
    environment: "all",
  },
  {
    name: "enable_advanced_analytics",
    description: "Enables advanced analytics dashboard with MCC usage patterns, beneficiary trends, and predictive insights. Requires additional compute resources.",
    enabled: true,
    environment: "production",
  },
  {
    name: "enable_opensearch_integration",
    description: "Enables full-text search across all entities using OpenSearch. Requires OpenSearch cluster configuration.",
    enabled: false,
    environment: "all",
  },
  {
    name: "enable_audit_log_export",
    description: "Allows exporting audit logs to external systems for compliance reporting and long-term archival.",
    enabled: true,
    environment: "production",
  },
  {
    name: "enable_program_templates",
    description: "Allows creating and using program templates for quick duplication of benefit programs with pre-configured MCC rules.",
    enabled: true,
    environment: "all",
  },
  {
    name: "enable_scheduled_reports",
    description: "Enables automated weekly and monthly summary reports delivered via email to administrators.",
    enabled: true,
    environment: "production",
  },
  {
    name: "enable_disbursement_simulation",
    description: "Allows running disbursement simulations to test payment schedules and beneficiary calculations before actual execution.",
    enabled: true,
    environment: "staging",
  },
  {
    name: "enable_api_rate_limiting",
    description: "Enforces rate limits on API endpoints to prevent abuse and ensure fair resource allocation.",
    enabled: true,
    environment: "production",
  },
];

async function seedFeatureFlags() {
  console.log("🌱 Starting feature flags seeding...\n");

  let connection;
  try {
    connection = await mysql.createConnection(DATABASE_URL);

    console.log(`📊 Creating ${featureFlags.length} feature flags\n`);

    let successCount = 0;
    let errorCount = 0;
    const errors = [];

    for (const flag of featureFlags) {
      try {
        await connection.query(
          `INSERT INTO feature_flags (name, description, enabled, environment) 
           VALUES (?, ?, ?, ?)
           ON DUPLICATE KEY UPDATE 
           description = VALUES(description),
           enabled = VALUES(enabled),
           environment = VALUES(environment)`,
          [flag.name, flag.description, flag.enabled, flag.environment]
        );
        
        const status = flag.enabled ? "✅ ENABLED" : "⚪ DISABLED";
        const env = flag.environment === "all" ? "ALL ENVS" : flag.environment.toUpperCase();
        console.log(`${status} | ${env.padEnd(12)} | ${flag.name}`);
        
        successCount++;
      } catch (error) {
        errorCount++;
        errors.push({
          name: flag.name,
          error: error.message,
        });
      }
    }

    console.log("\n" + "=".repeat(60));
    console.log("📈 Seeding Summary:");
    console.log("=".repeat(60));
    console.log(`✅ Successfully created: ${successCount} flags`);
    console.log(`❌ Failed: ${errorCount} flags`);

    if (errors.length > 0) {
      console.log("\n⚠️  Errors:");
      errors.forEach(({ name, error }) => {
        console.log(`   - ${name}: ${error}`);
      });
    }

    console.log("=".repeat(60));

    // Show breakdown by environment
    const [breakdown] = await connection.query(`
      SELECT 
        environment,
        COUNT(*) as total_flags,
        SUM(CASE WHEN enabled = 1 THEN 1 ELSE 0 END) as enabled_flags,
        SUM(CASE WHEN enabled = 0 THEN 1 ELSE 0 END) as disabled_flags
      FROM feature_flags
      GROUP BY environment
      ORDER BY 
        CASE environment
          WHEN 'all' THEN 1
          WHEN 'production' THEN 2
          WHEN 'staging' THEN 3
          WHEN 'development' THEN 4
        END
    `);

    console.log("\n📊 Feature Flags by Environment:");
    console.log("=".repeat(60));
    breakdown.forEach(row => {
      console.log(`\n${row.environment.toUpperCase()}:`);
      console.log(`  Total Flags: ${row.total_flags}`);
      console.log(`  Enabled: ${row.enabled_flags}`);
      console.log(`  Disabled: ${row.disabled_flags}`);
    });

    console.log("\n✨ Feature flags seeding completed!");
  } catch (error) {
    console.error("\n❌ Fatal error during seeding:", error);
    process.exit(1);
  } finally {
    if (connection) {
      await connection.end();
    }
  }
}

seedFeatureFlags().catch((error) => {
  console.error("❌ Unhandled error:", error);
  process.exit(1);
});
