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
import { trpc } from "@/lib/trpc";
import { Settings, Flag, Calendar, Shield, Plus, Loader2, CheckSquare, Square, Users, FileText } from "lucide-react";
import { useState } from "react";
import { Link } from "wouter";
import { toast } from "sonner";

export default function ProgramsPage() {
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [isTemplateDialogOpen, setIsTemplateDialogOpen] = useState(false);
  const [selectedTemplateId, setSelectedTemplateId] = useState<number | null>(null);
  const [newProgramFromTemplate, setNewProgramFromTemplate] = useState({
    name: "",
    description: "",
  });
  const [newProgram, setNewProgram] = useState({
    name: "",
    accountType: "",
    description: "",
    status: "active" as "active" | "inactive" | "pending_review",
  });
  const [selectedPrograms, setSelectedPrograms] = useState<number[]>([]);
  const [batchJustification, setBatchJustification] = useState("");
  const [isBatchDialogOpen, setIsBatchDialogOpen] = useState(false);
  const [batchAction, setBatchAction] = useState<"activate" | "deactivate" | null>(null);

  const { data: programs, isLoading, refetch } = trpc.programs.list.useQuery();
  const { data: templates } = trpc.programTemplates.list.useQuery();
  
  const batchUpdate = trpc.programs.batchUpdateStatus.useMutation({
    onSuccess: (data: any) => {
      toast.success(`Successfully updated ${data.count} programs`);
      setSelectedPrograms([]);
      setBatchJustification("");
      setIsBatchDialogOpen(false);
      refetch();
    },
    onError: (error: any) => {
      toast.error(`Failed to update programs: ${error.message}`);
    },
  });

  const createFromTemplate = trpc.programTemplates.createProgramFromTemplate.useMutation({
    onSuccess: () => {
      toast.success("Program created from template successfully");
      setIsTemplateDialogOpen(false);
      setSelectedTemplateId(null);
      setNewProgramFromTemplate({ name: "", description: "" });
      refetch();
    },
    onError: (error: any) => {
      toast.error(`Failed to create program: ${error.message}`);
    },
  });

  const createProgram = trpc.programs.create.useMutation({
    onSuccess: () => {
      toast.success("Program created successfully");
      setIsCreateDialogOpen(false);
      setNewProgram({ name: "", accountType: "", description: "", status: "active" });
      refetch();
    },
    onError: (error: any) => {
      toast.error(`Failed to create program: ${error.message}`);
    },
  });

  const toggleProgramSelection = (programId: number) => {
    setSelectedPrograms((prev) =>
      prev.includes(programId) ? prev.filter((id) => id !== programId) : [...prev, programId]
    );
  };

  const toggleSelectAll = () => {
    if (selectedPrograms.length === programs?.length) {
      setSelectedPrograms([]);
    } else {
      setSelectedPrograms(programs?.map((p) => p.id) || []);
    }
  };

  const openBatchDialog = (action: "activate" | "deactivate") => {
    if (selectedPrograms.length === 0) {
      toast.error("Please select at least one program");
      return;
    }
    setBatchAction(action);
    setIsBatchDialogOpen(true);
  };

  const handleBatchUpdate = () => {
    if (!batchAction || selectedPrograms.length === 0) return;
    if (batchJustification.length < 10) {
      toast.error("Justification must be at least 10 characters");
      return;
    }
    batchUpdate.mutate({
      programIds: selectedPrograms,
      isActive: batchAction === "activate",
      justification: batchJustification,
    });
  };

  const handleCreate = () => {
    if (!newProgram.name || !newProgram.accountType) {
      toast.error("Name and Account Type are required");
      return;
    }
    createProgram.mutate(newProgram);
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
            <h1 className="text-3xl font-bold tracking-tight">Benefit Programs & MCC Rules</h1>
            <p className="text-muted-foreground mt-2">
              Manage social benefit programs and control earmarked spending
            </p>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setIsTemplateDialogOpen(true)}>
              <FileText className="h-4 w-4 mr-2" />
              From Template
            </Button>
            <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
              <DialogTrigger asChild>
                <Button>
                  <Plus className="h-4 w-4 mr-2" />
                  New Program
                </Button>
              </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create Benefit Program</DialogTitle>
                <DialogDescription>
                  Add a new social benefit program to the platform
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="name">Program Name *</Label>
                  <Input
                    id="name"
                    placeholder="e.g., Food Subsidy Program"
                    value={newProgram.name}
                    onChange={(e) => setNewProgram({ ...newProgram, name: e.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="accountType">TigerBeetle Account Type *</Label>
                  <Input
                    id="accountType"
                    placeholder="e.g., FOOD_BENEFIT"
                    value={newProgram.accountType}
                    onChange={(e) =>
                      setNewProgram({ ...newProgram, accountType: e.target.value.toUpperCase() })
                    }
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="description">Description</Label>
                  <Textarea
                    id="description"
                    placeholder="Program description..."
                    value={newProgram.description}
                    onChange={(e) => setNewProgram({ ...newProgram, description: e.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="status">Status</Label>
                  <Select
                    value={newProgram.status}
                    onValueChange={(value: any) => setNewProgram({ ...newProgram, status: value })}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="active">Active</SelectItem>
                      <SelectItem value="inactive">Inactive</SelectItem>
                      <SelectItem value="pending_review">Pending Review</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsCreateDialogOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={handleCreate} disabled={createProgram.isPending}>
                  {createProgram.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                  Create Program
                </Button>
              </DialogFooter>
            </DialogContent>
            </Dialog>
          </div>
        </div>

        <Dialog open={isTemplateDialogOpen} onOpenChange={setIsTemplateDialogOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create from Template</DialogTitle>
              <DialogDescription>
                Select a template to quickly create a new program with pre-configured MCC rules
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="template">Template *</Label>
                <Select
                  value={selectedTemplateId?.toString() || ""}
                  onValueChange={(value) => setSelectedTemplateId(parseInt(value))}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Select a template" />
                  </SelectTrigger>
                  <SelectContent>
                    {templates && templates.length > 0 ? (
                      templates.map((template) => (
                        <SelectItem key={template.id} value={template.id.toString()}>
                          {template.name}
                        </SelectItem>
                      ))
                    ) : (
                      <div className="p-2 text-sm text-muted-foreground">No templates available</div>
                    )}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="programName">Program Name *</Label>
                <Input
                  id="programName"
                  placeholder="e.g., Food Subsidy Program 2024"
                  value={newProgramFromTemplate.name}
                  onChange={(e) =>
                    setNewProgramFromTemplate({ ...newProgramFromTemplate, name: e.target.value })
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="programDescription">Description</Label>
                <Textarea
                  id="programDescription"
                  placeholder="Optional description..."
                  value={newProgramFromTemplate.description}
                  onChange={(e) =>
                    setNewProgramFromTemplate({ ...newProgramFromTemplate, description: e.target.value })
                  }
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setIsTemplateDialogOpen(false)}>
                Cancel
              </Button>
              <Button
                onClick={() => {
                  if (!selectedTemplateId || !newProgramFromTemplate.name.trim()) {
                    toast.error("Template and program name are required");
                    return;
                  }
                  createFromTemplate.mutate({
                    templateId: selectedTemplateId,
                    programName: newProgramFromTemplate.name.trim(),
                    programDescription: newProgramFromTemplate.description.trim() || undefined,
                  });
                }}
                disabled={createFromTemplate.isPending}
              >
                {createFromTemplate.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                Create Program
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {selectedPrograms.length > 0 && (
          <Card className="border-blue-200 bg-blue-50">
            <CardContent className="pt-6">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <CheckSquare className="h-5 w-5 text-blue-600" />
                  <span className="font-medium">{selectedPrograms.length} program(s) selected</span>
                </div>
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" onClick={() => openBatchDialog("activate")}>
                    Activate Selected
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => openBatchDialog("deactivate")}>
                    Deactivate Selected
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => setSelectedPrograms([])}>
                    Clear Selection
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        )}

        <Card>
          <CardHeader>
            <CardTitle>All Programs</CardTitle>
            <CardDescription>
              Click on a program to manage its approved Merchant Category Codes
            </CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : !programs || programs.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground">
                No programs found. Create your first program to get started.
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-12">
                      <Button variant="ghost" size="sm" onClick={toggleSelectAll}>
                        {selectedPrograms.length === programs?.length ? (
                          <CheckSquare className="h-4 w-4" />
                        ) : (
                          <Square className="h-4 w-4" />
                        )}
                      </Button>
                    </TableHead>
                    <TableHead>Program Name</TableHead>
                    <TableHead>Account Type</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {programs.map((program) => (
                    <TableRow key={program.id}>
                      <TableCell>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => toggleProgramSelection(program.id)}
                        >
                          {selectedPrograms.includes(program.id) ? (
                            <CheckSquare className="h-4 w-4" />
                          ) : (
                            <Square className="h-4 w-4" />
                          )}
                        </Button>
                      </TableCell>
                      <TableCell className="font-medium">{program.name}</TableCell>
                      <TableCell>
                        <code className="bg-muted px-2 py-1 rounded text-sm">
                          {program.accountType}
                        </code>
                      </TableCell>
                      <TableCell>
                        <span
                          className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                            program.status === "active"
                              ? "bg-green-100 text-green-800"
                              : program.status === "inactive"
                              ? "bg-gray-100 text-gray-800"
                              : "bg-yellow-100 text-yellow-800"
                          }`}
                        >
                          {program.status}
                        </span>
                      </TableCell>
                      <TableCell>{new Date(program.createdAt).toLocaleDateString()}</TableCell>
                      <TableCell className="text-right">
                        <Link href={`/programs/${program.id}`}>
                          <Button variant="outline" size="sm">
                            Manage MCCs
                          </Button>
                        </Link>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Batch Update Dialog */}
      <Dialog open={isBatchDialogOpen} onOpenChange={setIsBatchDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {batchAction === "activate" ? "Activate" : "Deactivate"} {selectedPrograms.length} Program(s)
            </DialogTitle>
            <DialogDescription>
              This action will {batchAction === "activate" ? "activate" : "deactivate"} the selected programs.
              Please provide a justification for this change.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="batch-justification">Justification (minimum 10 characters) *</Label>
              <Textarea
                id="batch-justification"
                placeholder="Explain why these programs are being updated..."
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
              onClick={handleBatchUpdate} 
              disabled={batchUpdate.isPending || batchJustification.length < 10}
            >
              {batchUpdate.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
              Confirm {batchAction === "activate" ? "Activation" : "Deactivation"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DashboardLayout>
  );
}
