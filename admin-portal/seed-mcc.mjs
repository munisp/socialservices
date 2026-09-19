import { drizzle } from "drizzle-orm/mysql2";
import mysql from "mysql2/promise";
import fs from "fs";
import { parse } from "csv-parse/sync";
// Import not needed - we'll use raw SQL

const DATABASE_URL = process.env.DATABASE_URL;

if (!DATABASE_URL) {
  console.error("❌ DATABASE_URL environment variable is not set");
  process.exit(1);
}

async function seedMCC() {
  console.log("🌱 Starting MCC database seeding...\n");

  let connection;
  try {
    connection = await mysql.createConnection(DATABASE_URL);
    const db = drizzle(connection);

    // Read CSV file
    const csvContent = fs.readFileSync("mcc-seed-data.csv", "utf-8");
    const records = parse(csvContent, {
      columns: true,
      skip_empty_lines: true,
    });

    console.log(`📊 Found ${records.length} MCC codes in CSV file\n`);

    let successCount = 0;
    let errorCount = 0;
    const errors = [];

    for (const record of records) {
      try {
        await connection.query(
          `INSERT INTO mcc_database (mccCode, description, category) 
           VALUES (?, ?, ?) 
           ON DUPLICATE KEY UPDATE 
           description = VALUES(description), 
           category = VALUES(category)`,
          [record.mccCode, record.description, record.category]
        );
        successCount++;
        
        // Show progress every 50 records
        if (successCount % 50 === 0) {
          console.log(`✓ Imported ${successCount} codes...`);
        }
      } catch (error) {
        errorCount++;
        errors.push({
          code: record.mccCode,
          error: error.message,
        });
      }
    }

    console.log("\n" + "=".repeat(60));
    console.log("📈 Import Summary:");
    console.log("=".repeat(60));
    console.log(`✅ Successfully imported: ${successCount} codes`);
    console.log(`❌ Failed imports: ${errorCount} codes`);
    
    if (errors.length > 0 && errors.length <= 10) {
      console.log("\n⚠️  Errors:");
      errors.forEach(({ code, error }) => {
        console.log(`   - ${code}: ${error}`);
      });
    } else if (errors.length > 10) {
      console.log(`\n⚠️  ${errors.length} errors occurred (showing first 10):`);
      errors.slice(0, 10).forEach(({ code, error }) => {
        console.log(`   - ${code}: ${error}`);
      });
    }

    console.log("=".repeat(60));
    console.log("\n✨ MCC database seeding completed!");

    // Verify the import
    const [result] = await connection.query("SELECT COUNT(*) as count FROM mcc_database");
    console.log(`\n📊 Total MCC codes in database: ${result[0].count}`);

  } catch (error) {
    console.error("\n❌ Fatal error during seeding:", error);
    process.exit(1);
  } finally {
    if (connection) {
      await connection.end();
    }
  }
}

seedMCC().catch((error) => {
  console.error("❌ Unhandled error:", error);
  process.exit(1);
});
