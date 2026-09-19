import mysql from "mysql2/promise";

const DATABASE_URL = process.env.DATABASE_URL;

if (!DATABASE_URL) {
  console.error("❌ DATABASE_URL environment variable is not set");
  process.exit(1);
}

async function seedNotificationPreferences() {
  console.log("🌱 Starting notification preferences seeding...\n");

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
    console.log(`👤 Configuring notifications for: ${adminUser.name} (${adminUser.email})\n`);

    // Check if preferences already exist
    const [existing] = await connection.query(
      "SELECT id FROM notification_preferences WHERE userId = ?",
      [adminUser.id]
    );

    if (existing.length > 0) {
      console.log("⚠️  Notification preferences already exist. Updating...\n");
      
      await connection.query(
        `UPDATE notification_preferences 
         SET 
           notifyOnRoleChange = ?,
           notifyOnBatchOperations = ?,
           notifyOnProgramChanges = ?,
           notifyOnMccRuleChanges = ?,
           emailAddress = ?
         WHERE userId = ?`,
        [
          true,  // notifyOnRoleChange
          true,  // notifyOnBatchOperations
          true,  // notifyOnProgramChanges
          true,  // notifyOnMccRuleChanges
          adminUser.email,
          adminUser.id,
        ]
      );
      
      console.log("✅ Updated existing notification preferences");
    } else {
      console.log("📝 Creating new notification preferences...\n");
      
      await connection.query(
        `INSERT INTO notification_preferences 
         (userId, notifyOnRoleChange, notifyOnBatchOperations, notifyOnProgramChanges, notifyOnMccRuleChanges, emailAddress) 
         VALUES (?, ?, ?, ?, ?, ?)`,
        [
          adminUser.id,
          true,  // notifyOnRoleChange
          true,  // notifyOnBatchOperations
          true,  // notifyOnProgramChanges
          true,  // notifyOnMccRuleChanges
          adminUser.email,
        ]
      );
      
      console.log("✅ Created new notification preferences");
    }

    // Show configured preferences
    const [preferences] = await connection.query(
      `SELECT * FROM notification_preferences WHERE userId = ?`,
      [adminUser.id]
    );

    if (preferences.length > 0) {
      const prefs = preferences[0];
      
      console.log("\n" + "=".repeat(60));
      console.log("📧 Notification Preferences:");
      console.log("=".repeat(60));
      console.log(`Email Address: ${prefs.emailAddress || adminUser.email}`);
      console.log("\nEnabled Notifications:");
      console.log(`  ${prefs.notifyOnRoleChange ? '✅' : '⚪'} Role Changes - Alerts when user roles are modified`);
      console.log(`  ${prefs.notifyOnBatchOperations ? '✅' : '⚪'} Batch Operations - Notifications for bulk imports and updates`);
      console.log(`  ${prefs.notifyOnProgramChanges ? '✅' : '⚪'} Program Changes - Updates when benefit programs are modified`);
      console.log(`  ${prefs.notifyOnMccRuleChanges ? '✅' : '⚪'} MCC Rule Changes - Alerts for MCC code additions/removals`);
      console.log("=".repeat(60));
    }

    // Show notification triggers
    console.log("\n📬 Notification Triggers:");
    console.log("=".repeat(60));
    console.log("The following actions will trigger email notifications:\n");
    console.log("Role Changes:");
    console.log("  • User promoted to admin");
    console.log("  • User demoted from admin");
    console.log("  • Bulk role assignments\n");
    
    console.log("Batch Operations:");
    console.log("  • Bulk MCC import completed");
    console.log("  • Multiple programs updated");
    console.log("  • Batch feature flag toggles\n");
    
    console.log("Program Changes:");
    console.log("  • New benefit program created");
    console.log("  • Program deleted or archived");
    console.log("  • Program settings modified\n");
    
    console.log("MCC Rule Changes:");
    console.log("  • MCC codes added to programs");
    console.log("  • MCC codes removed from programs");
    console.log("  • MCC approval requests submitted");
    console.log("=".repeat(60));

    console.log("\n💡 Usage:");
    console.log("=".repeat(60));
    console.log("1. Navigate to 'Notification Settings' in the Admin Portal");
    console.log("2. Toggle notification types on/off as needed");
    console.log("3. Update email address if different from account email");
    console.log("4. Test notifications by performing a tracked action");

    console.log("\n✨ Notification preferences seeding completed!");
  } catch (error) {
    console.error("\n❌ Fatal error during seeding:", error);
    process.exit(1);
  } finally {
    if (connection) {
      await connection.end();
    }
  }
}

seedNotificationPreferences().catch((error) => {
  console.error("❌ Unhandled error:", error);
  process.exit(1);
});
