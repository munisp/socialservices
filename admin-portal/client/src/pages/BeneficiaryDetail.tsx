import { useParams } from "wouter";
import DashboardLayout from "@/components/DashboardLayout";
import { trpc } from "@/lib/trpc";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Loader2, CheckCircle2, XCircle, AlertTriangle } from "lucide-react";
import { toast } from "sonner";

export default function BeneficiaryDetail() {
  const params = useParams<{ id: string }>();
  const beneficiaryId = Number(params.id);
  const enabled = Number.isInteger(beneficiaryId) && beneficiaryId > 0;
  const { data: beneficiary, isLoading, refetch } = trpc.beneficiaries.getById.useQuery(
    { id: beneficiaryId },
    { enabled }
  );
  const { data: enrollments = [] } = trpc.beneficiaries.getEnrollments.useQuery(
    { beneficiaryId },
    { enabled }
  );

  const approve = trpc.beneficiaries.approve.useMutation({ onSuccess: async () => { toast.success("Beneficiary approved"); await refetch(); } });
  const reject = trpc.beneficiaries.reject.useMutation({ onSuccess: async () => { toast.success("Beneficiary rejected"); await refetch(); } });
  const suspend = trpc.beneficiaries.suspend.useMutation({ onSuccess: async () => { toast.success("Beneficiary suspended"); await refetch(); } });

  if (!enabled) return <DashboardLayout><p className="p-6">Invalid beneficiary identifier.</p></DashboardLayout>;
  if (isLoading) return <DashboardLayout><div className="flex min-h-64 items-center justify-center"><Loader2 className="h-8 w-8 animate-spin" /></div></DashboardLayout>;
  if (!beneficiary) return <DashboardLayout><p className="p-6">Beneficiary not found.</p></DashboardLayout>;

  const status = beneficiary.enrollmentStatus;
  const statusVariant = status === "approved" ? "default" : status === "rejected" ? "destructive" : status === "suspended" ? "secondary" : "outline";
  const fullName = `${beneficiary.firstName} ${beneficiary.lastName}`;

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div><h1 className="text-3xl font-bold">{fullName}</h1><p className="text-muted-foreground">Beneficiary #{beneficiary.id}</p></div>
          <div className="flex gap-2">
            {status === "pending" && <>
              <Button onClick={() => approve.mutate({ id: beneficiary.id })} disabled={approve.isPending}><CheckCircle2 className="mr-2 h-4 w-4" />Approve</Button>
              <Button variant="destructive" onClick={() => reject.mutate({ id: beneficiary.id })} disabled={reject.isPending}><XCircle className="mr-2 h-4 w-4" />Reject</Button>
            </>}
            {status === "approved" && <Button variant="outline" onClick={() => suspend.mutate({ id: beneficiary.id })} disabled={suspend.isPending}><AlertTriangle className="mr-2 h-4 w-4" />Suspend</Button>}
          </div>
        </div>

        <div className="grid gap-4 md:grid-cols-3">
          <Card><CardHeader><CardDescription>Enrollment status</CardDescription></CardHeader><CardContent><Badge variant={statusVariant}>{status}</Badge></CardContent></Card>
          <Card><CardHeader><CardDescription>KYC status</CardDescription></CardHeader><CardContent><Badge variant="outline">{beneficiary.kycStatus.replace("_", " ")}</Badge></CardContent></Card>
          <Card><CardHeader><CardDescription>Programme enrollments</CardDescription></CardHeader><CardContent className="text-2xl font-bold">{enrollments.length}</CardContent></Card>
        </div>

        <Card>
          <CardHeader><CardTitle>Verified profile fields</CardTitle><CardDescription>Only fields returned by the beneficiary service are shown.</CardDescription></CardHeader>
          <CardContent className="grid gap-4 md:grid-cols-2">
            <Field label="National ID" value={beneficiary.nationalId} />
            <Field label="Date of birth" value={new Date(beneficiary.dateOfBirth).toLocaleDateString()} />
            <Field label="Phone" value={beneficiary.phoneNumber} />
            <Field label="Email" value={beneficiary.email} />
            <Field label="Address" value={[beneficiary.address, beneficiary.city, beneficiary.state, beneficiary.postalCode].filter(Boolean).join(", ")} />
            <Field label="Enrolled" value={new Date(beneficiary.enrolledAt).toLocaleDateString()} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader><CardTitle>Programme enrollments</CardTitle></CardHeader>
          <CardContent>
            <Table><TableHeader><TableRow><TableHead>Programme ID</TableHead><TableHead>Status</TableHead><TableHead>Monthly allocation</TableHead><TableHead>Enrollment date</TableHead></TableRow></TableHeader>
              <TableBody>{enrollments.length ? enrollments.map((row) => <TableRow key={row.id}><TableCell>{row.programId}</TableCell><TableCell>{row.status}</TableCell><TableCell>{row.monthlyAllocation}</TableCell><TableCell>{new Date(row.enrollmentDate).toLocaleDateString()}</TableCell></TableRow>) : <TableRow><TableCell colSpan={4} className="text-center text-muted-foreground">No programme enrollments.</TableCell></TableRow>}</TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}

function Field({ label, value }: { label: string; value: string | number | null | undefined }) {
  return <div><p className="text-sm text-muted-foreground">{label}</p><p className="font-medium">{value || "Not recorded"}</p></div>;
}
