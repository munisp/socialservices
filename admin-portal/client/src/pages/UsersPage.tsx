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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { trpc } from "@/lib/trpc";
import { Settings, Flag, Calendar, Shield, Users, Loader2, History, ShieldCheck, Search, Filter, X, Download } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

export default function UsersPage() {
  const [isRoleDialogOpen, setIsRoleDialogOpen] = useState(false);
  const [selectedUser, setSelectedUser] = useState<any>(null);
  const [newRole, setNewRole] = useState<"admin" | "user">("user");
  const [justification, setJustification] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [actionTypeFilter, setActionTypeFilter] = useState<string>("");

  const { data: users, isLoading: usersLoading, refetch: refetchUsers } = trpc.users.list.useQuery();
  const { data: auditLog, refetch: refetchAudit } = trpc.adminAudit.list.useQuery(
    {
      startDate: startDate || undefined,
      endDate: endDate || undefined,
      actionType: actionTypeFilter || undefined,
    },
    {
      enabled: true,
    }
  );

  const updateRole = trpc.users.updateRole.useMutation({
    onSuccess: () => {
      toast.success("User role updated successfully");
      setIsRoleDialogOpen(false);
      setSelectedUser(null);
      setJustification("");
      refetchUsers();
      refetchAudit();
    },
    onError: (error) => {
      toast.error(`Failed to update role: ${error.message}`);
    },
  });

  const handleRoleChange = () => {
    if (!selectedUser || !justification) {
      toast.error("Justification is required");
      return;
    }
    updateRole.mutate({
      userId: selectedUser.id,
      role: newRole,
      justification,
    });
  };

  const openRoleDialog = (user: any) => {
    setSelectedUser(user);
    setNewRole(user.role === "admin" ? "user" : "admin");
    setIsRoleDialogOpen(true);
  };

  const clearFilters = () => {
    setStartDate("");
    setEndDate("");
    setActionTypeFilter("");
  };

  const hasActiveFilters = startDate || endDate || actionTypeFilter;

  // Get unique action types from audit log
  const actionTypes = Array.from(new Set(auditLog?.map((entry) => entry.action) || []));

  const exportToCSV = () => {
    if (!auditLog || auditLog.length === 0) {
      toast.error("No data to export");
      return;
    }

    // Create CSV content
    const headers = ["ID", "Action", "Performed By", "Target User ID", "Justification", "Performed At"];
    const rows = auditLog.map((entry) => [
      entry.id,
      entry.action,
      entry.performedBy,
      entry.targetUserId || "N/A",
      entry.justification?.replace(/"/g, '""') || "N/A",
      new Date(entry.performedAt).toLocaleString(),
    ]);

    const csvContent = [
      headers.join(","),
      ...rows.map((row) => row.map((cell) => `"${cell}"`).join(",")),
    ].join("\n");

    // Create download link
    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    const url = URL.createObjectURL(blob);
    link.setAttribute("href", url);
    link.setAttribute("download", `audit-log-${new Date().toISOString().split("T")[0]}.csv`);
    link.style.visibility = "hidden";
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);

    toast.success(`Exported ${auditLog.length} audit entries`);
  };

  const filteredUsers = users?.filter(
    (user) =>
      user.name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
      user.email?.toLowerCase().includes(searchQuery.toLowerCase())
  );

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
          <h1 className="text-3xl font-bold tracking-tight">User Management</h1>
          <p className="text-muted-foreground mt-2">Manage user roles and view audit logs of administrative actions</p>
        </div>

        <Tabs defaultValue="users" className="space-y-4">
          <TabsList>
            <TabsTrigger value="users">
              <Users className="h-4 w-4 mr-2" />
              All Users
            </TabsTrigger>
            <TabsTrigger value="audit">
              <History className="h-4 w-4 mr-2" />
              Audit Log
            </TabsTrigger>
          </TabsList>

          <TabsContent value="users">
            <Card>
              <CardHeader>
                <CardTitle>Platform Users</CardTitle>
                <CardDescription>View and manage user roles across the platform</CardDescription>
                <div className="mt-4">
                  <div className="relative">
                    <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                    <Input
                      placeholder="Search by name or email..."
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      className="pl-10"
                    />
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                {usersLoading ? (
                  <div className="flex items-center justify-center py-8">
                    <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                  </div>
                ) : !filteredUsers || filteredUsers.length === 0 ? (
                  <div className="text-center py-8 text-muted-foreground">
                    {searchQuery ? "No users found matching your search." : "No users found."}
                  </div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Name</TableHead>
                        <TableHead>Email</TableHead>
                        <TableHead>Role</TableHead>
                        <TableHead>Login Method</TableHead>
                        <TableHead>Last Sign In</TableHead>
                        <TableHead className="text-right">Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {filteredUsers.map((user) => (
                        <TableRow key={user.id}>
                          <TableCell className="font-medium">{user.name || "—"}</TableCell>
                          <TableCell>{user.email || "—"}</TableCell>
                          <TableCell>
                            <div className="flex items-center gap-2">
                              {user.role === "admin" && <ShieldCheck className="h-4 w-4 text-orange-600" />}
                              <span
                                className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                                  user.role === "admin"
                                    ? "bg-orange-100 text-orange-800"
                                    : "bg-gray-100 text-gray-800"
                                }`}
                              >
                                {user.role}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell>{user.loginMethod || "—"}</TableCell>
                          <TableCell>{user.lastSignedIn ? new Date(user.lastSignedIn).toLocaleDateString() : "—"}</TableCell>
                          <TableCell className="text-right">
                            <Button variant="outline" size="sm" onClick={() => openRoleDialog(user)}>
                              Change Role
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
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle>Admin Action Audit Log</CardTitle>
                    <CardDescription>Complete history of all administrative actions performed on the platform</CardDescription>
                  </div>
                  <Button onClick={exportToCSV} variant="outline" size="sm" disabled={!auditLog || auditLog.length === 0}>
                    <Download className="h-4 w-4 mr-2" />
                    Export CSV
                  </Button>
                </div>
                <div className="mt-4 space-y-4">
                  <div className="flex items-center gap-2">
                    <Filter className="h-4 w-4 text-muted-foreground" />
                    <span className="text-sm font-medium">Filters</span>
                    {hasActiveFilters && (
                      <Button variant="ghost" size="sm" onClick={clearFilters} className="h-7 px-2">
                        <X className="h-3 w-3 mr-1" />
                        Clear
                      </Button>
                    )}
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <div className="space-y-2">
                      <Label htmlFor="startDate" className="text-sm">
                        Start Date
                      </Label>
                      <Input
                        id="startDate"
                        type="date"
                        value={startDate}
                        onChange={(e) => setStartDate(e.target.value)}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="endDate" className="text-sm">
                        End Date
                      </Label>
                      <Input
                        id="endDate"
                        type="date"
                        value={endDate}
                        onChange={(e) => setEndDate(e.target.value)}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="actionType" className="text-sm">
                        Action Type
                      </Label>
                      <Select value={actionTypeFilter} onValueChange={setActionTypeFilter}>
                        <SelectTrigger>
                          <SelectValue placeholder="All actions" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="">All actions</SelectItem>
                          {actionTypes.map((type) => (
                            <SelectItem key={type} value={type}>
                              {type.replace(/_/g, " ")}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                  {hasActiveFilters && (
                    <div className="text-sm text-muted-foreground">
                      Showing {auditLog?.length || 0} filtered result(s)
                    </div>
                  )}
                </div>
              </CardHeader>
              <CardContent>
                {!auditLog || auditLog.length === 0 ? (
                  <div className="text-center py-8 text-muted-foreground">No audit entries yet</div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Action</TableHead>
                        <TableHead>Target User</TableHead>
                        <TableHead>Details</TableHead>
                        <TableHead>Justification</TableHead>
                        <TableHead>Performed At</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {auditLog.map((entry) => {
                        const details = entry.details as any;
                        return (
                          <TableRow key={entry.id}>
                            <TableCell>
                              <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
                                {entry.action.replace(/_/g, " ")}
                              </span>
                            </TableCell>
                            <TableCell>
                              <div>
                                <div className="font-medium">{details?.targetUserName || "—"}</div>
                                <div className="text-sm text-muted-foreground">{details?.targetUserEmail || "—"}</div>
                              </div>
                            </TableCell>
                            <TableCell>
                              {entry.action === "role_change" && details && (
                                <div className="text-sm">
                                  <span className="text-muted-foreground">{details.oldRole}</span>
                                  {" → "}
                                  <span className="font-medium">{details.newRole}</span>
                                </div>
                              )}
                            </TableCell>
                            <TableCell className="max-w-md truncate">{entry.justification || "—"}</TableCell>
                            <TableCell>{new Date(entry.performedAt).toLocaleString()}</TableCell>
                          </TableRow>
                        );
                      })}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>

        <Dialog open={isRoleDialogOpen} onOpenChange={setIsRoleDialogOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Change User Role</DialogTitle>
              <DialogDescription>
                Update the role for <span className="font-medium">{selectedUser?.name || selectedUser?.email}</span>
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Current Role</Label>
                <div className="text-sm text-muted-foreground">
                  <span
                    className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                      selectedUser?.role === "admin" ? "bg-orange-100 text-orange-800" : "bg-gray-100 text-gray-800"
                    }`}
                  >
                    {selectedUser?.role}
                  </span>
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="newRole">New Role *</Label>
                <Select value={newRole} onValueChange={(value: any) => setNewRole(value)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="user">User</SelectItem>
                    <SelectItem value="admin">Admin</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="justification">Justification *</Label>
                <Textarea
                  id="justification"
                  placeholder="Explain why this role change is necessary..."
                  value={justification}
                  onChange={(e) => setJustification(e.target.value)}
                />
              </div>
            </div>
            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => {
                  setIsRoleDialogOpen(false);
                  setJustification("");
                }}
              >
                Cancel
              </Button>
              <Button onClick={handleRoleChange} disabled={updateRole.isPending}>
                {updateRole.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                Update Role
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </DashboardLayout>
  );
}
