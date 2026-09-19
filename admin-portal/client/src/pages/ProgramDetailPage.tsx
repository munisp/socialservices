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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { trpc } from "@/lib/trpc";
import { Settings, Flag, Calendar, Shield, Plus, Trash2, Loader2, ArrowLeft, History, Save } from "lucide-react";
import { useState } from "react";
import { Link, useParams } from "wouter";
import { toast } from "sonner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

export default function ProgramDetailPage() {
  const params = useParams();
  const programId = parseInt(params.id || "0");

  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false);
  const [isRemoveDialogOpen, setIsRemoveDialogOpen] = useState(false);
  const [selectedMcc, setSelectedMcc] = useState<string | null>(null);
  const [newMcc, setNewMcc] = useState({
    mccCode: "",
    mccDescription: "",
    justification: "",
  });
  const [removeJustification, setRemoveJustification] = useState("");
  const [showSaveTemplateDialog, setShowSaveTemplateDialog] = useState(false);
  const [templateName, setTemplateName] = useState("");
  const [templateDescription, setTemplateDescription] = useState("");

  const { data: program, isLoading: programLoading } = trpc.programs.getById.useQuery({ id: programId });
  const { data: mccRules, isLoading: mccLoading, refetch: refetchMcc } = trpc.mccRules.getByProgramId.useQuery({ programId });
  const { data: auditLog, refetch: refetchAudit } = trpc.mccRules.getAudit.useQuery({ programId });

  const addMcc = trpc.mccRules.add.useMutation({
    onSuccess: () => {
      toast.success("MCC added successfully");
      setIsAddDialogOpen(false);
      setNewMcc({ mccCode: "", mccDescription: "", justification: "" });
      refetchMcc();
      refetchAudit();
    },
    onError: (error) => {
      toast.error(`Failed to add MCC: ${error.message}`);
    },
  });

  const saveTemplateMutation = trpc.programTemplates.create.useMutation({
    onSuccess: () => {
      setShowSaveTemplateDialog(false);
      setTemplateName("");
      setTemplateDescription("");
      toast.success("Program saved as template");
    },
    onError: (error) => {
      toast.error(`Failed to save template: ${error.message}`);
    },
  });

  const removeMcc = trpc.mccRules.remove.useMutation({
    onSuccess: () => {
      toast.success("MCC removed successfully");
      setIsRemoveDialogOpen(false);
      setSelectedMcc(null);
      setRemoveJustification("");
      refetchMcc();
      refetchAudit();
    },
    onError: (error) => {
      toast.error(`Failed to remove MCC: ${error.message}`);
    },
  });

  const handleAdd = () => {
    if (!newMcc.mccCode || !newMcc.justification) {
      toast.error("MCC Code and Justification are required");
      return;
    }
    addMcc.mutate({
      programId,
      ...newMcc,
    });
  };

  const handleRemove = () => {
    if (!selectedMcc || !removeJustification) {
      toast.error("Justification is required");
      return;
    }
    removeMcc.mutate({
      programId,
      mccCode: selectedMcc,
      justification: removeJustification,
    });
  };

  const handleSaveAsTemplate = () => {
    if (!templateName.trim()) {
      toast.error("Template name is required");
      return;
    }
    saveTemplateMutation.mutate({
      programId,
      templateName: templateName.trim(),
      templateDescription: templateDescription.trim() || undefined,
    });
  };

  if (programLoading) {
    return (
      <DashboardLayout
        items={[
          { label: "Dashboard", href: "/", icon: Shield },
          { label: "Programs & MCC Rules", href: "/programs", icon: Settings },
          { label: "Feature Flags", href: "/feature-flags", icon: Flag },
          { label: "Disbursements", href: "/disbursements", icon: Calendar },
        ]}
      >
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </DashboardLayout>
    );
  }

  if (!program) {
    return (
      <DashboardLayout
        items={[
          { label: "Dashboard", href: "/", icon: Shield },
          { label: "Programs & MCC Rules", href: "/programs", icon: Settings },
          { label: "Feature Flags", href: "/feature-flags", icon: Flag },
          { label: "Disbursements", href: "/disbursements", icon: Calendar },
        ]}
      >
        <div className="text-center py-12">
          <p className="text-muted-foreground">Program not found</p>
          <Link href="/programs">
            <Button variant="outline" className="mt-4">
              Back to Programs
            </Button>
          </Link>
        </div>
      </DashboardLayout>
    );
  }

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
        <div>
          <Link href="/programs">
            <Button variant="ghost" size="sm" className="mb-4">
              <ArrowLeft className="h-4 w-4 mr-2" />
              Back to Programs
            </Button>
          </Link>
          <div className="flex items-start justify-between">
            <div>
              <h1 className="text-3xl font-bold tracking-tight">{program.name}</h1>
              <p className="text-muted-foreground mt-2">
                Account Type: <code className="bg-muted px-2 py-1 rounded">{program.accountType}</code>
              </p>
              {program.description && <p className="text-sm text-muted-foreground mt-2">{program.description}</p>}
            </div>
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => setShowSaveTemplateDialog(true)}>
                <Save className="h-4 w-4 mr-2" />
                Save as Template
              </Button>
              <Dialog open={isAddDialogOpen} onOpenChange={setIsAddDialogOpen}>
                <DialogTrigger asChild>
                  <Button>
                    <Plus className="h-4 w-4 mr-2" />
                    Add MCC
                  </Button>
                </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Add Approved MCC</DialogTitle>
                  <DialogDescription>
                    Add a new Merchant Category Code to the approved list for this program
                  </DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label htmlFor="mccCode">MCC Code *</Label>
                    <Input
                      id="mccCode"
                      placeholder="e.g., 5411"
                      value={newMcc.mccCode}
                      onChange={(e) => setNewMcc({ ...newMcc, mccCode: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="mccDescription">MCC Description</Label>
                    <Input
                      id="mccDescription"
                      placeholder="e.g., Grocery Stores"
                      value={newMcc.mccDescription}
                      onChange={(e) => setNewMcc({ ...newMcc, mccDescription: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="justification">Justification *</Label>
                    <Textarea
                      id="justification"
                      placeholder="Explain why this MCC should be approved..."
                      value={newMcc.justification}
                      onChange={(e) => setNewMcc({ ...newMcc, justification: e.target.value })}
                    />
                  </div>
                </div>
                <DialogFooter>
                  <Button variant="outline" onClick={() => setIsAddDialogOpen(false)}>
                    Cancel
                  </Button>
                  <Button onClick={handleAdd} disabled={addMcc.isPending}>
                    {addMcc.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                    Add MCC
                  </Button>
                </DialogFooter>
              </DialogContent>
              </Dialog>
            </div>
          </div>
        </div>

        <Dialog open={showSaveTemplateDialog} onOpenChange={setShowSaveTemplateDialog}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Save as Template</DialogTitle>
              <DialogDescription>
                Save this program and its MCC rules as a template for quick duplication
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="templateName">Template Name *</Label>
                <Input
                  id="templateName"
                  placeholder="e.g., Food Assistance Program Template"
                  value={templateName}
                  onChange={(e) => setTemplateName(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="templateDescription">Description</Label>
                <Textarea
                  id="templateDescription"
                  placeholder="Describe this template..."
                  value={templateDescription}
                  onChange={(e) => setTemplateDescription(e.target.value)}
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setShowSaveTemplateDialog(false)}>
                Cancel
              </Button>
              <Button onClick={handleSaveAsTemplate} disabled={saveTemplateMutation.isPending}>
                {saveTemplateMutation.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                Save Template
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Tabs defaultValue="mccs" className="space-y-4">
          <TabsList>
            <TabsTrigger value="mccs">Approved MCCs</TabsTrigger>
            <TabsTrigger value="audit">
              <History className="h-4 w-4 mr-2" />
              Audit Log
            </TabsTrigger>
          </TabsList>

          <TabsContent value="mccs">
            <Card>
              <CardHeader>
                <CardTitle>Approved Merchant Category Codes</CardTitle>
                <CardDescription>
                  MCCs in this list are eligible for spending with {program.accountType} funds
                </CardDescription>
              </CardHeader>
              <CardContent>
                {mccLoading ? (
                  <div className="flex items-center justify-center py-8">
                    <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                  </div>
                ) : !mccRules || mccRules.length === 0 ? (
                  <div className="text-center py-8 text-muted-foreground">
                    No MCCs approved yet. Add your first MCC to get started.
                  </div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>MCC Code</TableHead>
                        <TableHead>Description</TableHead>
                        <TableHead>Added</TableHead>
                        <TableHead className="text-right">Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {mccRules.map((rule) => (
                        <TableRow key={rule.id}>
                          <TableCell>
                            <code className="bg-muted px-2 py-1 rounded font-mono">{rule.mccCode}</code>
                          </TableCell>
                          <TableCell>{rule.mccDescription || "—"}</TableCell>
                          <TableCell>{new Date(rule.createdAt).toLocaleDateString()}</TableCell>
                          <TableCell className="text-right">
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => {
                                setSelectedMcc(rule.mccCode);
                                setIsRemoveDialogOpen(true);
                              }}
                            >
                              <Trash2 className="h-4 w-4 text-destructive" />
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="audit">
            <Card>
              <CardHeader>
                <CardTitle>Audit Trail</CardTitle>
                <CardDescription>Complete history of changes to this program's MCC rules</CardDescription>
              </CardHeader>
              <CardContent>
                {!auditLog || auditLog.length === 0 ? (
                  <div className="text-center py-8 text-muted-foreground">No audit entries yet</div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Action</TableHead>
                        <TableHead>MCC Code</TableHead>
                        <TableHead>Justification</TableHead>
                        <TableHead>Date</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {auditLog.map((entry) => (
                        <TableRow key={entry.id}>
                          <TableCell>
                            <span
                              className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                                entry.action === "add_mcc"
                                  ? "bg-green-100 text-green-800"
                                  : "bg-red-100 text-red-800"
                              }`}
                            >
                              {entry.action === "add_mcc" ? "Added" : "Removed"}
                            </span>
                          </TableCell>
                          <TableCell>
                            <code className="bg-muted px-2 py-1 rounded font-mono">{entry.mccCode || "—"}</code>
                          </TableCell>
                          <TableCell className="max-w-md truncate">{entry.justification}</TableCell>
                          <TableCell>{new Date(entry.performedAt).toLocaleString()}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>

        <Dialog open={isRemoveDialogOpen} onOpenChange={setIsRemoveDialogOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Remove MCC</DialogTitle>
              <DialogDescription>
                Are you sure you want to remove MCC <code className="bg-muted px-2 py-1 rounded">{selectedMcc}</code>?
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="removeJustification">Justification *</Label>
                <Textarea
                  id="removeJustification"
                  placeholder="Explain why this MCC should be removed..."
                  value={removeJustification}
                  onChange={(e) => setRemoveJustification(e.target.value)}
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setIsRemoveDialogOpen(false)}>
                Cancel
              </Button>
              <Button variant="destructive" onClick={handleRemove} disabled={removeMcc.isPending}>
                {removeMcc.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                Remove MCC
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </DashboardLayout>
  );
}
