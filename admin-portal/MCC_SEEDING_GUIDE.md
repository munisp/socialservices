# MCC Database Seeding Guide

This guide explains how to populate the Admin Portal's MCC (Merchant Category Code) database with the standard ISO 18245 codes.

---

## Overview

The Admin Portal includes a comprehensive CSV file (`mcc-seed-data.csv`) containing **450+ standard Merchant Category Codes** from the ISO 18245 standard. These codes are essential for implementing earmarked spending controls in social benefit programs.

---

## MCC Categories Included

The seed data includes codes across all major categories:

- **Airlines** (3000-3100) - Major international and domestic carriers
- **Transportation** (4011-4789) - Railroads, buses, taxis, freight, airports
- **Utilities** (4900) - Electric, gas, water, sanitary services
- **Retail** (5000-5999) - All retail categories including:
  - Grocery stores and supermarkets (5411)
  - Pharmacies (5912)
  - Department stores (5311)
  - Gas stations (5541-5542)
  - Clothing stores (5611-5699)
  - Electronics (5732)
  - Books and music (5942, 5735)
- **Food & Beverage** (5811-5921) - Restaurants, bars, liquor stores
- **Automotive** (5511-5599, 7511-7549) - Dealers, parts, service, repairs
- **Healthcare** (8011-8099) - Doctors, dentists, hospitals, medical services
- **Education** (8211-8299) - Schools, colleges, vocational training
- **Financial Services** (6000-6399) - Banks, insurance, securities
- **Lodging** (7011-7033) - Hotels, motels, resorts, campgrounds
- **Recreation** (7032-7999) - Sports, entertainment, amusement
- **Professional Services** (7311-8999) - Legal, accounting, consulting
- **Government** (9211-9405) - Taxes, fines, postal services
- **Non-Profit** (8398-8699) - Charitable, religious, civic organizations

---

## Seeding Methods

### Method 1: Bulk Import via Admin Portal UI (Recommended)

This is the easiest method and includes validation and preview.

**Steps:**

1. **Log in to Admin Portal** as an administrator
2. **Navigate to "Bulk MCC Import"** from the sidebar menu
3. **Upload the CSV file:**
   - Click "Choose File" or drag and drop `mcc-seed-data.csv`
   - The file is located in the admin portal root directory
4. **Preview the data:**
   - Review the first 50 entries to verify format
   - Check for any validation errors
5. **Import:**
   - Click "Import MCC Codes" button
   - Wait for confirmation message
   - View import summary (success count, any errors)

**Validation:**
- The system validates each row for required fields
- Duplicate MCC codes are automatically skipped
- Invalid entries are reported with line numbers

### Method 2: Direct Database Import

For advanced users who prefer direct database access.

**Using MySQL Client:**

```sql
-- Connect to your database
mysql -u username -p database_name

-- Load the CSV file
LOAD DATA LOCAL INFILE '/path/to/mcc-seed-data.csv'
INTO TABLE mccDatabase
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 ROWS
(mccCode, description, category);
```

**Using Database Management Tool:**
- Open your preferred database management tool (phpMyAdmin, DBeaver, etc.)
- Navigate to the `mccDatabase` table
- Use the import CSV feature
- Map columns: mccCode → mccCode, description → description, category → category
- Skip the header row
- Execute import

### Method 3: Programmatic Seeding Script

Create a Node.js script to seed the database programmatically.

**Create `seed-mcc.mjs`:**

```javascript
import { drizzle } from "drizzle-orm/mysql2";
import mysql from "mysql2/promise";
import fs from "fs";
import { parse } from "csv-parse/sync";

const DATABASE_URL = process.env.DATABASE_URL || "mysql://user:pass@localhost:3306/dbname";

async function seedMCC() {
  const connection = await mysql.createConnection(DATABASE_URL);
  const db = drizzle(connection);

  // Read CSV file
  const csvContent = fs.readFileSync("mcc-seed-data.csv", "utf-8");
  const records = parse(csvContent, {
    columns: true,
    skip_empty_lines: true,
  });

  console.log(`Importing ${records.length} MCC codes...`);

  let successCount = 0;
  let errorCount = 0;

  for (const record of records) {
    try {
      await db.insert(mccDatabase).values({
        mccCode: record.mccCode,
        description: record.description,
        category: record.category,
      }).onDuplicateKeyUpdate({
        set: {
          description: record.description,
          category: record.category,
        },
      });
      successCount++;
    } catch (error) {
      console.error(`Failed to import ${record.mccCode}:`, error.message);
      errorCount++;
    }
  }

  console.log(`Import complete: ${successCount} successful, ${errorCount} errors`);
  await connection.end();
}

seedMCC().catch(console.error);
```

**Run the script:**

```bash
node seed-mcc.mjs
```

---

## CSV File Format

The CSV file follows this format:

```csv
mccCode,description,category
0742,Veterinary Services,Services
5411,Grocery Stores Supermarkets,Food & Beverage
5912,Drug Stores and Pharmacies,Healthcare
```

