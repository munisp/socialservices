import { useState } from "react";
import { trpc } from "@/lib/trpc";
import DashboardLayout from "@/components/DashboardLayout";
import { Button } from "@/components/ui/button";
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  AlertTriangle,
  TrendingUp,
  DollarSign,
  Activity,
  Download,
} from "lucide-react";
import { toast } from "sonner";

export default function Transactions() {
  const [activeTab, setActiveTab] = useState("all");

  const { data: transactions, isLoading } = trpc.transactions.list.useQuery({ limit: 100 });
  const { data: nonCompliant } = trpc.transactions.getNonCompliant.useQuery({ limit: 50 });
  const { data: stats } = trpc.transactions.getStats.useQuery();
  const { data: mccUsage } = trpc.transactions.getMccUsage.useQuery({ limit: 10 });
  const { data: fraudAlerts } = trpc.fraudAlerts.getOpen.useQuery();
  const { data: fraudStats } = trpc.fraudAlerts.getStats.useQuery();

  const exportMutation = trpc.exports.transactions.useMutation({
    onSuccess: (data) => {
      const blob = new Blob([data.csv], { type: "text/csv" });
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = data.filename;
      a.click();
      toast.success("Transactions exported successfully");
    },
    onError: (error) => {
      toast.error(`Export failed: ${error.message}`);
    },
  });

  const getStatusBadge = (status: string) => {
    const variants: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
      pending: "outline",
      approved: "default",
      declined: "destructive",
      reversed: "secondary",
    };
    return <Badge variant={variants[status] || "default"}>{status}</Badge>;
  };

  const getComplianceBadge = (status: string) => {
    const variants: Record<string, "default" | "secondary" | "destructive"> = {
      compliant: "default",
      non_compliant: "destructive",
      under_review: "secondary",
    };
    return <Badge variant={variants[status] || "default"}>{status.replace("_", " ")}</Badge>;
  };

  const formatAmount = (amount: number, currency: string) => {
    return new Intl.NumberFormat("en-NG", {
      style: "currency",
      currency: currency || "NGN",
    }).format(amount / 100);
  };

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold">Transaction Monitoring</h1>
            <p className="text-muted-foreground">
              Real-time transaction monitoring, compliance checking, and fraud detection
            </p>
          </div>
          <Button
            onClick={() => exportMutation.mutate({ format: "csv" })}
            disabled={exportMutation.isPending}
          >
            <Download className="mr-2 h-4 w-4" />
            Export Transactions
          </Button>
        </div>

        {/* Statistics Cards */}
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium">Total Transactions</CardTitle>
              <Activity className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stats?.total || 0}</div>
              <p className="text-xs text-muted-foreground mt-1">
                {stats?.approved || 0} approved, {stats?.declined || 0} declined
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium">Total Volume</CardTitle>
              <DollarSign className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {formatAmount(stats?.totalAmount || 0, "NGN")}
              </div>
              <p className="text-xs text-muted-foreground mt-1">
                Avg: {formatAmount(stats?.avgAmount || 0, "NGN")}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium">Non-Compliant</CardTitle>
              <AlertTriangle className="h-4 w-4 text-destructive" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-destructive">
                {nonCompliant?.length || 0}
              </div>
              <p className="text-xs text-muted-foreground mt-1">Requires review</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium">Fraud Alerts</CardTitle>
              <AlertTriangle className="h-4 w-4 text-orange-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-orange-500">
                {fraudStats?.open || 0}
              </div>
              <p className="text-xs text-muted-foreground mt-1">
                {fraudStats?.investigating || 0} investigating
              </p>
            </CardContent>
          </Card>
        </div>

        {/* MCC Usage Stats */}
        <Card>
          <CardHeader>
            <CardTitle>Top MCC Categories</CardTitle>
            <CardDescription>Most frequently used merchant categories</CardDescription>
          </CardHeader>
          <CardContent>
            {mccUsage && mccUsage.length > 0 ? (
              <div className="space-y-4">
                {mccUsage.map((mcc: any, index: number) => (
                  <div key={index} className="flex items-center justify-between">
                    <div className="flex-1">
                      <div className="font-medium">
                        {mcc.mccCode} - {mcc.mccDescription}
                      </div>
                      <div className="text-sm text-muted-foreground">
                        {mcc.transactionCount} transactions • {formatAmount(mcc.totalAmount, "NGN")}
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="font-medium">{formatAmount(mcc.avgAmount, "NGN")}</div>
                      <div className="text-sm text-muted-foreground">avg</div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-8 text-muted-foreground">No data available</div>
            )}
          </CardContent>
        </Card>

        {/* Transactions Table */}
        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList>
            <TabsTrigger value="all">All Transactions</TabsTrigger>
            <TabsTrigger value="non-compliant">
              Non-Compliant ({nonCompliant?.length || 0})
            </TabsTrigger>
            <TabsTrigger value="fraud">
              Fraud Alerts ({fraudAlerts?.length || 0})
            </TabsTrigger>
          </TabsList>

          <TabsContent value="all">
            <Card>
              <CardHeader>
                <CardTitle>Recent Transactions</CardTitle>
                <CardDescription>Latest 100 transactions across all programs</CardDescription>
              </CardHeader>
              <CardContent>
                {isLoading ? (
                  <div className="text-center py-8 text-muted-foreground">Loading...</div>
                ) : !transactions || transactions.length === 0 ? (
                  <div className="text-center py-8 text-muted-foreground">
                    No transactions found
                  </div>
                ) : (
                  <div className="rounded-md border">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Transaction ID</TableHead>
                          <TableHead>Beneficiary</TableHead>
                          <TableHead>Merchant</TableHead>
                          <TableHead>MCC</TableHead>
                          <TableHead>Amount</TableHead>
                          <TableHead>Status</TableHead>
                          <TableHead>Compliance</TableHead>
                          <TableHead>Date</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {transactions.map((tx: any) => (
                          <TableRow key={tx.id}>
                            <TableCell className="font-mono text-sm">
                              {tx.transactionId.slice(0, 12)}...
                            </TableCell>
                            <TableCell>{tx.beneficiaryId}</TableCell>
                            <TableCell>
                              <div className="text-sm">
                                <div>{tx.merchantName || "Unknown"}</div>
                                <div className="text-muted-foreground">{tx.merchantId}</div>
                              </div>
                            </TableCell>
                            <TableCell>
                              <div className="text-sm">
                                <div className="font-medium">{tx.mccCode}</div>
                                <div className="text-muted-foreground">{tx.mccDescription}</div>
                              </div>
                            </TableCell>
                            <TableCell className="font-medium">
                              {formatAmount(tx.amount, tx.currency)}
                            </TableCell>
                            <TableCell>{getStatusBadge(tx.status)}</TableCell>
                            <TableCell>{getComplianceBadge(tx.complianceStatus)}</TableCell>
                            <TableCell>
                              {new Date(tx.transactionDate).toLocaleString()}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="non-compliant">
            <Card>
              <CardHeader>
                <CardTitle>Non-Compliant Transactions</CardTitle>
                <CardDescription>
                  Transactions that violated MCC spending rules
                </CardDescription>
              </CardHeader>
              <CardContent>
                {!nonCompliant || nonCompliant.length === 0 ? (
                  <div className="text-center py-8 text-muted-foreground">
                    No non-compliant transactions
                  </div>
                ) : (
                  <div className="rounded-md border">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Transaction ID</TableHead>
                          <TableHead>Beneficiary</TableHead>
                          <TableHead>Merchant</TableHead>
                          <TableHead>MCC</TableHead>
                          <TableHead>Amount</TableHead>
                          <TableHead>Decline Reason</TableHead>
                          <TableHead>Date</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {nonCompliant.map((tx: any) => (
                          <TableRow key={tx.id}>
                            <TableCell className="font-mono text-sm">
                              {tx.transactionId.slice(0, 12)}...
                            </TableCell>
                            <TableCell>{tx.beneficiaryId}</TableCell>
                            <TableCell>{tx.merchantName || "Unknown"}</TableCell>
                            <TableCell>
                              <div className="text-sm">
                                <div className="font-medium">{tx.mccCode}</div>
                                <div className="text-muted-foreground">{tx.mccDescription}</div>
                              </div>
                            </TableCell>
                            <TableCell className="font-medium">
                              {formatAmount(tx.amount, tx.currency)}
                            </TableCell>
                            <TableCell className="text-sm text-destructive">
                              {tx.declineReason || "MCC not allowed"}
                            </TableCell>
                            <TableCell>
                              {new Date(tx.transactionDate).toLocaleString()}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="fraud">
            <Card>
              <CardHeader>
                <CardTitle>Open Fraud Alerts</CardTitle>
                <CardDescription>
                  Suspicious transactions requiring investigation
                </CardDescription>
              </CardHeader>
              <CardContent>
                {!fraudAlerts || fraudAlerts.length === 0 ? (
                  <div className="text-center py-8 text-muted-foreground">
                    No open fraud alerts
                  </div>
                ) : (
                  <div className="rounded-md border">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Alert ID</TableHead>
                          <TableHead>Beneficiary</TableHead>
                          <TableHead>Alert Type</TableHead>
                          <TableHead>Severity</TableHead>
                          <TableHead>Description</TableHead>
                          <TableHead>Status</TableHead>
                          <TableHead>Created</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {fraudAlerts.map((alert: any) => (
                          <TableRow key={alert.id}>
                            <TableCell className="font-medium">{alert.id}</TableCell>
                            <TableCell>{alert.beneficiaryId}</TableCell>
                            <TableCell>{alert.alertType.replace("_", " ")}</TableCell>
                            <TableCell>
                              <Badge
                                variant={
                                  alert.severity === "critical" || alert.severity === "high"
                                    ? "destructive"
                                    : "secondary"
                                }
                              >
                                {alert.severity}
                              </Badge>
                            </TableCell>
                            <TableCell className="max-w-xs truncate">
                              {alert.description}
                            </TableCell>
                            <TableCell>
                              <Badge variant="outline">{alert.status}</Badge>
                            </TableCell>
                            <TableCell>
                              {new Date(alert.createdAt).toLocaleString()}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </DashboardLayout>
  );
}
