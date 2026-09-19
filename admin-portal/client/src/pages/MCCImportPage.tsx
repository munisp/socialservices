import DashboardLayout from "@/components/DashboardLayout";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { trpc } from "@/lib/trpc";
import { Settings, Flag, Calendar, Shield, Users, Upload, FileText, Check, X, AlertCircle, Loader2 } from "lucide-react";
import { useState, useRef } from "react";
import { toast } from "sonner";

type MCCEntry = {
  mccCode: string;
  description: string;
  category?: string;
};

export default function MCCImportPage() {
  const [csvFile, setCsvFile] = useState<File | null>(null);
  const [parsedData, setParsedData] = useState<MCCEntry[]>([]);
  const [errors, setErrors] = useState<string[]>([]);
  const [isDragging, setIsDragging] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const bulkImport = trpc.mccDatabase.bulkImport.useMutation({
    onSuccess: (data) => {
      toast.success(`Successfully imported ${data.count} MCC entries`);
      setCsvFile(null);
      setParsedData([]);
      setErrors([]);
    },
    onError: (error) => {
      toast.error(`Import failed: ${error.message}`);
    },
  });

  const parseCSV = (text: string): { data: MCCEntry[]; errors: string[] } => {
    const lines = text.trim().split("\n");
    const data: MCCEntry[] = [];
    const errors: string[] = [];

    if (lines.length === 0) {
      errors.push("CSV file is empty");
      return { data, errors };
    }

    // Skip header row if it exists
    const startIndex = lines[0].toLowerCase().includes("mcc") || lines[0].toLowerCase().includes("code") ? 1 : 0;

    for (let i = startIndex; i < lines.length; i++) {
      const line = lines[i].trim();
      if (!line) continue;

      const parts = line.split(",").map((p) => p.trim().replace(/^"|"$/g, ""));

      if (parts.length < 2) {
        errors.push(`Line ${i + 1}: Invalid format (expected at least 2 columns: code, description)`);
        continue;
      }

      const [mccCode, description, category] = parts;

      if (!mccCode || !description) {
        errors.push(`Line ${i + 1}: Missing required fields`);
        continue;
      }

      data.push({
        mccCode,
        description,
        category: category || undefined,
      });
    }

    return { data, errors };
  };

  const handleFileChange = (file: File | null) => {
    if (!file) return;

    if (!file.name.endsWith(".csv")) {
      toast.error("Please upload a CSV file");
      return;
    }

    setCsvFile(file);
    const reader = new FileReader();

    reader.onload = (e) => {
      const text = e.target?.result as string;
      const { data, errors } = parseCSV(text);
      setParsedData(data);
      setErrors(errors);
    };

    reader.onerror = () => {
      toast.error("Failed to read file");
    };

    reader.readAsText(file);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const file = e.dataTransfer.files[0];
    handleFileChange(file);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = () => {
    setIsDragging(false);
  };

  const handleImport = () => {
    if (parsedData.length === 0) {
      toast.error("No data to import");
      return;
    }

    bulkImport.mutate({ entries: parsedData });
  };

  return (
    <DashboardLayout
      items={[
        { label: "Dashboard", href: "/", icon: Shield },
        { label: "Programs & MCC Rules", href: "/programs", icon: Settings },
        { label: "Feature Flags", href: "/feature-flags", icon: Flag },
        { label: "Disbursements", href: "/disbursements", icon: Calendar },
        { label: "User Management", href: "/users", icon: Users },
      ]}
    >
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Bulk MCC Import</h1>
          <p className="text-muted-foreground mt-2">
            Upload a CSV file to import Merchant Category Codes in bulk
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Upload CSV File</CardTitle>
            <CardDescription>
              CSV format: <code className="bg-muted px-2 py-1 rounded">mccCode, description, category (optional)</code>
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div
              onDrop={handleDrop}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              className={`border-2 border-dashed rounded-lg p-8 text-center transition-colors ${
                isDragging ? "border-primary bg-primary/5" : "border-muted-foreground/25"
              }`}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".csv"
                onChange={(e) => handleFileChange(e.target.files?.[0] || null)}
                className="hidden"
              />
              <Upload className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
              <p className="text-lg font-medium mb-2">
                {csvFile ? csvFile.name : "Drag and drop your CSV file here"}
              </p>
              <p className="text-sm text-muted-foreground mb-4">or</p>
              <Button onClick={() => fileInputRef.current?.click()} variant="outline">
                <FileText className="h-4 w-4 mr-2" />
                Browse Files
              </Button>
            </div>

            {errors.length > 0 && (
              <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertDescription>
                  <div className="font-medium mb-2">Found {errors.length} error(s):</div>
                  <ul className="list-disc list-inside space-y-1 text-sm">
                    {errors.slice(0, 5).map((error, idx) => (
                      <li key={idx}>{error}</li>
                    ))}
                    {errors.length > 5 && <li>...and {errors.length - 5} more</li>}
                  </ul>
                </AlertDescription>
              </Alert>
            )}

            {parsedData.length > 0 && (
              <div className="flex items-center justify-between p-4 bg-green-50 border border-green-200 rounded-lg">
                <div className="flex items-center gap-2">
                  <Check className="h-5 w-5 text-green-600" />
                  <span className="font-medium text-green-900">
                    {parsedData.length} MCC entries ready to import
                  </span>
                </div>
                <Button onClick={handleImport} disabled={bulkImport.isPending || errors.length > 0}>
                  {bulkImport.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                  Import All
                </Button>
              </div>
            )}
          </CardContent>
        </Card>

        {parsedData.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle>Preview ({parsedData.length} entries)</CardTitle>
              <CardDescription>Review the data before importing</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="max-h-96 overflow-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>MCC Code</TableHead>
                      <TableHead>Description</TableHead>
                      <TableHead>Category</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {parsedData.slice(0, 50).map((entry, idx) => (
                      <TableRow key={idx}>
                        <TableCell>
                          <code className="bg-muted px-2 py-1 rounded font-mono">{entry.mccCode}</code>
                        </TableCell>
                        <TableCell>{entry.description}</TableCell>
                        <TableCell>{entry.category || "—"}</TableCell>
                      </TableRow>
                    ))}
                    {parsedData.length > 50 && (
                      <TableRow>
                        <TableCell colSpan={3} className="text-center text-muted-foreground">
                          ...and {parsedData.length - 50} more entries
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </div>
            </CardContent>
          </Card>
        )}

        <Card>
          <CardHeader>
            <CardTitle>CSV Format Example</CardTitle>
            <CardDescription>Use this format for your CSV file</CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
              {`mccCode,description,category
5411,Grocery Stores,Food & Groceries
5812,Eating Places and Restaurants,Food & Dining
5912,Drug Stores and Pharmacies,Health & Pharmacy
5541,Service Stations,Automotive
5999,Miscellaneous and Specialty Retail Stores,Retail`}
            </pre>
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
