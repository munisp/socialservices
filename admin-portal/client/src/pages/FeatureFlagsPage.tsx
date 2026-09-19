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
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { trpc } from "@/lib/trpc";
import { Settings, Flag, Calendar, Shield, Plus, Loader2, CheckSquare, Square } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

export default function FeatureFlagsPage() {
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [newFlag, setNewFlag] = useState({
    name: "",
    description: "",
    enabled: false,
    environment: "all" as "all" | "production" | "staging" | "development",
  });
  const [selectedFlags, setSelectedFlags] = useState<number[]>([]);
  const [batchJustification, setBatchJustification] = useState("");
  const [isBatchDialogOpen, setIsBatchDialogOpen] = useState(false);
  const [batchAction, setBatchAction] = useState<"enable" | "disable" | null>(null);

  const { data: flags, isLoading, refetch } = trpc.featureFlags.list.useQuery();
  
  const batchToggle = trpc.featureFlags.batchToggle.useMutation({
    onSuccess: (data: any) => {
      toast.success(`Successfully updated ${data.count} feature flags`);
      setSelectedFlags([]);
      setBatchJustification("");
      setIsBatchDialogOpen(false);
      refetch();
    },
    onError: (error: any) => {
      toast.error(`Failed to update flags: ${error.message}`);
    },
  });

  const createFlag = trpc.featureFlags.create.useMutation({
    onSuccess: () => {
      toast.success("Feature flag created successfully");
      setIsCreateDialogOpen(false);
      setNewFlag({ name: "", description: "", enabled: false, environment: "all" });
      refetch();
    },
    onError: (error: any) => {
      toast.error(`Failed to create flag: ${error.message}`);
    },
  });

  const toggleFlag = trpc.featureFlags.toggle.useMutation({
    onSuccess: () => {
      toast.success("Feature flag updated");
      refetch();
    },
    onError: (error: any) => {
      toast.error(`Failed to update flag: ${error.message}`);
    },
  });

  const toggleFlagSelection = (flagId: number) => {
    setSelectedFlags((prev) =>
      prev.includes(flagId) ? prev.filter((id) => id !== flagId) : [...prev, flagId]
    );
  };

  const toggleSelectAll = () => {
    if (selectedFlags.length === flags?.length) {
      setSelectedFlags([]);
    } else {
      setSelectedFlags(flags?.map((f) => f.id) || []);
    }
  };

  const openBatchDialog = (action: "enable" | "disable") => {
    if (selectedFlags.length === 0) {
      toast.error("Please select at least one feature flag");
      return;
    }
    setBatchAction(action);
    setIsBatchDialogOpen(true);
  };

  const handleBatchToggle = () => {
    if (!batchAction || selectedFlags.length === 0) return;
    if (batchJustification.length < 10) {
      toast.error("Justification must be at least 10 characters");
      return;
    }
    batchToggle.mutate({
      flagIds: selectedFlags,
      enabled: batchAction === "enable",
      justification: batchJustification,
    });
  };

  const handleCreate = () => {
    if (!newFlag.name) {
      toast.error("Name is required");
      return;
    }
    createFlag.mutate(newFlag);
  };

  const handleToggle = (id: number, enabled: boolean) => {
    toggleFlag.mutate({ id, enabled });
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
            <h1 className="text-3xl font-bold tracking-tight">Feature Flags</h1>
            <p className="text-muted-foreground mt-2">
              Control platform features across different environments
            </p>
          </div>
          <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="h-4 w-4 mr-2" />
                New Flag
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create Feature Flag</DialogTitle>
                <DialogDescription>Add a new feature flag to control platform functionality</DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="name">Flag Name *</Label>
                  <Input
                    id="name"
                    placeholder="e.g., enable_new_dashboard"
                    value={newFlag.name}
                    onChange={(e) => setNewFlag({ ...newFlag, name: e.target.value.toLowerCase().replace(/\s+/g, "_") })}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="description">Description</Label>
                  <Textarea
                    id="description"
                    placeholder="What does this flag control?"
                    value={newFlag.description}
                    onChange={(e) => setNewFlag({ ...newFlag, description: e.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="environment">Environment</Label>
                  <Select
                    value={newFlag.environment}
                    onValueChange={(value: any) => setNewFlag({ ...newFlag, environment: value })}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">All Environments</SelectItem>
                      <SelectItem value="production">Production Only</SelectItem>
                      <SelectItem value="staging">Staging Only</SelectItem>
                      <SelectItem value="development">Development Only</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex items-center space-x-2">
                  <Switch
                    id="enabled"
                    checked={newFlag.enabled}
                    onCheckedChange={(checked) => setNewFlag({ ...newFlag, enabled: checked })}
                  />
                  <Label htmlFor="enabled">Enable immediately</Label>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsCreateDialogOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={handleCreate} disabled={createFlag.isPending}>
                  {createFlag.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                  Create Flag
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>

        {selectedFlags.length > 0 && (
          <Card className="border-blue-200 bg-blue-50">
            <CardContent className="pt-6">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <CheckSquare className="h-5 w-5 text-blue-600" />
                  <span className="font-medium">{selectedFlags.length} flag(s) selected</span>
                </div>
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" onClick={() => openBatchDialog("enable")}>
                    Enable Selected
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => openBatchDialog("disable")}>
                    Disable Selected
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => setSelectedFlags([])}>
                    Clear Selection
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        )}

        <Card>
          <CardHeader>
            <CardTitle>All Feature Flags</CardTitle>
            <CardDescription>Toggle flags to enable or disable features across the platform</CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : !flags || flags.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground">
                No feature flags found. Create your first flag to get started.
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-12">
                      <Button variant="ghost" size="sm" onClick={toggleSelectAll}>
                        {selectedFlags.length === flags?.length ? (
                          <CheckSquare className="h-4 w-4" />
                        ) : (
                          <Square className="h-4 w-4" />
                        )}
                      </Button>
                    </TableHead>
                    <TableHead>Flag Name</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead>Environment</TableHead>
                    <TableHead>Last Updated</TableHead>
                    <TableHead className="text-right">Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {flags.map((flag) => (
                    <TableRow key={flag.id}>
                      <TableCell>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => toggleFlagSelection(flag.id)}
                        >
                          {selectedFlags.includes(flag.id) ? (
                            <CheckSquare className="h-4 w-4" />
                          ) : (
                            <Square className="h-4 w-4" />
                          )}
                        </Button>
                      </TableCell>
                      <TableCell>
                        <code className="bg-muted px-2 py-1 rounded text-sm font-mono">{flag.name}</code>
                      </TableCell>
                      <TableCell className="max-w-md truncate">{flag.description || "—"}</TableCell>
                      <TableCell>
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
                          {flag.environment}
                        </span>
                      </TableCell>
                      <TableCell>{new Date(flag.updatedAt).toLocaleDateString()}</TableCell>
                      <TableCell className="text-right">
                        <div className="flex items-center justify-end gap-2">
                          <span className="text-sm text-muted-foreground">
                            {flag.enabled ? "Enabled" : "Disabled"}
                          </span>
                          <Switch
                            checked={flag.enabled}
                            onCheckedChange={(checked) => handleToggle(flag.id, checked)}
                            disabled={toggleFlag.isPending}
                          />
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Batch Toggle Dialog */}
      <Dialog open={isBatchDialogOpen} onOpenChange={setIsBatchDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {batchAction === "enable" ? "Enable" : "Disable"} {selectedFlags.length} Feature Flag(s)
            </DialogTitle>
            <DialogDescription>
              This action will {batchAction === "enable" ? "enable" : "disable"} the selected feature flags.
              Please provide a justification for this change.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="batch-justification">Justification (minimum 10 characters) *</Label>
              <Textarea
                id="batch-justification"
                placeholder="Explain why these flags are being updated..."
                value={batchJustification}
                onChange={(e) => setBatchJustification(e.target.value)}
                rows={4}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsBatchDialogOpen(false)}>
              Cancel
            </Button>
            <Button 
              onClick={handleBatchToggle} 
              disabled={batchToggle.isPending || batchJustification.length < 10}
            >
              {batchToggle.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
              Confirm {batchAction === "enable" ? "Enable" : "Disable"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DashboardLayout>
  );
}
