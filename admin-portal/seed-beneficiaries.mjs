import { drizzle } from "drizzle-orm/mysql2";
import mysql from "mysql2/promise";

const db = drizzle(process.env.DATABASE_URL);

const sampleBeneficiaries = [
  // Approved beneficiaries
  { firstName: "Amina", lastName: "Ibrahim", nationalId: "12345678901", email: "amina.ibrahim@example.com", phoneNumber: "+2348012345001", city: "Lagos", state: "Lagos", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Chidi", lastName: "Okafor", nationalId: "12345678902", email: "chidi.okafor@example.com", phoneNumber: "+2348012345002", city: "Enugu", state: "Enugu", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Fatima", lastName: "Bello", nationalId: "12345678903", email: "fatima.bello@example.com", phoneNumber: "+2348012345003", city: "Kano", state: "Kano", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Emeka", lastName: "Nwosu", nationalId: "12345678904", email: "emeka.nwosu@example.com", phoneNumber: "+2348012345004", city: "Aba", state: "Abia", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Zainab", lastName: "Mohammed", nationalId: "12345678905", email: "zainab.mohammed@example.com", phoneNumber: "+2348012345005", city: "Abuja", state: "FCT", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Tunde", lastName: "Adeyemi", nationalId: "12345678906", email: "tunde.adeyemi@example.com", phoneNumber: "+2348012345006", city: "Ibadan", state: "Oyo", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Blessing", lastName: "Okonkwo", nationalId: "12345678907", email: "blessing.okonkwo@example.com", phoneNumber: "+2348012345007", city: "Port Harcourt", state: "Rivers", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Yusuf", lastName: "Abubakar", nationalId: "12345678908", email: "yusuf.abubakar@example.com", phoneNumber: "+2348012345008", city: "Kaduna", state: "Kaduna", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Chioma", lastName: "Eze", nationalId: "12345678909", email: "chioma.eze@example.com", phoneNumber: "+2348012345009", city: "Onitsha", state: "Anambra", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Ibrahim", lastName: "Musa", nationalId: "12345678910", email: "ibrahim.musa@example.com", phoneNumber: "+2348012345010", city: "Jos", state: "Plateau", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Grace", lastName: "Okoro", nationalId: "12345678911", email: "grace.okoro@example.com", phoneNumber: "+2348012345011", city: "Calabar", state: "Cross River", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Ahmed", lastName: "Suleiman", nationalId: "12345678912", email: "ahmed.suleiman@example.com", phoneNumber: "+2348012345012", city: "Maiduguri", state: "Borno", enrollmentStatus: "approved", kycStatus: "completed" },
  
  // Pending beneficiaries
  { firstName: "Ngozi", lastName: "Chukwu", nationalId: "12345678913", email: "ngozi.chukwu@example.com", phoneNumber: "+2348012345013", city: "Awka", state: "Anambra", enrollmentStatus: "pending", kycStatus: "in_progress" },
  { firstName: "Musa", lastName: "Aliyu", nationalId: "12345678914", email: "musa.aliyu@example.com", phoneNumber: "+2348012345014", city: "Sokoto", state: "Sokoto", enrollmentStatus: "pending", kycStatus: "in_progress" },
  { firstName: "Adaeze", lastName: "Nnamdi", nationalId: "12345678915", email: "adaeze.nnamdi@example.com", phoneNumber: "+2348012345015", city: "Owerri", state: "Imo", enrollmentStatus: "pending", kycStatus: "not_started" },
  { firstName: "Abdullahi", lastName: "Garba", nationalId: "12345678916", email: "abdullahi.garba@example.com", phoneNumber: "+2348012345016", city: "Katsina", state: "Katsina", enrollmentStatus: "pending", kycStatus: "in_progress" },
  { firstName: "Ifeoma", lastName: "Obi", nationalId: "12345678917", email: "ifeoma.obi@example.com", phoneNumber: "+2348012345017", city: "Nsukka", state: "Enugu", enrollmentStatus: "pending", kycStatus: "not_started" },
  { firstName: "Sani", lastName: "Baba", nationalId: "12345678918", email: "sani.baba@example.com", phoneNumber: "+2348012345018", city: "Bauchi", state: "Bauchi", enrollmentStatus: "pending", kycStatus: "in_progress" },
  
  // Rejected beneficiaries
  { firstName: "Adeola", lastName: "Fashola", nationalId: "12345678919", email: "adeola.fashola@example.com", phoneNumber: "+2348012345019", city: "Lagos", state: "Lagos", enrollmentStatus: "rejected", kycStatus: "failed" },
  { firstName: "Usman", lastName: "Yaro", nationalId: "12345678920", email: "usman.yaro@example.com", phoneNumber: "+2348012345020", city: "Gombe", state: "Gombe", enrollmentStatus: "rejected", kycStatus: "failed" },
  { firstName: "Chiamaka", lastName: "Udeh", nationalId: "12345678921", email: "chiamaka.udeh@example.com", phoneNumber: "+2348012345021", city: "Umuahia", state: "Abia", enrollmentStatus: "rejected", kycStatus: "failed" },
  
  // Suspended beneficiaries
  { firstName: "Bashir", lastName: "Lawal", nationalId: "12345678922", email: "bashir.lawal@example.com", phoneNumber: "+2348012345022", city: "Ilorin", state: "Kwara", enrollmentStatus: "suspended", kycStatus: "completed" },
  { firstName: "Nneka", lastName: "Okafor", nationalId: "12345678923", email: "nneka.okafor@example.com", phoneNumber: "+2348012345023", city: "Abakaliki", state: "Ebonyi", enrollmentStatus: "suspended", kycStatus: "completed" },
  { firstName: "Aliyu", lastName: "Tanko", nationalId: "12345678924", email: "aliyu.tanko@example.com", phoneNumber: "+2348012345024", city: "Zaria", state: "Kaduna", enrollmentStatus: "suspended", kycStatus: "completed" },
  
  // Additional approved
  { firstName: "Oluwaseun", lastName: "Ajayi", nationalId: "12345678925", email: "oluwaseun.ajayi@example.com", phoneNumber: "+2348012345025", city: "Akure", state: "Ondo", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Halima", lastName: "Umar", nationalId: "12345678926", email: "halima.umar@example.com", phoneNumber: "+2348012345026", city: "Yola", state: "Adamawa", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Chinedu", lastName: "Obi", nationalId: "12345678927", email: "chinedu.obi@example.com", phoneNumber: "+2348012345027", city: "Asaba", state: "Delta", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Aisha", lastName: "Kabir", nationalId: "12345678928", email: "aisha.kabir@example.com", phoneNumber: "+2348012345028", city: "Dutse", state: "Jigawa", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Obinna", lastName: "Nwankwo", nationalId: "12345678929", email: "obinna.nwankwo@example.com", phoneNumber: "+2348012345029", city: "Warri", state: "Delta", enrollmentStatus: "approved", kycStatus: "completed" },
  { firstName: "Hauwa", lastName: "Sani", nationalId: "12345678930", email: "hauwa.sani@example.com", phoneNumber: "+2348012345030", city: "Lafia", state: "Nasarawa", enrollmentStatus: "approved", kycStatus: "completed" },
];

async function seedBeneficiaries() {
  console.log("Starting beneficiary seeding...");
  
  const connection = await mysql.createConnection(process.env.DATABASE_URL);
  
  try {
    // Clear existing beneficiary data
    console.log("Clearing existing beneficiary data...");
    await connection.query("DELETE FROM kyc_documents");
    await connection.query("DELETE FROM benefit_cards");
    await connection.query("DELETE FROM program_enrollments");
    await connection.query("DELETE FROM beneficiaries");
    console.log("Existing data cleared.");
    
    // Get existing programs
    const [programs] = await connection.query("SELECT id, name FROM benefit_programs LIMIT 3");
    
    if (programs.length === 0) {
      console.error("No programs found. Please seed programs first.");
      process.exit(1);
    }
    
    console.log(`Found ${programs.length} programs to assign beneficiaries to`);
    
    let beneficiaryCount = 0;
    let enrollmentCount = 0;
    let documentCount = 0;
    let cardCount = 0;
    
    for (const beneficiary of sampleBeneficiaries) {
      const now = new Date();
      const enrolledAt = new Date(now.getTime() - Math.random() * 90 * 24 * 60 * 60 * 1000); // Random date in last 90 days
      
      let approvedAt = null;
      let approvedBy = null;
      if (beneficiary.enrollmentStatus === "approved") {
        approvedAt = new Date(enrolledAt.getTime() + Math.random() * 7 * 24 * 60 * 60 * 1000); // Approved within 7 days
        approvedBy = 1; // Admin user ID
      }
      
      // Generate random date of birth (18-45 years old) - use timestamp format
      // Avoiding dates before 1970 to prevent timestamp range issues
      const age = Math.floor(Math.random() * 27) + 18; // 18-45 years old
      const dobYear = now.getFullYear() - age;
      const dobMonth = Math.floor(Math.random() * 12);
      const dobDay = Math.floor(Math.random() * 28) + 1;
      const dateOfBirth = new Date(dobYear, dobMonth, dobDay);
      
      // Insert beneficiary
      const [result] = await connection.query(
        `INSERT INTO beneficiaries 
        (first_name, last_name, date_of_birth, national_id, email, phone_number, city, state, enrollment_status, kyc_status, enrolled_at, approved_at, approved_by, created_at, updated_at) 
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`,
        [
          beneficiary.firstName,
          beneficiary.lastName,
          dateOfBirth,
          beneficiary.nationalId,
          beneficiary.email,
          beneficiary.phoneNumber,
          beneficiary.city,
          beneficiary.state,
          beneficiary.enrollmentStatus,
          beneficiary.kycStatus,
          enrolledAt,
          approvedAt,
          approvedBy,
        ]
      );
      
      const beneficiaryId = result.insertId;
      beneficiaryCount++;
      
      // Create program enrollments for approved beneficiaries
      if (beneficiary.enrollmentStatus === "approved") {
        // Enroll in 1-2 random programs
        const numPrograms = Math.floor(Math.random() * 2) + 1;
        const selectedPrograms = programs.sort(() => 0.5 - Math.random()).slice(0, numPrograms);
        
        for (const program of selectedPrograms) {
          // Random monthly allocation between 5000 and 50000 NGN
          const monthlyAllocation = Math.floor(Math.random() * 45000) + 5000;
          
          await connection.query(
            `INSERT INTO program_enrollments 
            (beneficiary_id, program_id, enrollment_date, monthly_allocation, status, created_at, updated_at) 
            VALUES (?, ?, ?, ?, 'active', NOW(), NOW())`,
            [beneficiaryId, program.id, enrolledAt, monthlyAllocation]
          );
          enrollmentCount++;
          
          // Create benefit card for each enrollment
          const cardNumber = `5200${String(beneficiaryId).padStart(4, '0')}${String(program.id).padStart(4, '0')}${Math.floor(Math.random() * 10000).toString().padStart(4, '0')}`;
          const expiryDate = new Date(now.getFullYear() + 3, now.getMonth(), 1); // 3 years from now
          const cardType = Math.random() > 0.3 ? 'physical' : 'virtual';
          
          await connection.query(
            `INSERT INTO benefit_cards 
            (beneficiary_id, card_number, card_type, status, issued_at, expires_at, activated_at, created_at, updated_at) 
            VALUES (?, ?, ?, 'active', ?, ?, ?, NOW(), NOW())`,
            [beneficiaryId, cardNumber, cardType, enrolledAt, expiryDate, enrolledAt]
          );
          cardCount++;
        }
      }
      
      // Create KYC documents
      if (beneficiary.kycStatus === "completed" || beneficiary.kycStatus === "in_progress") {
        const documentTypes = ["national_id", "proof_of_address", "photo"];
        const numDocs = beneficiary.kycStatus === "completed" ? 3 : Math.floor(Math.random() * 2) + 1;
        
        for (let i = 0; i < numDocs; i++) {
          const docType = documentTypes[i];
          const docStatus = beneficiary.kycStatus === "completed" ? "verified" : (Math.random() > 0.5 ? "pending" : "verified");
          
          const fileName = `${docType}.pdf`;
          const fileKey = `kyc/${beneficiaryId}/${fileName}`;
          
          await connection.query(
            `INSERT INTO kyc_documents 
            (beneficiary_id, document_type, file_name, file_key, document_url, verification_status, uploaded_at) 
            VALUES (?, ?, ?, ?, ?, ?, ?)`,
            [
              beneficiaryId,
              docType,
              fileName,
              fileKey,
              `https://storage.example.com/${fileKey}`,
              docStatus,
              enrolledAt,
            ]
          );
          documentCount++;
        }
      }
    }
    
    console.log(`✅ Seeded ${beneficiaryCount} beneficiaries`);
    console.log(`✅ Created ${enrollmentCount} program enrollments`);
    console.log(`✅ Created ${cardCount} benefit cards`);
    console.log(`✅ Created ${documentCount} KYC documents`);
    
  } catch (error) {
    console.error("Error seeding beneficiaries:", error);
    throw error;
  } finally {
    await connection.end();
  }
}

seedBeneficiaries()
  .then(() => {
    console.log("Beneficiary seeding completed successfully!");
    process.exit(0);
  })
  .catch((error) => {
    console.error("Beneficiary seeding failed:", error);
    process.exit(1);
  });
