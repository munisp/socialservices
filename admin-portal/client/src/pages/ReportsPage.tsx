import DashboardLayout from "@/components/DashboardLayout";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { trpc } from "@/lib/trpc";
import { Settings, Flag, Calendar, Shield, Plus, Loader2, Play, Trash2, Clock, FileText, Users } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

export default function ReportsPage() {
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [selectedReportId, setSelectedReportId] = useState<number | null>(null);
  const [newReport, setNewReport] = useState({
    name: "",
    description: "",
    reportType: "weekly" as "weekly" | "monthly",
    recipients: "",
  });

  const { data: reports, isLoading, refetch } = trpc.scheduledReports.list.useQuery();
  const { data: history } = trpc.scheduledReports.getHistory.useQuery(
    { reportId: selectedReportId! },
    { enabled: !!selectedReportId }
  );

  const createReport = trpc.scheduledReports.create.useMutation({
    onSuccess: () => {
      toast.success("Scheduled report created");
      setIsCreateDialogOpen(false);
      setNewReport({ name: "", description: "", reportType: "weekly", recipients: "" });
      refetch();
    },
    onError: (error) => {
      toast.error(`Failed to create report: ${error.message}`);
    },
  });

  const deleteReport = trpc.scheduledReports.delete.useMutation({
    onSuccess: () => {
      toast.success("Report deleted");
      refetch();
    },
    onError: (error) => {
      toast.error(`Failed to delete report: ${error.message}`);
    },
  });

  const generateNow = trpc.scheduledReports.generateNow.useMutation({
    onSuccess: () => {
      toast.success("Report generated and sent successfully");
      refetch();
    },
    onError: (error) => {
      toast.error(`Failed to generate report: ${error.message}`);
    },
  });

  const toggleActive = trpc.scheduledReports.update.useMutation({
    onSuccess: () => {
      toast.success("Report status updated");
      refetch();
    },
    onError: (error) => {
      toast.error(`Failed to update report: ${error.message}`);
    },
  });

  const handleCreate = () => {
    if (!newReport.name || !newReport.recipients) {
      toast.error("Name and recipients are required");
      return;
    }

    const recipientEmails = newReport.recipients
      .split(",")
      .map((email) => email.trim())
      .filter((email) => email.length > 0);

    if (recipientEmails.length === 0) {
      toast.error("At least one recipient email is required");
      return;
    }

    createReport.mutate({
      name: newReport.name,
      description: newReport.description || undefined,
      reportType: newReport.reportType,
      recipients: recipientEmails,
    });
  };

  return (
    <DashboardLayout
      items={[
        { label: "Dashboard", href: "/", icon: Shield },
        { label: "Programs & MCC Rules", href: "/programs", icon: Settings },
        { label: "Feature Flags", href: "/feature-flags", icon: Flag },
        { label: "Disbursements", href: "/disbursements", icon: Calendar },
        { label: "Users", href: "/users", icon: Users },
      ]}
    >
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight">Scheduled Reports</h1>
            <p className="text-muted-foreground mt-2">
              Configure automated weekly and monthly summary reports
            </p>
          </div>
          <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="h-4 w-4 mr-2" />
                New Report
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create Scheduled Report</DialogTitle>
                <DialogDescription>
                  Configure a new automated report to be sent via email
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="name">Report Name *</Label>
                  <Input
                    id="name"
                    placeholder="e.g., Weekly Admin Summary"
                    value={newReport.name}
                    onChange={(e) => setNewReport({ ...newReport, name: e.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="description">Description</Label>
                  <Textarea
                    id="description"
                    placeholder="Optional description..."
                    value={newReport.description}
                    onChange={(e) => setNewReport({ ...newReport, description: e.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="reportType">Report Type *</Label>
                  <Select
                    value={newReport.reportType}
                    onValueChange={(value: "weekly" | "monthly") =>
                      setNewReport({ ...newReport, reportType: value })
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="weekly">Weekly (Every Monday)</SelectItem>
                      <SelectItem value="monthly">Monthly (1st of each month)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="recipients">Recipients *</Label>
                  <Textarea
                    id="recipients"
                    placeholder="admin@example.com, manager@example.com"
                    value={newReport.recipients}
                    onChange={(e) => setNewReport({ ...newReport, recipients: e.target.value })}
                  />
                  <p className="text-xs text-muted-foreground">
                    Comma-separated email addresses
                  </p>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsCreateDialogOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={handleCreate} disabled={createReport.isPending}>
                  {createReport.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                  Create Report
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Configured Reports</CardTitle>
            <CardDescription>
              Manage automated report generation and delivery schedules
            </CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : !reports || reports.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground">
                No scheduled reports configured yet
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Report Name</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Recipients</TableHead>
                    <TableHead>Next Run</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {reports.map((report) => {
                    const recipients = JSON.parse(report.recipients) as string[];
                    return (
                      <TableRow key={report.id}>
                        <TableCell className="font-medium">{report.name}</TableCell>
                        <TableCell>
                          <Badge variant="outline">
                            {report.reportType === "weekly" ? "Weekly" : "Monthly"}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {recipients.length} recipient{recipients.length !== 1 ? "s" : ""}
                        </TableCell>
                        <TableCell className="text-sm">
                          {new Date(report.nextRunAt).toLocaleString()}
                        </TableCell>
                        <TableCell>
                          <Badge variant={report.isActive ? "default" : "secondary"}>
                            {report.isActive ? "Active" : "Inactive"}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="flex justify-end gap-2">
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => setSelectedReportId(report.id)}
                              title="View history"
                            >
                              <Clock className="h-4 w-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() =>
                                toggleActive.mutate({
                                  id: report.id,
                                  isActive: !report.isActive,
                                })
                              }
                              title={report.isActive ? "Deactivate" : "Activate"}
                            >
                              {report.isActive ? "Pause" : "Resume"}
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => generateNow.mutate({ id: report.id })}
                              disabled={generateNow.isPending}
                              title="Generate now"
                            >
                              <Play className="h-4 w-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => deleteReport.mutate({ id: report.id })}
                              title="Delete"
                            >
                              <Trash2 className="h-4 w-4 text-destructive" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        {selectedReportId && (
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>Report History</CardTitle>
                  <CardDescription>
                    Past executions of{" "}
                    {reports?.find((r) => r.id === selectedReportId)?.name}
                  </CardDescription>
                </div>
                <Button variant="outline" size="sm" onClick={() => setSelectedReportId(null)}>
                  Close
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              {!history || history.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  No execution history yet
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Generated At</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Error</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {history.map((entry) => (
                      <TableRow key={entry.id}>
                        <TableCell>{new Date(entry.generatedAt).toLocaleString()}</TableCell>
                        <TableCell>
                          <Badge variant={entry.status === "success" ? "default" : "destructive"}>
                            {entry.status}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {entry.errorMessage || "—"}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        )}
      </div>
    </DashboardLayout>
  );
}
