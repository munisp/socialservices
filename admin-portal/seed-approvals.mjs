import mysql from "mysql2/promise";

const DATABASE_URL = process.env.DATABASE_URL;

if (!DATABASE_URL) {
  console.error("❌ DATABASE_URL environment variable is not set");
  process.exit(1);
}

async function seedApprovals() {
  console.log("🌱 Starting sample approval workflow seeding...\n");

  let connection;
  try {
    connection = await mysql.createConnection(DATABASE_URL);

    // Get the admin user (should be the first user)
    const [users] = await connection.query(
      "SELECT id, name, email FROM users WHERE role = 'admin' LIMIT 1"
    );

    if (users.length === 0) {
      console.error("❌ No admin user found. Please ensure you're logged in as admin first.");
      process.exit(1);
    }

    const adminUser = users[0];
    console.log(`👤 Found admin user: ${adminUser.name || adminUser.email} (ID: ${adminUser.id})\n`);

    // Get the Food Assistance Program
    const [programs] = await connection.query(
      "SELECT id, name FROM benefit_programs WHERE accountType = 'FOOD_BENEFIT' LIMIT 1"
    );

    if (programs.length === 0) {
      console.error("❌ No Food Assistance Program found. Please run seed-programs.mjs first.");
      process.exit(1);
    }

    const foodProgram = programs[0];
    console.log(`📋 Target program: ${foodProgram.name} (ID: ${foodProgram.id})\n`);

    // Create approval requests
    const approvalRequests = [
      {
        requestType: "BATCH_PROGRAM_UPDATE",
        targetId: foodProgram.id,
        requestData: JSON.stringify({
          action: "add_mcc_code",
          mccCode: "5814",
          mccDescription: "Fast Food Restaurants",
          programId: foodProgram.id,
          programName: foodProgram.name,
        }),
        justification: "Adding fast food restaurants to Food Assistance Program to provide more flexibility for beneficiaries who may not have cooking facilities or time to prepare meals. This change has been requested by multiple beneficiaries and social workers.",
        status: "pending",
        requestedBy: adminUser.id,
        requiredApprovals: 2,
        currentApprovals: 0,
      },
      {
        requestType: "BATCH_PROGRAM_UPDATE",
        targetId: foodProgram.id,
        requestData: JSON.stringify({
          action: "remove_mcc_code",
          mccCode: "5441",
          mccDescription: "Candy Nut and Confectionery Stores",
          programId: foodProgram.id,
          programName: foodProgram.name,
        }),
        justification: "Removing candy and confectionery stores from approved MCC list to align with nutritional guidelines and ensure benefit funds are used for healthy food purchases. This change supports program goals of improving beneficiary health outcomes.",
        status: "pending",
        requestedBy: adminUser.id,
        requiredApprovals: 2,
        currentApprovals: 0,
      },
      {
        requestType: "BATCH_PROGRAM_UPDATE",
        targetId: foodProgram.id,
        requestData: JSON.stringify({
          action: "update_description",
          oldDescription: "Current description",
          newDescription: "Monthly food benefit for low-income families. Funds can be spent at grocery stores, supermarkets, bakeries, food markets, and select fast food restaurants. Alcohol, tobacco, and non-food items are restricted.",
          programId: foodProgram.id,
          programName: foodProgram.name,
        }),
        justification: "Updating program description to accurately reflect the current list of approved merchant categories and provide clear guidance to beneficiaries.",
        status: "approved",
        requestedBy: adminUser.id,
        requiredApprovals: 1,
        currentApprovals: 1,
      },
    ];

    console.log(`📊 Creating ${approvalRequests.length} approval requests\n`);

    let successCount = 0;
    const createdRequests = [];

    for (const request of approvalRequests) {
      try {
        const [result] = await connection.query(
          `INSERT INTO approval_requests 
           (request_type, target_id, request_data, justification, status, requested_by, required_approvals, current_approvals) 
           VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
          [
            request.requestType,
            request.targetId,
            request.requestData,
            request.justification,
            request.status,
            request.requestedBy,
            request.requiredApprovals,
            request.currentApprovals,
          ]
        );

        const requestId = result.insertId;
        createdRequests.push({ id: requestId, ...request });

        const statusIcon = request.status === "approved" ? "✅" : "⏳";
        console.log(`${statusIcon} Created ${request.requestType} request (ID: ${requestId})`);
        
        successCount++;
      } catch (error) {
        console.error(`❌ Failed to create approval request:`, error.message);
      }
    }

    // Create approval records for the approved request
    const approvedRequest = createdRequests.find(r => r.status === "approved");
    if (approvedRequest) {
      console.log(`\n📝 Creating approval record for approved request (ID: ${approvedRequest.id})`);
      
      await connection.query(
        `INSERT INTO approvals 
         (request_id, approved_by, decision, comment) 
         VALUES (?, ?, ?, ?)`,
        [
          approvedRequest.id,
          adminUser.id,
          "approved",
          "Approved: Description update accurately reflects current program configuration.",
        ]
      );
      
      console.log(`✓ Added approval from ${adminUser.name || adminUser.email}`);
    }

    console.log("\n" + "=".repeat(60));
    console.log("📈 Seeding Summary:");
    console.log("=".repeat(60));
    console.log(`✅ Successfully created: ${successCount} approval requests`);
    console.log("=".repeat(60));

    // Show breakdown by status
    const [breakdown] = await connection.query(`
      SELECT 
        status,
        COUNT(*) as count,
        GROUP_CONCAT(request_type SEPARATOR ', ') as types
      FROM approval_requests
      GROUP BY status
    `);

    console.log("\n📊 Approval Requests by Status:");
    console.log("=".repeat(60));
    breakdown.forEach(row => {
      const statusIcon = row.status === "approved" ? "✅" : row.status === "pending" ? "⏳" : "❌";
      console.log(`${statusIcon} ${row.status.toUpperCase()}: ${row.count} request(s)`);
      console.log(`   Types: ${row.types}`);
    });

    console.log("\n💡 Next Steps:");
    console.log("=".repeat(60));
    console.log("1. Navigate to 'Pending Approvals' in the Admin Portal");
    console.log("2. Review the pending approval requests");
    console.log("3. Approve or reject requests with comments");
    console.log("4. Check audit logs to see approval history");

    console.log("\n✨ Sample approval workflow seeding completed!");
  } catch (error) {
    console.error("\n❌ Fatal error during seeding:", error);
    process.exit(1);
  } finally {
    if (connection) {
      await connection.end();
    }
  }
}

seedApprovals().catch((error) => {
  console.error("❌ Unhandled error:", error);
  process.exit(1);
});