**Fields:**
- `mccCode` - 4-digit merchant category code (string)
- `description` - Human-readable description of the category
- `category` - High-level category grouping (e.g., "Food & Beverage", "Healthcare")

---

## Verification

After seeding, verify the import was successful:

### Via Admin Portal UI:

1. Navigate to "MCC Database Management"
2. Search for specific codes (e.g., 5411 for grocery stores)
3. Check the total count (should be 450+ entries)

### Via Database Query:

```sql
-- Count total MCC codes
SELECT COUNT(*) FROM mccDatabase;

-- View sample entries
SELECT * FROM mccDatabase LIMIT 10;

-- Check specific categories
SELECT category, COUNT(*) as count 
FROM mccDatabase 
GROUP BY category 
ORDER BY count DESC;
```

### Via API:

```bash
# Get MCC codes (requires authentication)
curl -X GET "http://localhost:3000/api/trpc/mcc.list" \
  -H "Cookie: your-session-cookie"
```

---

## Using MCC Codes for Earmarked Spending

Once the MCC database is populated, you can create benefit programs with spending restrictions:

**Example: Food Assistance Program**

1. Create a new benefit program
2. Set benefit type to "FOOD_BENEFIT"
3. Add MCC rules:
   - **Allow:** 5411 (Grocery Stores), 5422 (Meat Provisioners), 5462 (Bakeries)
   - **Deny:** 5921 (Liquor Stores), 5813 (Bars), 5993 (Cigar Stores)

**Example: Health Benefit Program**

1. Create a new benefit program
2. Set benefit type to "HEALTH_BENEFIT"
3. Add MCC rules:
   - **Allow:** 5912 (Pharmacies), 8011 (Doctors), 8062 (Hospitals), 8021 (Dentists)
   - **Deny:** All other categories

**Example: Education Benefit Program**

1. Create a new benefit program
2. Set benefit type to "EDUCATION_BENEFIT"
3. Add MCC rules:
   - **Allow:** 8211 (Schools), 8220 (Colleges), 5942 (Book Stores), 5733 (Music Stores)
   - **Deny:** Entertainment and recreation categories

---

## Updating MCC Codes

To update existing MCC codes or add new ones:

### Via Admin Portal:

1. Navigate to "MCC Database Management"
2. Search for the code you want to update
3. Click "Edit" button
4. Modify description or category
5. Save changes

### Via Bulk Import:

1. Prepare a CSV file with updated codes
2. Use the Bulk Import feature
3. The system will update existing codes and add new ones

---

## Common MCC Codes for Social Programs

Here are the most commonly used MCC codes for social benefit programs:

**Food Assistance:**
- 5411 - Grocery Stores, Supermarkets
- 5422 - Freezer and Locker Meat Provisioners
- 5441 - Candy, Nut and Confectionery Stores
- 5451 - Dairy Products Stores
- 5462 - Bakeries
- 5499 - Convenience Stores and Specialty Markets

**Healthcare:**
- 5912 - Drug Stores and Pharmacies
- 8011 - Doctors and Physicians
- 8021 - Dentists and Orthodontists
- 8042 - Optometrists and Ophthalmologists
- 8062 - Hospitals
- 8071 - Medical and Dental Laboratories

**Education:**
- 8211 - Elementary and Secondary Schools
- 8220 - Colleges and Universities
- 5942 - Book Stores
- 5943 - Stationery and School Supply Stores

**Transportation:**
- 4121 - Taxicabs and Limousines
- 4131 - Bus Lines
- 4511 - Airlines
- 5541 - Service Stations (Gas)

**Restricted Categories (Often Denied):**
- 5813 - Bars, Taverns, Nightclubs
- 5921 - Package Stores - Beer, Wine, Liquor
- 5993 - Cigar Stores and Stands
- 7995 - Betting, Gambling, Lottery
- 7801 - Online Casinos

---

## Troubleshooting

**Issue: Import fails with "Duplicate entry" error**
- Solution: The MCC code already exists in the database. Use the update functionality or skip duplicates.

**Issue: CSV file not recognized**
- Solution: Ensure the file is UTF-8 encoded and uses comma separators. Check that the header row matches exactly: `mccCode,description,category`

**Issue: Some codes are missing after import**
- Solution: Check the import summary for error messages. Verify that all rows have valid data in all three columns.

**Issue: Cannot find the CSV file**
- Solution: The file is located in the admin portal root directory: `/path/to/admin-portal/mcc-seed-data.csv`

---

## Additional Resources

- **ISO 18245 Standard:** Official merchant category code standard
- **Admin Portal Documentation:** Complete guide to MCC rule management
- **Earmarked Spending Guide:** Detailed explanation of spending control implementation

---

## Support

For questions or issues with MCC seeding:
1. Check the Admin Portal audit logs for import errors
2. Review the database schema in `drizzle/schema.ts`
3. Consult the MCC Management documentation

---

**Last Updated:** November 10, 2025
