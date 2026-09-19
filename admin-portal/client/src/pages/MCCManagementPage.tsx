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
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { trpc } from "@/lib/trpc";
import { Settings, Flag, Calendar, Shield, Users, Search, Edit, Trash2, Loader2, ChevronLeft, ChevronRight } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

export default function MCCManagementPage() {
  const [searchQuery, setSearchQuery] = useState("");
  const [page, setPage] = useState(1);
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const [selectedMcc, setSelectedMcc] = useState<any>(null);
  const [editForm, setEditForm] = useState({ mccCode: "", description: "", category: "" });

  const { data: mccData, refetch } = trpc.mccDatabase.list.useQuery({
    query: searchQuery,
    page,
    pageSize: 50,
  });

  const updateMcc = trpc.mccDatabase.update.useMutation({
    onSuccess: () => {
      toast.success("MCC updated successfully");
      setIsEditDialogOpen(false);
      refetch();
    },
    onError: (error) => {
      toast.error(`Failed to update MCC: ${error.message}`);
    },
  });

  const deleteMcc = trpc.mccDatabase.delete.useMutation({
    onSuccess: () => {
      toast.success("MCC deleted successfully");
      setIsDeleteDialogOpen(false);
      refetch();
    },
    onError: (error) => {
      toast.error(`Failed to delete MCC: ${error.message}`);
    },
  });

  const openEditDialog = (mcc: any) => {
    setSelectedMcc(mcc);
    setEditForm({
      mccCode: mcc.mccCode,
      description: mcc.description,
      category: mcc.category || "",
    });
    setIsEditDialogOpen(true);
  };

  const openDeleteDialog = (mcc: any) => {
    setSelectedMcc(mcc);
    setIsDeleteDialogOpen(true);
  };

  const handleUpdate = () => {
    if (!selectedMcc) return;
    updateMcc.mutate({
      id: selectedMcc.id,
      ...editForm,
    });
  };

  const handleDelete = () => {
    if (!selectedMcc) return;
    deleteMcc.mutate({ id: selectedMcc.id });
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    refetch();
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
          <h1 className="text-3xl font-bold tracking-tight">MCC Database Management</h1>
          <p className="text-muted-foreground mt-2">Search, edit, and manage Merchant Category Codes</p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Search MCC Database</CardTitle>
            <CardDescription>Find MCC entries by code or description</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSearch} className="flex gap-2">
              <div className="relative flex-1">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search by code or description..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-10"
                />
              </div>
              <Button type="submit">Search</Button>
            </form>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle>MCC Entries</CardTitle>
                <CardDescription>
                  {mccData ? `Showing ${mccData.results.length} of ${mccData.total} entries` : "Loading..."}
                </CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>MCC Code</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead>Category</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {mccData?.results.map((mcc) => (
                  <TableRow key={mcc.id}>
                    <TableCell>
                      <code className="bg-muted px-2 py-1 rounded font-mono">{mcc.mccCode}</code>
                    </TableCell>
                    <TableCell>{mcc.description}</TableCell>
                    <TableCell>{mcc.category || "—"}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button variant="ghost" size="sm" onClick={() => openEditDialog(mcc)}>
                          <Edit className="h-4 w-4" />
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => openDeleteDialog(mcc)}>
                          <Trash2 className="h-4 w-4 text-destructive" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
                {mccData?.results.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-muted-foreground py-8">
                      No MCC entries found
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>

            {mccData && mccData.totalPages > 1 && (
              <div className="flex items-center justify-between mt-4">
                <div className="text-sm text-muted-foreground">
                  Page {mccData.page} of {mccData.totalPages}
                </div>
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" onClick={() => setPage(page - 1)} disabled={page === 1}>
                    <ChevronLeft className="h-4 w-4" />
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage(page + 1)}
                    disabled={page >= mccData.totalPages}
                  >
                    Next
                    <ChevronRight className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Edit Dialog */}
      <Dialog open={isEditDialogOpen} onOpenChange={setIsEditDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit MCC Entry</DialogTitle>
            <DialogDescription>Update the details of this Merchant Category Code</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label htmlFor="mccCode">MCC Code</Label>
              <Input
                id="mccCode"
                value={editForm.mccCode}
                onChange={(e) => setEditForm({ ...editForm, mccCode: e.target.value })}
              />
            </div>
            <div>
              <Label htmlFor="description">Description</Label>
              <Input
                id="description"
                value={editForm.description}
                onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
              />
            </div>
            <div>
              <Label htmlFor="category">Category (Optional)</Label>
              <Input
                id="category"
                value={editForm.category}
                onChange={(e) => setEditForm({ ...editForm, category: e.target.value })}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsEditDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleUpdate} disabled={updateMcc.isPending}>
              {updateMcc.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
              Save Changes
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Are you sure?</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete the MCC entry <strong>{selectedMcc?.mccCode}</strong>. This action cannot be
              undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete} disabled={deleteMcc.isPending} className="bg-destructive">
              {deleteMcc.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </DashboardLayout>
  );
}
