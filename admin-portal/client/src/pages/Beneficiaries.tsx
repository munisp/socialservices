import { useState } from "react";
import { trpc } from "@/lib/trpc";
import DashboardLayout from "@/components/DashboardLayout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Search, UserPlus, CheckCircle, XCircle, Eye } from "lucide-react";
import { useLocation } from "wouter";
import { toast } from "sonner";

export default function Beneficiaries() {
  const [, setLocation] = useLocation();
  const [searchQuery, setSearchQuery] = useState("");
  const utils = trpc.useUtils();

  const { data: beneficiaries, isLoading } = trpc.beneficiaries.list.useQuery();
  const { data: stats } = trpc.beneficiaries.getStats.useQuery();
  const { data: searchResults } = trpc.beneficiaries.search.useQuery(
    { query: searchQuery },
    { enabled: searchQuery.length > 0 }
  );

  const approveMutation = trpc.beneficiaries.approve.useMutation({
    onSuccess: () => {
      toast.success("Beneficiary approved successfully");
      utils.beneficiaries.list.invalidate();
      utils.beneficiaries.getStats.invalidate();
    },
    onError: (error) => {
      toast.error(`Failed to approve: ${error.message}`);
    },
  });

  const rejectMutation = trpc.beneficiaries.reject.useMutation({
    onSuccess: () => {
      toast.success("Beneficiary rejected");
      utils.beneficiaries.list.invalidate();
      utils.beneficiaries.getStats.invalidate();
    },
    onError: (error) => {
      toast.error(`Failed to reject: ${error.message}`);
    },
  });

  const displayData = searchQuery.length > 0 ? searchResults : beneficiaries;

  const getStatusBadge = (status: string) => {
    const variants: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
      pending: "outline",
      approved: "default",
      rejected: "destructive",
      suspended: "secondary",
    };
    return <Badge variant={variants[status] || "default"}>{status}</Badge>;
  };

  const getKycBadge = (status: string) => {
    const variants: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
      not_started: "outline",
      in_progress: "secondary",
      completed: "default",
      failed: "destructive",
    };
    return <Badge variant={variants[status] || "default"}>{status.replace("_", " ")}</Badge>;
  };

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold">Beneficiary Management</h1>
            <p className="text-muted-foreground">
              Manage beneficiary enrollments, KYC verification, and benefit card issuance
            </p>
          </div>
          <Button onClick={() => setLocation("/beneficiaries/new")}>
            <UserPlus className="mr-2 h-4 w-4" />
            Add Beneficiary
          </Button>
        </div>

        {/* Statistics Cards */}
        {stats && (
          <div className="grid gap-4 md:grid-cols-5">
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Total Beneficiaries</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{stats.total}</div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Pending Approval</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{stats.pending}</div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Approved</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-green-600">{stats.approved}</div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Rejected</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-red-600">{stats.rejected}</div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Suspended</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-orange-600">{stats.suspended}</div>
              </CardContent>
            </Card>
          </div>
        )}

        {/* Search */}
        <Card>
          <CardHeader>
            <CardTitle>Search Beneficiaries</CardTitle>
            <CardDescription>
              Search by name, national ID, email, or phone number
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="relative">
              <Search className="absolute left-3 top-3 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search beneficiaries..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-10"
              />
            </div>
          </CardContent>
        </Card>

        {/* Beneficiaries Table */}
        <Card>
          <CardHeader>
            <CardTitle>All Beneficiaries</CardTitle>
            <CardDescription>
              {displayData?.length || 0} beneficiaries found
            </CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="text-center py-8 text-muted-foreground">Loading...</div>
            ) : !displayData || displayData.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground">
                No beneficiaries found
              </div>
            ) : (
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>ID</TableHead>
                      <TableHead>Name</TableHead>
                      <TableHead>National ID</TableHead>
                      <TableHead>Contact</TableHead>
                      <TableHead>Enrollment Status</TableHead>
                      <TableHead>KYC Status</TableHead>
                      <TableHead>Enrolled Date</TableHead>
                      <TableHead>Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {displayData.map((beneficiary: any) => (
                      <TableRow key={beneficiary.id}>
                        <TableCell className="font-medium">{beneficiary.id}</TableCell>
                        <TableCell>
                          {beneficiary.firstName} {beneficiary.lastName}
                        </TableCell>
                        <TableCell>{beneficiary.nationalId || "N/A"}</TableCell>
                        <TableCell>
                          <div className="text-sm">
                            <div>{beneficiary.email || "No email"}</div>
                            <div className="text-muted-foreground">
                              {beneficiary.phoneNumber || "No phone"}
                            </div>
                          </div>
                        </TableCell>
                        <TableCell>{getStatusBadge(beneficiary.enrollmentStatus)}</TableCell>
                        <TableCell>{getKycBadge(beneficiary.kycStatus)}</TableCell>
                        <TableCell>
                          {new Date(beneficiary.enrolledAt).toLocaleDateString()}
                        </TableCell>
                        <TableCell>
                          <div className="flex gap-2">
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => setLocation(`/beneficiaries/${beneficiary.id}`)}
                            >
                              <Eye className="h-4 w-4" />
                            </Button>
                            {beneficiary.enrollmentStatus === "pending" && (
                              <>
                                <Button
                                  size="sm"
                                  variant="default"
                                  onClick={() => approveMutation.mutate({ id: beneficiary.id })}
                                  disabled={approveMutation.isPending}
                                >
                                  <CheckCircle className="h-4 w-4" />
                                </Button>
                                <Button
                                  size="sm"
                                  variant="destructive"
                                  onClick={() => rejectMutation.mutate({ id: beneficiary.id })}
                                  disabled={rejectMutation.isPending}
                                >
                                  <XCircle className="h-4 w-4" />
                                </Button>
                              </>
                            )}
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
