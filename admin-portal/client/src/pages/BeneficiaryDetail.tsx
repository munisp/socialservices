import { useParams } from "wouter";
import { useAuth } from "@/_core/hooks/useAuth";
import { trpc } from "@/lib/trpc";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Loader2, User, FileText, CreditCard, AlertTriangle, CheckCircle2, XCircle, Clock } from "lucide-react";
import { toast } from "sonner";

export default function BeneficiaryDetail() {
  const { id } = useParams();
  const { user } = useAuth();
  
  // Fetch beneficiary data
  const { data: beneficiary, isLoading, refetch } = trpc.beneficiaries.getById.useQuery(
    { id: id! },
    { enabled: !!id }
  );
  
  const approveMutation = trpc.beneficiaries.approve.useMutation({
    onSuccess: () => {
      toast.success("Beneficiary approved");
      refetch();
    },
  });
  
  const rejectMutation = trpc.beneficiaries.reject.useMutation({
    onSuccess: () => {
      toast.success("Beneficiary rejected");
      refetch();
    },
  });
  
  const suspendMutation = trpc.beneficiaries.suspend.useMutation({
    onSuccess: () => {
      toast.success("Beneficiary suspended");
      refetch();
    },
  });
  
  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <Loader2 className="h-8 w-8 animate-spin" />
      </div>
    );
  }
  
  if (!beneficiary) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <p>Beneficiary not found</p>
      </div>
    );
  }
  
  const getStatusBadge = (status: string) => {
    const variants: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
      active: "default",
      pending: "secondary",
      suspended: "destructive",
      rejected: "outline",
    };
    
    return <Badge variant={variants[status] || "outline"}>{status}</Badge>;
  };
  
  return (
    <div className="container mx-auto py-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Avatar className="h-16 w-16">
            <AvatarFallback>
              <User className="h-8 w-8" />
            </AvatarFallback>
          </Avatar>
          <div>
            <h1 className="text-3xl font-bold">{beneficiary.name}</h1>
            <p className="text-muted-foreground">ID: {beneficiary.id}</p>
          </div>
        </div>
        <div className="flex gap-2">
          {beneficiary.status === "pending" && (
            <>
              <Button
                onClick={() => approveMutation.mutate({ id: beneficiary.id })}
                disabled={approveMutation.isLoading}
              >
                <CheckCircle2 className="mr-2 h-4 w-4" />
                Approve
              </Button>
              <Button
                variant="destructive"
                onClick={() => rejectMutation.mutate({ id: beneficiary.id })}
                disabled={rejectMutation.isLoading}
              >
                <XCircle className="mr-2 h-4 w-4" />
                Reject
              </Button>
            </>
          )}
          {beneficiary.status === "active" && (
            <Button
              variant="outline"
              onClick={() => suspendMutation.mutate({ id: beneficiary.id })}
              disabled={suspendMutation.isLoading}
            >
              <AlertTriangle className="mr-2 h-4 w-4" />
              Suspend
            </Button>
          )}
        </div>
      </div>
      
      {/* Overview Cards */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Status</CardDescription>
          </CardHeader>
          <CardContent>
            {getStatusBadge(beneficiary.status)}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Programs</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{beneficiary.programs?.length || 0}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Total Disbursed</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">${beneficiary.totalDisbursed || 0}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Fraud Alerts</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold text-destructive">{beneficiary.fraudAlerts || 0}</p>
          </CardContent>
        </Card>
      </div>
      
      {/* Detailed Information */}
      <Tabs defaultValue="info" className="w-full">
        <TabsList>
          <TabsTrigger value="info">Personal Information</TabsTrigger>
          <TabsTrigger value="programs">Programs & Benefits</TabsTrigger>
          <TabsTrigger value="kyc">KYC Documents</TabsTrigger>
          <TabsTrigger value="card">Benefit Card</TabsTrigger>
          <TabsTrigger value="transactions">Transactions</TabsTrigger>
          <TabsTrigger value="alerts">Fraud Alerts</TabsTrigger>
        </TabsList>
        
        <TabsContent value="info" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Personal Information</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-4 md:grid-cols-2">
              <div>
                <p className="text-sm text-muted-foreground">Full Name</p>
                <p className="font-medium">{beneficiary.name}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Email</p>
                <p className="font-medium">{beneficiary.email || "N/A"}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Phone</p>
                <p className="font-medium">{beneficiary.phone || "N/A"}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Date of Birth</p>
                <p className="font-medium">{beneficiary.dateOfBirth || "N/A"}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Address</p>
                <p className="font-medium">{beneficiary.address || "N/A"}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Enrollment Date</p>
                <p className="font-medium">{new Date(beneficiary.createdAt).toLocaleDateString()}</p>
              </div>
            </CardContent>
          </Card>
          
          <Card>
            <CardHeader>
              <CardTitle>Enrollment History</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-2">
                {beneficiary.enrollmentHistory?.map((entry: any, index: number) => (
                  <div key={index} className="flex items-center gap-2">
                    <Clock className="h-4 w-4 text-muted-foreground" />
                    <span className="text-sm">{entry.date}: {entry.action}</span>
                  </div>
                )) || <p className="text-sm text-muted-foreground">No history available</p>}
              </div>
            </CardContent>
          </Card>
        </TabsContent>
        
        <TabsContent value="programs">
          <Card>
            <CardHeader>
              <CardTitle>Linked Programs</CardTitle>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Program Name</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Enrolled Date</TableHead>
                    <TableHead>Monthly Benefit</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {beneficiary.programs?.map((program: any) => (
                    <TableRow key={program.id}>
                      <TableCell>{program.name}</TableCell>
                      <TableCell>{getStatusBadge(program.status)}</TableCell>
                      <TableCell>{new Date(program.enrolledDate).toLocaleDateString()}</TableCell>
                      <TableCell>${program.monthlyBenefit}</TableCell>
                    </TableRow>
                  )) || (
                    <TableRow>
                      <TableCell colSpan={4} className="text-center text-muted-foreground">
                        No programs linked
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
        
        <TabsContent value="kyc">
          <Card>
            <CardHeader>
              <CardTitle>KYC Documents</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4 md:grid-cols-2">
                {beneficiary.kycDocuments?.map((doc: any) => (
                  <Card key={doc.id}>
                    <CardHeader className="pb-2">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <FileText className="h-4 w-4" />
                          <CardTitle className="text-sm">{doc.type}</CardTitle>
                        </div>
                        {doc.verified && <CheckCircle2 className="h-4 w-4 text-green-500" />}
                      </div>
                    </CardHeader>
                    <CardContent>
                      <p className="text-xs text-muted-foreground mb-2">Uploaded: {new Date(doc.uploadedAt).toLocaleDateString()}</p>
                      <div className="flex gap-2">
                        <Button size="sm" variant="outline">View</Button>
                        <Button size="sm" variant="outline">Download</Button>
                      </div>
                    </CardContent>
                  </Card>
                )) || <p className="text-sm text-muted-foreground">No documents uploaded</p>}
              </div>
            </CardContent>
          </Card>
        </TabsContent>
        
        <TabsContent value="card">
          <Card>
            <CardHeader>
              <CardTitle>Benefit Card Details</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {beneficiary.card ? (
                <>
                  <div className="grid gap-4 md:grid-cols-2">
                    <div>
                      <p className="text-sm text-muted-foreground">Card Number</p>
                      <p className="font-mono">**** **** **** {beneficiary.card.lastFour}</p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Status</p>
                      {getStatusBadge(beneficiary.card.status)}
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Issued Date</p>
                      <p>{new Date(beneficiary.card.issuedDate).toLocaleDateString()}</p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Expiry Date</p>
                      <p>{new Date(beneficiary.card.expiryDate).toLocaleDateString()}</p>
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <Button variant="outline">
                      <CreditCard className="mr-2 h-4 w-4" />
                      Block Card
                    </Button>
                    <Button variant="outline">
                      <CreditCard className="mr-2 h-4 w-4" />
                      Reissue Card
                    </Button>
                  </div>
                </>
              ) : (
                <div className="text-center py-8">
                  <CreditCard className="h-12 w-12 mx-auto mb-4 text-muted-foreground" />
                  <p className="text-muted-foreground mb-4">No benefit card issued</p>
                  <Button>Issue Card</Button>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
        
        <TabsContent value="transactions">
          <Card>
            <CardHeader>
              <CardTitle>Transaction History</CardTitle>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Date</TableHead>
                    <TableHead>Merchant</TableHead>
                    <TableHead>Amount</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {beneficiary.transactions?.map((txn: any) => (
                    <TableRow key={txn.id}>
                      <TableCell>{new Date(txn.date).toLocaleString()}</TableCell>
                      <TableCell>{txn.merchant}</TableCell>
                      <TableCell>${txn.amount}</TableCell>
                      <TableCell>{getStatusBadge(txn.status)}</TableCell>
                    </TableRow>
                  )) || (
                    <TableRow>
                      <TableCell colSpan={4} className="text-center text-muted-foreground">
                        No transactions found
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
        
        <TabsContent value="alerts">
          <Card>
            <CardHeader>
              <CardTitle>Fraud Alerts</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {beneficiary.fraudAlerts?.map((alert: any) => (
                  <Card key={alert.id} className="border-destructive">
                    <CardHeader className="pb-2">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <AlertTriangle className="h-4 w-4 text-destructive" />
                          <CardTitle className="text-sm">{alert.type}</CardTitle>
                        </div>
                        <Badge variant="destructive">{alert.severity}</Badge>
                      </div>
                    </CardHeader>
                    <CardContent>
                      <p className="text-sm mb-2">{alert.description}</p>
                      <p className="text-xs text-muted-foreground">Detected: {new Date(alert.detectedAt).toLocaleString()}</p>
                    </CardContent>
                  </Card>
                )) || <p className="text-sm text-muted-foreground">No fraud alerts</p>}
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
