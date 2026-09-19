import { Parser } from "json2csv";

export interface ExportColumn {
  label: string;
  value: string | ((row: any) => any);
}

export interface ExportOptions {
  filename: string;
  columns: ExportColumn[];
  data: any[];
}

/**
 * Convert data to CSV format
 */
export function generateCSV(options: ExportOptions): string {
  const { columns, data } = options;
  
  const fields = columns.map(col => ({
    label: col.label,
    value: col.value,
  }));
  
  const parser = new Parser({ fields });
  return parser.parse(data);
}

/**
 * Format data for Excel-compatible CSV (with BOM for proper UTF-8 encoding)
 */
export function generateExcelCSV(options: ExportOptions): string {
  const csv = generateCSV(options);
  // Add BOM for Excel UTF-8 recognition
  return '\ufeff' + csv;
}

/**
 * Generate filename with timestamp
 */
export function generateFilename(prefix: string, extension: string): string {
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
  return `${prefix}_${timestamp}.${extension}`;
}

/**
 * Pre-defined export templates
 */

export const BENEFICIARY_EXPORT_COLUMNS: ExportColumn[] = [
  { label: "ID", value: "id" },
  { label: "First Name", value: "firstName" },
  { label: "Last Name", value: "lastName" },
  { label: "National ID", value: "nationalId" },
  { label: "Email", value: "email" },
  { label: "Phone Number", value: "phoneNumber" },
  { label: "City", value: "city" },
  { label: "State", value: "state" },
  { label: "Enrollment Status", value: "enrollmentStatus" },
  { label: "KYC Status", value: "kycStatus" },
  { label: "Enrolled At", value: (row) => new Date(row.enrolledAt).toISOString() },
  { label: "Approved At", value: (row) => row.approvedAt ? new Date(row.approvedAt).toISOString() : "" },
];

export const TRANSACTION_EXPORT_COLUMNS: ExportColumn[] = [
  { label: "Transaction ID", value: "transactionId" },
  { label: "Beneficiary ID", value: "beneficiaryId" },
  { label: "Program ID", value: "programId" },
  { label: "Merchant Name", value: "merchantName" },
  { label: "MCC Code", value: "mccCode" },
  { label: "MCC Description", value: "mccDescription" },
  { label: "Amount", value: (row) => (row.amount / 100).toFixed(2) },
  { label: "Currency", value: "currency" },
  { label: "Transaction Type", value: "transactionType" },
  { label: "Status", value: "status" },
  { label: "Compliance Status", value: "complianceStatus" },
  { label: "Fraud Score", value: "fraudScore" },
  { label: "Transaction Date", value: (row) => new Date(row.transactionDate).toISOString() },
  { label: "Decline Reason", value: "declineReason" },
];

export const AUDIT_LOG_EXPORT_COLUMNS: ExportColumn[] = [
  { label: "ID", value: "id" },
  { label: "User ID", value: "userId" },
  { label: "Action Type", value: "actionType" },
  { label: "Target Type", value: "targetType" },
  { label: "Target ID", value: "targetId" },
  { label: "Changes", value: (row) => JSON.stringify(row.changes) },
  { label: "IP Address", value: "ipAddress" },
  { label: "User Agent", value: "userAgent" },
  { label: "Timestamp", value: (row) => new Date(row.timestamp).toISOString() },
];

export const PROGRAM_EXPORT_COLUMNS: ExportColumn[] = [
  { label: "ID", value: "id" },
  { label: "Name", value: "name" },
  { label: "Account Type", value: "accountType" },
  { label: "Description", value: "description" },
  { label: "Status", value: "status" },
  { label: "Created At", value: (row) => new Date(row.createdAt).toISOString() },
  { label: "Updated At", value: (row) => new Date(row.updatedAt).toISOString() },
];

export const FRAUD_ALERT_EXPORT_COLUMNS: ExportColumn[] = [
  { label: "ID", value: "id" },
  { label: "Transaction ID", value: "transactionId" },
  { label: "Beneficiary ID", value: "beneficiaryId" },
  { label: "Alert Type", value: "alertType" },
  { label: "Severity", value: "severity" },
  { label: "Description", value: "description" },
  { label: "Status", value: "status" },
  { label: "Assigned To", value: "assignedTo" },
  { label: "Resolved By", value: "resolvedBy" },
  { label: "Resolution", value: "resolution" },
  { label: "Created At", value: (row) => new Date(row.createdAt).toISOString() },
  { label: "Resolved At", value: (row) => row.resolvedAt ? new Date(row.resolvedAt).toISOString() : "" },
];

export const ENROLLMENT_EXPORT_COLUMNS: ExportColumn[] = [
  { label: "ID", value: "id" },
  { label: "Beneficiary ID", value: "beneficiaryId" },
  { label: "Program ID", value: "programId" },
  { label: "Enrollment Date", value: (row) => new Date(row.enrollmentDate).toISOString() },
  { label: "Status", value: "status" },
  { label: "Monthly Allocation", value: (row) => (row.monthlyAllocation / 100).toFixed(2) },
  { label: "Last Disbursement", value: (row) => row.lastDisbursement ? new Date(row.lastDisbursement).toISOString() : "" },
];

/**
 * Sanitize data for export (remove sensitive fields)
 */
export function sanitizeForExport<T extends Record<string, any>>(
  data: T[],
  excludeFields: string[] = []
): Partial<T>[] {
  return data.map(row => {
    const sanitized = { ...row };
    excludeFields.forEach(field => {
      delete sanitized[field];
    });
    return sanitized;
  });
}
