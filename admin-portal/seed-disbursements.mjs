import mysql from "mysql2/promise";

const DATABASE_URL = process.env.DATABASE_URL;

if (!DATABASE_URL) {
  console.error("❌ DATABASE_URL environment variable is not set");
  process.exit(1);
}

// Helper to generate future dates
function addMonths(date, months) {
  const result = new Date(date);
  result.setMonth(result.getMonth() + months);
  return result;
}

function addDays(date, days) {
  const result = new Date(date);
  result.setDate(result.getDate() + days);
  return result;
}

async function seedDisbursements() {
  console.log("🌱 Starting disbursement schedules seeding...\n");

  let connection;
  try {
    connection = await mysql.createConnection(DATABASE_URL);

    // Get program IDs
    const [programs] = await connection.query(
      "SELECT id, name, accountType FROM benefit_programs ORDER BY id"
    );

    if (programs.length === 0) {
      console.error("❌ No benefit programs found. Please run seed-programs.mjs first.");
      process.exit(1);
    }

    console.log(`📊 Found ${programs.length} benefit programs\n`);

    const today = new Date();
    const schedules = [];

    // Food Assistance Program - Monthly disbursements
    const foodProgram = programs.find(p => p.accountType === "FOOD_BENEFIT");
    if (foodProgram) {
      console.log(`📅 Creating monthly schedules for: ${foodProgram.name}`);
      for (let i = 0; i < 6; i++) {
        const scheduledDate = addMonths(today, i);
        schedules.push({
          programId: foodProgram.id,
          scheduledDate,
          amount: 25000000, // 250,000 in smallest unit (kobo/cents)
          beneficiaryCount: 1500 + Math.floor(Math.random() * 200),
          status: i === 0 ? "processing" : "pending",
          metadata: JSON.stringify({
            frequency: "monthly",
            programType: "food_assistance",
            notes: `Month ${i + 1} disbursement for food benefits`,
          }),
        });
      }
    }

    // Health Benefits Program - Monthly disbursements
    const healthProgram = programs.find(p => p.accountType === "HEALTH_BENEFIT");
    if (healthProgram) {
      console.log(`📅 Creating monthly schedules for: ${healthProgram.name}`);
      for (let i = 0; i < 6; i++) {
        const scheduledDate = addMonths(today, i);
        // Add 5 days offset from food program
        scheduledDate.setDate(scheduledDate.getDate() + 5);
        
        schedules.push({
          programId: healthProgram.id,
          scheduledDate,
          amount: 50000000, // 500,000 in smallest unit
          beneficiaryCount: 800 + Math.floor(Math.random() * 150),
          status: i === 0 ? "processing" : "pending",
          metadata: JSON.stringify({
            frequency: "monthly",
            programType: "health_benefits",
            notes: `Month ${i + 1} disbursement for healthcare expenses`,
          }),
        });
      }
    }

    // Education Support Program - Quarterly disbursements
    const educationProgram = programs.find(p => p.accountType === "EDUCATION_BENEFIT");
    if (educationProgram) {
      console.log(`📅 Creating quarterly schedules for: ${educationProgram.name}`);
      for (let i = 0; i < 4; i++) {
        const scheduledDate = addMonths(today, i * 3);
        // Schedule at beginning of quarter
        scheduledDate.setDate(1);
        
        schedules.push({
          programId: educationProgram.id,
          scheduledDate,
          amount: 75000000, // 750,000 in smallest unit
          beneficiaryCount: 500 + Math.floor(Math.random() * 100),
          status: i === 0 ? "processing" : "pending",
          metadata: JSON.stringify({
            frequency: "quarterly",
            programType: "education_support",
            quarter: `Q${i + 1}`,
            notes: `Quarter ${i + 1} disbursement for education expenses`,
          }),
        });
      }
    }

    console.log(`\n📊 Total schedules to create: ${schedules.length}\n`);

    let successCount = 0;
    let errorCount = 0;

    for (const schedule of schedules) {
      try {
        await connection.query(
          `INSERT INTO disbursement_schedules 
           (programId, scheduledDate, amount, beneficiaryCount, status, metadata) 
           VALUES (?, ?, ?, ?, ?, ?)`,
          [
            schedule.programId,
            schedule.scheduledDate,
            schedule.amount,
            schedule.beneficiaryCount,
            schedule.status,
            schedule.metadata,
          ]
        );
        successCount++;
        
        if (successCount % 5 === 0) {
          console.log(`✓ Created ${successCount} schedules...`);
        }
      } catch (error) {
        console.error(`❌ Failed to create schedule:`, error.message);
        errorCount++;
      }
    }

    console.log("\n" + "=".repeat(60));
    console.log("📈 Seeding Summary:");
    console.log("=".repeat(60));
    console.log(`✅ Successfully created: ${successCount} schedules`);
    console.log(`❌ Failed: ${errorCount} schedules`);
    console.log("=".repeat(60));

    // Show breakdown by program
    const [breakdown] = await connection.query(`
      SELECT 
        bp.name,
        bp.accountType,
        COUNT(ds.id) as schedule_count,
        SUM(ds.amount) as total_amount,
        SUM(ds.beneficiaryCount) as total_beneficiaries
      FROM benefit_programs bp
      LEFT JOIN disbursement_schedules ds ON bp.id = ds.programId
      GROUP BY bp.id, bp.name, bp.accountType
    `);

    console.log("\n📊 Disbursement Breakdown by Program:");
    console.log("=".repeat(60));
    breakdown.forEach(row => {
      console.log(`\n${row.name}:`);
      console.log(`  Schedules: ${row.schedule_count}`);
      console.log(`  Total Amount: ${(row.total_amount / 100).toLocaleString()} (in currency units)`);
      console.log(`  Total Beneficiaries: ${row.total_beneficiaries?.toLocaleString() || 0}`);
    });

    console.log("\n✨ Disbursement schedules seeding completed!");
  } catch (error) {
    console.error("\n❌ Fatal error during seeding:", error);
    process.exit(1);
  } finally {
    if (connection) {
      await connection.end();
    }
  }
}

seedDisbursements().catch((error) => {
  console.error("❌ Unhandled error:", error);
  process.exit(1);
});
