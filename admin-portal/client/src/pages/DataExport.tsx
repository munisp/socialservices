import { useState } from "react";
import { trpc } from "@/lib/trpc";
import DashboardLayout from "@/components/DashboardLayout";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Download, FileSpreadsheet, FileText, Database } from "lucide-react";
import { toast } from "sonner";

export default function DataExport() {
  const [format, setFormat] = useState<"csv" | "excel">("csv");

  const beneficiariesExport = trpc.exports.beneficiaries.useMutation({
    onSuccess: (data) => {
      downloadFile(data.csv, data.filename);
      toast.success("Beneficiaries exported successfully");
    },
    onError: (error) => {
      toast.error(`Export failed: ${error.message}`);
    },
  });

  const transactionsExport = trpc.exports.transactions.useMutation({
    onSuccess: (data) => {
      downloadFile(data.csv, data.filename);
      toast.success("Transactions exported successfully");
    },
    onError: (error) => {
      toast.error(`Export failed: ${error.message}`);
    },
  });

  const auditLogsExport = trpc.exports.auditLogs.useMutation({
    onSuccess: (data) => {
      downloadFile(data.csv, data.filename);
      toast.success("Audit logs exported successfully");
    },
    onError: (error) => {
      toast.error(`Export failed: ${error.message}`);
    },
  });

  const fraudAlertsExport = trpc.exports.fraudAlerts.useMutation({
    onSuccess: (data) => {
      downloadFile(data.csv, data.filename);
      toast.success("Fraud alerts exported successfully");
    },
    onError: (error) => {
      toast.error(`Export failed: ${error.message}`);
    },
  });

  const downloadFile = (content: string, filename: string) => {
    const blob = new Blob([content], { type: "text/csv;charset=utf-8;" });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  };

  const exportOptions = [
    {
      title: "Beneficiaries",
      description: "Export all beneficiary records with enrollment status and KYC information",
      icon: Database,
      mutation: beneficiariesExport,
      fields: [
        "ID",
        "First Name",
        "Last Name",
        "National ID",
        "Email",
        "Phone Number",
        "City",
        "State",
        "Enrollment Status",
        "KYC Status",
        "Enrolled At",
        "Approved At",
      ],
    },
    {
      title: "Transactions",
      description: "Export transaction history with MCC codes and compliance status",
      icon: FileSpreadsheet,
      mutation: transactionsExport,
      fields: [
        "Transaction ID",
        "Beneficiary ID",
        "Program ID",
        "Merchant Name",
        "MCC Code",
        "MCC Description",
        "Amount",
        "Currency",
        "Transaction Type",
        "Status",
        "Compliance Status",
        "Fraud Score",
        "Transaction Date",
        "Decline Reason",
      ],
    },
    {
      title: "Audit Logs",
      description: "Export complete audit trail of all administrative actions",
      icon: FileText,
      mutation: auditLogsExport,
      fields: [
        "ID",
        "User ID",
        "Action Type",
        "Target Type",
        "Target ID",
        "Changes",
        "IP Address",
        "User Agent",
        "Timestamp",
      ],
    },
    {
      title: "Fraud Alerts",
      description: "Export fraud detection alerts and investigation records",
      icon: FileText,
      mutation: fraudAlertsExport,
      fields: [
        "ID",
        "Transaction ID",
        "Beneficiary ID",
        "Alert Type",
        "Severity",
        "Description",
        "Status",
        "Assigned To",
        "Resolved By",
        "Resolution",
        "Created At",
        "Resolved At",
      ],
    },
  ];

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold">Data Export & Reporting</h1>
            <p className="text-muted-foreground">
              Export platform data in CSV or Excel format for analysis and compliance
            </p>
          </div>
          <Select value={format} onValueChange={(value: any) => setFormat(value)}>
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="Select format" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="csv">CSV Format</SelectItem>
              <SelectItem value="excel">Excel Format</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="grid gap-6 md:grid-cols-2">
          {exportOptions.map((option, index) => {
            const Icon = option.icon;
            return (
              <Card key={index}>
                <CardHeader>
                  <div className="flex items-center gap-4">
                    <div className="p-2 bg-primary/10 rounded-lg">
                      <Icon className="h-6 w-6 text-primary" />
                    </div>
                    <div>
                      <CardTitle>{option.title}</CardTitle>
                      <CardDescription>{option.description}</CardDescription>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div>
                    <h4 className="text-sm font-medium mb-2">Exported Fields:</h4>
                    <div className="flex flex-wrap gap-2">
                      {option.fields.map((field, fieldIndex) => (
                        <span
                          key={fieldIndex}
                          className="text-xs bg-secondary px-2 py-1 rounded"
                        >
                          {field}
                        </span>
                      ))}
                    </div>
                  </div>
                  <Button
                    className="w-full"
                    onClick={() => option.mutation.mutate({ format })}
                    disabled={option.mutation.isPending}
                  >
                    <Download className="mr-2 h-4 w-4" />
                    {option.mutation.isPending ? "Exporting..." : `Export ${option.title}`}
                  </Button>
                </CardContent>
              </Card>
            );
          })}
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Export Guidelines</CardTitle>
            <CardDescription>Important information about data exports</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <h4 className="font-medium mb-2">Data Privacy & Security</h4>
              <ul className="list-disc list-inside space-y-1 text-sm text-muted-foreground">
                <li>All exports are logged in the audit trail for compliance</li>
                <li>Exported files contain sensitive personal information - handle with care</li>
                <li>Do not share exported data via unsecured channels</li>
                <li>Delete exported files after use or store in encrypted storage</li>
              </ul>
            </div>
            <div>
              <h4 className="font-medium mb-2">File Formats</h4>
              <ul className="list-disc list-inside space-y-1 text-sm text-muted-foreground">
                <li>
                  <strong>CSV:</strong> Universal format compatible with all spreadsheet applications
                </li>
                <li>
                  <strong>Excel:</strong> Formatted for Microsoft Excel with UTF-8 encoding
                </li>
              </ul>
            </div>
            <div>
              <h4 className="font-medium mb-2">Export Limits</h4>
              <ul className="list-disc list-inside space-y-1 text-muted-foreground">
                <li>Beneficiaries: All records</li>
                <li>Transactions: Last 1,000 records</li>
                <li>Audit Logs: All records</li>
                <li>Fraud Alerts: Last 1,000 records</li>
              </ul>
            </div>
            <div>
              <h4 className="font-medium mb-2">Scheduled Exports</h4>
              <p className="text-sm text-muted-foreground">
                For automated recurring exports, configure scheduled reports in the{" "}
                <a href="/scheduled-reports" className="text-primary hover:underline">
                  Scheduled Reports
                </a>{" "}
                section.
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
