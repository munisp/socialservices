import mysql from "mysql2/promise";

const DATABASE_URL = process.env.DATABASE_URL;

if (!DATABASE_URL) {
  console.error("❌ DATABASE_URL environment variable is not set");
  process.exit(1);
}

const programTemplates = [
  {
    name: "Standard Food Assistance",
    description: "Pre-configured template for standard food assistance programs. Includes approved MCC codes for grocery stores, supermarkets, bakeries, and food markets. Excludes alcohol, tobacco, and non-food items.",
    accountType: "FOOD_BENEFIT",
    mccRules: [
      { mccCode: "5411", mccDescription: "Grocery Stores Supermarkets" },
      { mccCode: "5422", mccDescription: "Freezer and Locker Meat Provisioners" },
      { mccCode: "5441", mccDescription: "Candy Nut and Confectionery Stores" },
      { mccCode: "5451", mccDescription: "Dairy Products Stores" },
      { mccCode: "5462", mccDescription: "Bakeries" },
      { mccCode: "5499", mccDescription: "Miscellaneous Food Stores - Convenience Stores" },
    ],
  },
  {
    name: "Emergency Health Relief",
    description: "Rapid-deployment template for emergency healthcare assistance. Covers essential medical services including emergency care, prescriptions, urgent care facilities, and ambulance services.",
    accountType: "HEALTH_BENEFIT",
    mccRules: [
      { mccCode: "5912", mccDescription: "Drug Stores and Pharmacies" },
      { mccCode: "8011", mccDescription: "Doctors and Physicians - Not Elsewhere Classified" },
      { mccCode: "8021", mccDescription: "Dentists and Orthodontists" },
      { mccCode: "8041", mccDescription: "Chiropractors" },
      { mccCode: "8042", mccDescription: "Optometrists and Ophthalmologists" },
      { mccCode: "8043", mccDescription: "Opticians Optical Goods and Eyeglasses" },
      { mccCode: "8062", mccDescription: "Hospitals" },
      { mccCode: "8099", mccDescription: "Medical Services and Health Practitioners" },
      { mccCode: "4119", mccDescription: "Ambulance Services" },
    ],
  },
  {
    name: "Education Grant - K-12",
    description: "Template for elementary and secondary education support programs. Includes school tuition, books, supplies, uniforms, and educational materials. Suitable for K-12 education assistance.",
    accountType: "EDUCATION_BENEFIT",
    mccRules: [
      { mccCode: "8211", mccDescription: "Elementary and Secondary Schools" },
      { mccCode: "5942", mccDescription: "Book Stores" },
      { mccCode: "5943", mccDescription: "Stationery Stores Office and School Supply Stores" },
      { mccCode: "5651", mccDescription: "Family Clothing Stores" },
      { mccCode: "5641", mccDescription: "Childrens and Infants Wear Stores" },
    ],
  },
  {
    name: "Higher Education Support",
    description: "Template for college and university education assistance. Covers tuition, textbooks, computers, software, and educational technology. Designed for post-secondary education programs.",
    accountType: "EDUCATION_BENEFIT",
    mccRules: [
      { mccCode: "8220", mccDescription: "Colleges Junior Colleges Universities" },
      { mccCode: "8244", mccDescription: "Business and Secretarial Schools" },
      { mccCode: "8249", mccDescription: "Vocational Schools and Trade Schools" },
      { mccCode: "5942", mccDescription: "Book Stores" },
      { mccCode: "5943", mccDescription: "Stationery Stores Office and School Supply Stores" },
      { mccCode: "5045", mccDescription: "Computers Peripherals and Software" },
      { mccCode: "5734", mccDescription: "Computer Software Stores" },
    ],
  },
  {
    name: "Comprehensive Healthcare",
    description: "Full-spectrum healthcare template covering preventive care, specialist visits, dental, vision, mental health, and medical equipment. Ideal for comprehensive health benefit programs.",
    accountType: "HEALTH_BENEFIT",
    mccRules: [
      { mccCode: "5912", mccDescription: "Drug Stores and Pharmacies" },
      { mccCode: "8011", mccDescription: "Doctors and Physicians - Not Elsewhere Classified" },
      { mccCode: "8021", mccDescription: "Dentists and Orthodontists" },
      { mccCode: "8031", mccDescription: "Osteopaths" },
      { mccCode: "8041", mccDescription: "Chiropractors" },
      { mccCode: "8042", mccDescription: "Optometrists and Ophthalmologists" },
      { mccCode: "8043", mccDescription: "Opticians Optical Goods and Eyeglasses" },
      { mccCode: "8049", mccDescription: "Podiatrists and Chiropodists" },
      { mccCode: "8050", mccDescription: "Nursing and Personal Care Facilities" },
      { mccCode: "8062", mccDescription: "Hospitals" },
      { mccCode: "8071", mccDescription: "Medical and Dental Laboratories" },
      { mccCode: "8099", mccDescription: "Medical Services and Health Practitioners" },
      { mccCode: "5975", mccDescription: "Hearing Aids - Sales Service and Supply Stores" },
      { mccCode: "5976", mccDescription: "Orthopedic Goods - Prosthetic Devices" },
    ],
  },
];

