import mysql from "mysql2/promise";

const DATABASE_URL = process.env.DATABASE_URL;

if (!DATABASE_URL) {
  console.error("❌ DATABASE_URL environment variable is not set");
  process.exit(1);
}

const samplePrograms = [
  {
    name: "Food Assistance Program",
    description: "Monthly food benefit for low-income families. Funds can only be spent at grocery stores, supermarkets, bakeries, and food markets. Alcohol, tobacco, and non-food items are restricted.",
    accountType: "FOOD_BENEFIT",
    status: "active",
    mccRules: [
      { mccCode: "5411", action: "allow", description: "Grocery Stores Supermarkets" },
      { mccCode: "5422", action: "allow", description: "Freezer and Locker Meat Provisioners" },
      { mccCode: "5441", action: "allow", description: "Candy Nut and Confectionery Stores" },
      { mccCode: "5451", action: "allow", description: "Dairy Products Stores" },
      { mccCode: "5462", action: "allow", description: "Bakeries" },
      { mccCode: "5499", action: "allow", description: "Miscellaneous Food Stores - Convenience Stores" },
      { mccCode: "5921", action: "deny", description: "Package Stores - Beer Wine and Liquor" },
      { mccCode: "5813", action: "deny", description: "Drinking Places - Bars Taverns Nightclubs" },
      { mccCode: "5993", action: "deny", description: "Cigar Stores and Stands" },
    ],
  },
  {
    name: "Health Benefits Program",
    description: "Healthcare assistance for medical expenses including doctor visits, prescriptions, dental care, and hospital services. Covers essential healthcare services and medical supplies.",
    accountType: "HEALTH_BENEFIT",
    status: "active",
    mccRules: [
      { mccCode: "5912", action: "allow", description: "Drug Stores and Pharmacies" },
      { mccCode: "8011", action: "allow", description: "Doctors and Physicians - Not Elsewhere Classified" },
      { mccCode: "8021", action: "allow", description: "Dentists and Orthodontists" },
      { mccCode: "8031", action: "allow", description: "Osteopaths" },
      { mccCode: "8041", action: "allow", description: "Chiropractors" },
      { mccCode: "8042", action: "allow", description: "Optometrists and Ophthalmologists" },
      { mccCode: "8043", action: "allow", description: "Opticians Optical Goods and Eyeglasses" },
      { mccCode: "8049", action: "allow", description: "Podiatrists and Chiropodists" },
      { mccCode: "8050", action: "allow", description: "Nursing and Personal Care Facilities" },
      { mccCode: "8062", action: "allow", description: "Hospitals" },
      { mccCode: "8071", action: "allow", description: "Medical and Dental Laboratories" },
      { mccCode: "8099", action: "allow", description: "Medical Services and Health Practitioners" },
      { mccCode: "5975", action: "allow", description: "Hearing Aids - Sales Service and Supply Stores" },
      { mccCode: "5976", action: "allow", description: "Orthopedic Goods - Prosthetic Devices" },
    ],
  },
  {
    name: "Education Support Program",
    description: "Educational assistance for students covering tuition, books, school supplies, and educational materials. Supports elementary through higher education expenses.",
    accountType: "EDUCATION_BENEFIT",
    status: "active",
    mccRules: [
      { mccCode: "8211", action: "allow", description: "Elementary and Secondary Schools" },
      { mccCode: "8220", action: "allow", description: "Colleges Junior Colleges Universities" },
      { mccCode: "8241", action: "allow", description: "Correspondence Schools" },
      { mccCode: "8244", action: "allow", description: "Business and Secretarial Schools" },
      { mccCode: "8249", action: "allow", description: "Vocational Schools and Trade Schools" },
      { mccCode: "8299", action: "allow", description: "Schools and Educational Services" },
      { mccCode: "5942", action: "allow", description: "Book Stores" },
      { mccCode: "5943", action: "allow", description: "Stationery Stores Office and School Supply Stores" },
      { mccCode: "5733", action: "allow", description: "Music Stores - Musical Instruments" },
      { mccCode: "5045", action: "allow", description: "Computers Peripherals and Software" },
      { mccCode: "5734", action: "allow", description: "Computer Software Stores" },
    ],
  },
];

async function seedPrograms() {
  console.log("🌱 Starting sample benefit programs seeding...\n");

  let connection;
  try {
    connection = await mysql.createConnection(DATABASE_URL);

    let successCount = 0;
    let errorCount = 0;

    for (const program of samplePrograms) {
      try {
        // Insert the benefit program
        const [result] = await connection.query(
          `INSERT INTO benefit_programs 
           (name, description, accountType, status) 
           VALUES (?, ?, ?, ?)`,
          [
            program.name,
            program.description,
            program.accountType,
            program.status,
          ]
        );

        const programId = result.insertId;
        console.log(`✓ Created program: ${program.name} (ID: ${programId})`);

        // Insert MCC rules for this program
        for (const rule of program.mccRules) {
          await connection.query(
            `INSERT INTO mcc_rules (programId, mccCode, mccDescription) 
             VALUES (?, ?, ?)`,
            [programId, rule.mccCode, rule.description]
          );
        }

        console.log(`  └─ Added ${program.mccRules.length} MCC rules\n`);
        successCount++;
      } catch (error) {
        console.error(`❌ Failed to create ${program.name}:`, error.message);
        errorCount++;
      }
    }

    console.log("=".repeat(60));
    console.log("📈 Seeding Summary:");
    console.log("=".repeat(60));
    console.log(`✅ Successfully created: ${successCount} programs`);
    console.log(`❌ Failed: ${errorCount} programs`);
    console.log("=".repeat(60));

    // Verify the programs
    const [programs] = await connection.query(
      "SELECT COUNT(*) as count FROM benefit_programs"
    );
    const [rules] = await connection.query(
      "SELECT COUNT(*) as count FROM mcc_rules"
    );

    console.log(`\n📊 Total benefit programs in database: ${programs[0].count}`);
    console.log(`📊 Total MCC rules in database: ${rules[0].count}`);

    console.log("\n✨ Sample benefit programs seeding completed!");
  } catch (error) {
    console.error("\n❌ Fatal error during seeding:", error);
    process.exit(1);
  } finally {
    if (connection) {
      await connection.end();
    }
  }
}

seedPrograms().catch((error) => {
  console.error("❌ Unhandled error:", error);
  process.exit(1);
});
