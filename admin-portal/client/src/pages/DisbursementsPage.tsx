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
import { trpc } from "@/lib/trpc";
import { Settings, Flag, Calendar, Shield, Plus, Loader2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

export default function DisbursementsPage() {
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [newSchedule, setNewSchedule] = useState({
    programId: "",
    scheduledDate: "",
    amount: "",
    beneficiaryCount: "",
  });

  const { data: schedules, isLoading, refetch } = trpc.disbursements.list.useQuery();
  const { data: programs } = trpc.programs.list.useQuery();
  
  const createSchedule = trpc.disbursements.create.useMutation({
    onSuccess: () => {
      toast.success("Disbursement schedule created successfully");
      setIsCreateDialogOpen(false);
      setNewSchedule({ programId: "", scheduledDate: "", amount: "", beneficiaryCount: "" });
      refetch();
    },
    onError: (error) => {
      toast.error(`Failed to create schedule: ${error.message}`);
    },
  });

  const updateStatus = trpc.disbursements.updateStatus.useMutation({
    onSuccess: () => {
      toast.success("Status updated");
      refetch();
    },
    onError: (error) => {
      toast.error(`Failed to update status: ${error.message}`);
    },
  });

  const handleCreate = () => {
    if (!newSchedule.programId || !newSchedule.scheduledDate || !newSchedule.amount) {
      toast.error("Program, Date, and Amount are required");
      return;
    }
    createSchedule.mutate({
      programId: parseInt(newSchedule.programId),
      scheduledDate: new Date(newSchedule.scheduledDate),
      amount: parseInt(newSchedule.amount),
      beneficiaryCount: newSchedule.beneficiaryCount ? parseInt(newSchedule.beneficiaryCount) : undefined,
    });
  };

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat("en-NG", {
      style: "currency",
      currency: "NGN",
      minimumFractionDigits: 0,
    }).format(amount / 100); // Convert from kobo to naira
  };

  return (
    <DashboardLayout
      items={[
        { label: "Dashboard", href: "/", icon: Shield },
        { label: "Programs & MCC Rules", href: "/programs", icon: Settings },
        { label: "Feature Flags", href: "/feature-flags", icon: Flag },
        { label: "Disbursements", href: "/disbursements", icon: Calendar },
      ]}
    >
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight">Disbursement Schedules</h1>
            <p className="text-muted-foreground mt-2">
              Plan and track social benefit payment schedules
            </p>
          </div>
          <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="h-4 w-4 mr-2" />
                New Schedule
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create Disbursement Schedule</DialogTitle>
                <DialogDescription>Schedule a new social benefit payment</DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="programId">Benefit Program *</Label>
                  <Select
                    value={newSchedule.programId}
                    onValueChange={(value) => setNewSchedule({ ...newSchedule, programId: value })}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="Select a program" />
                    </SelectTrigger>
                    <SelectContent>
                      {programs?.map((program) => (
                        <SelectItem key={program.id} value={program.id.toString()}>
                          {program.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="scheduledDate">Scheduled Date *</Label>
                  <Input
                    id="scheduledDate"
                    type="datetime-local"
                    value={newSchedule.scheduledDate}
                    onChange={(e) => setNewSchedule({ ...newSchedule, scheduledDate: e.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="amount">Total Amount (in Kobo) *</Label>
                  <Input
                    id="amount"
                    type="number"
                    placeholder="e.g., 5000000 (₦50,000)"
                    value={newSchedule.amount}
                    onChange={(e) => setNewSchedule({ ...newSchedule, amount: e.target.value })}
                  />
                  {newSchedule.amount && (
                    <p className="text-sm text-muted-foreground">
                      = {formatCurrency(parseInt(newSchedule.amount))}
                    </p>
                  )}
                </div>
                <div className="space-y-2">
                  <Label htmlFor="beneficiaryCount">Beneficiary Count</Label>
                  <Input
                    id="beneficiaryCount"
                    type="number"
                    placeholder="Optional"
                    value={newSchedule.beneficiaryCount}
                    onChange={(e) => setNewSchedule({ ...newSchedule, beneficiaryCount: e.target.value })}
                  />
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsCreateDialogOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={handleCreate} disabled={createSchedule.isPending}>
                  {createSchedule.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                  Create Schedule
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>All Schedules</CardTitle>
            <CardDescription>View and manage upcoming and past disbursements</CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : !schedules || schedules.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground">
                No schedules found. Create your first schedule to get started.
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Program</TableHead>
                    <TableHead>Scheduled Date</TableHead>
                    <TableHead>Amount</TableHead>
                    <TableHead>Beneficiaries</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {schedules.map((schedule) => {
                    const program = programs?.find((p) => p.id === schedule.programId);
                    return (
                      <TableRow key={schedule.id}>
                        <TableCell className="font-medium">{program?.name || "Unknown"}</TableCell>
                        <TableCell>{new Date(schedule.scheduledDate).toLocaleString()}</TableCell>
                        <TableCell>{formatCurrency(schedule.amount)}</TableCell>
                        <TableCell>{schedule.beneficiaryCount?.toLocaleString() || "—"}</TableCell>
                        <TableCell>
                          <span
                            className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                              schedule.status === "completed"
                                ? "bg-green-100 text-green-800"
                                : schedule.status === "processing"
                                ? "bg-blue-100 text-blue-800"
                                : schedule.status === "failed"
                                ? "bg-red-100 text-red-800"
                                : "bg-yellow-100 text-yellow-800"
                            }`}
                          >
                            {schedule.status}
                          </span>
                        </TableCell>
                        <TableCell className="text-right">
                          {schedule.status === "pending" && (
                            <Select
                              onValueChange={(value: any) => updateStatus.mutate({ id: schedule.id, status: value })}
                            >
                              <SelectTrigger className="w-[140px]">
                                <SelectValue placeholder="Change status" />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="processing">Processing</SelectItem>
                                <SelectItem value="completed">Completed</SelectItem>
                                <SelectItem value="failed">Failed</SelectItem>
                              </SelectContent>
                            </Select>
                          )}
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