async function seedTemplates() {
  console.log("🌱 Starting program templates seeding...\n");

  let connection;
  try {
    connection = await mysql.createConnection(DATABASE_URL);

    // Get admin user
    const [users] = await connection.query(
      "SELECT id, name FROM users WHERE role = 'admin' LIMIT 1"
    );

    if (users.length === 0) {
      console.error("❌ No admin user found. Please ensure you're logged in as admin first.");
      process.exit(1);
    }

    const adminUser = users[0];
    console.log(`👤 Creating templates as: ${adminUser.name} (ID: ${adminUser.id})\n`);
    console.log(`📊 Creating ${programTemplates.length} program templates\n`);

    let successCount = 0;
    let errorCount = 0;

    for (const template of programTemplates) {
      try {
        await connection.query(
          `INSERT INTO program_templates 
           (name, description, accountType, mccRules, createdBy) 
           VALUES (?, ?, ?, ?, ?)
           ON DUPLICATE KEY UPDATE 
           description = VALUES(description),
           accountType = VALUES(accountType),
           mccRules = VALUES(mccRules)`,
          [
            template.name,
            template.description,
            template.accountType,
            JSON.stringify(template.mccRules),
            adminUser.id,
          ]
        );

        console.log(`✓ Created template: ${template.name}`);
        console.log(`  Account Type: ${template.accountType}`);
        console.log(`  MCC Rules: ${template.mccRules.length} codes\n`);
        
        successCount++;
      } catch (error) {
        console.error(`❌ Failed to create ${template.name}:`, error.message);
        errorCount++;
      }
    }

    console.log("=".repeat(60));
    console.log("📈 Seeding Summary:");
    console.log("=".repeat(60));
    console.log(`✅ Successfully created: ${successCount} templates`);
    console.log(`❌ Failed: ${errorCount} templates`);
    console.log("=".repeat(60));

    // Show breakdown by account type
    const [breakdown] = await connection.query(`
      SELECT 
        accountType,
        COUNT(*) as template_count,
        GROUP_CONCAT(name SEPARATOR ', ') as template_names
      FROM program_templates
      GROUP BY accountType
    `);

    console.log("\n📊 Templates by Account Type:");
    console.log("=".repeat(60));
    breakdown.forEach(row => {
      console.log(`\n${row.accountType}:`);
      console.log(`  Count: ${row.template_count}`);
      console.log(`  Templates: ${row.template_names}`);
    });

    console.log("\n💡 Usage:");
    console.log("=".repeat(60));
    console.log("1. Navigate to 'Program Templates' in the Admin Portal");
    console.log("2. Select a template to view its pre-configured MCC rules");
    console.log("3. Click 'Use Template' to create a new program");
    console.log("4. Customize the program name and settings as needed");

    console.log("\n✨ Program templates seeding completed!");
  } catch (error) {
    console.error("\n❌ Fatal error during seeding:", error);
    process.exit(1);
  } finally {
    if (connection) {
      await connection.end();
    }
  }
}

seedTemplates().catch((error) => {
  console.error("❌ Unhandled error:", error);
  process.exit(1);
});
