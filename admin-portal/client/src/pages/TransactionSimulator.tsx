import { useState } from "react";
import { useAuth } from "@/_core/hooks/useAuth";
import { trpc } from "@/lib/trpc";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { toast } from "sonner";
import { Loader2, Zap, AlertTriangle, CheckCircle, XCircle } from "lucide-react";

export default function TransactionSimulator() {
  const { user, loading: authLoading } = useAuth();
  const [beneficiaryId, setBeneficiaryId] = useState("");
  const [count, setCount] = useState("10");
  const [pattern, setPattern] = useState<"compliant" | "mcc_violation" | "fraud_pattern" | "mixed">("mixed");
  const [minAmount, setMinAmount] = useState("500");
  const [maxAmount, setMaxAmount] = useState("5000");

  const { data: beneficiaries } = trpc.beneficiaries.list.useQuery();
  const generateMutation = trpc.transactionSimulator.generateTransactions.useMutation({
    onSuccess: (data) => {
      toast.success(`Successfully generated ${data.count} transactions!`);
      setBeneficiaryId("");
      setCount("10");
    },
    onError: (error) => {
      toast.error(`Failed to generate transactions: ${error.message}`);
    },
  });

  const handleGenerate = () => {
    if (!beneficiaryId) {
      toast.error("Please select a beneficiary");
      return;
    }

    const countNum = parseInt(count);
    if (isNaN(countNum) || countNum < 1 || countNum > 100) {
      toast.error("Count must be between 1 and 100");
      return;
    }

    const minNum = parseInt(minAmount);
    const maxNum = parseInt(maxAmount);
    if (isNaN(minNum) || isNaN(maxNum) || minNum >= maxNum) {
      toast.error("Invalid amount range");
      return;
    }

    generateMutation.mutate({
      beneficiaryId: parseInt(beneficiaryId),
      count: countNum,
      pattern,
      amountRange: { min: minNum, max: maxNum },
    });
  };

  if (authLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <Loader2 className="h-8 w-8 animate-spin" />
      </div>
    );
  }

  if (!user || user.role !== "admin") {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <Card>
          <CardHeader>
            <CardTitle>Access Denied</CardTitle>
            <CardDescription>You must be an admin to access this page.</CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="container mx-auto py-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold mb-2">Transaction Simulator</h1>
        <p className="text-muted-foreground">
          Generate test transactions for beneficiaries to demonstrate monitoring and compliance features
        </p>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Zap className="h-5 w-5" />
              Generate Transactions
            </CardTitle>
            <CardDescription>
              Create simulated transactions with different patterns for testing
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="beneficiary">Beneficiary</Label>
              <Select value={beneficiaryId} onValueChange={setBeneficiaryId}>
                <SelectTrigger id="beneficiary">
                  <SelectValue placeholder="Select beneficiary" />
                </SelectTrigger>
                <SelectContent>
                  {beneficiaries?.map((b) => (
                    <SelectItem key={b.id} value={b.id.toString()}>
                      {b.firstName} {b.lastName} ({b.nationalId})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="count">Number of Transactions</Label>
              <Input
                id="count"
                type="number"
                min="1"
                max="100"
                value={count}
                onChange={(e) => setCount(e.target.value)}
                placeholder="10"
              />
              <p className="text-sm text-muted-foreground">Between 1 and 100</p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="pattern">Transaction Pattern</Label>
              <Select value={pattern} onValueChange={(v: any) => setPattern(v)}>
                <SelectTrigger id="pattern">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="compliant">
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-green-500" />
                      Compliant Only
                    </div>
                  </SelectItem>
                  <SelectItem value="mcc_violation">
                    <div className="flex items-center gap-2">
                      <AlertTriangle className="h-4 w-4 text-yellow-500" />
                      MCC Violations
                    </div>
                  </SelectItem>
                  <SelectItem value="fraud_pattern">
                    <div className="flex items-center gap-2">
                      <XCircle className="h-4 w-4 text-red-500" />
                      Fraud Patterns
                    </div>
                  </SelectItem>
                  <SelectItem value="mixed">
                    <div className="flex items-center gap-2">
                      <Zap className="h-4 w-4" />
                      Mixed (70% compliant, 20% violations, 10% fraud)
                    </div>
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="minAmount">Min Amount (NGN)</Label>
                <Input
                  id="minAmount"
                  type="number"
                  value={minAmount}
                  onChange={(e) => setMinAmount(e.target.value)}
                  placeholder="500"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="maxAmount">Max Amount (NGN)</Label>
                <Input
                  id="maxAmount"
                  type="number"
                  value={maxAmount}
                  onChange={(e) => setMaxAmount(e.target.value)}
                  placeholder="5000"
                />
              </div>
            </div>

            <Button
              onClick={handleGenerate}
              disabled={generateMutation.isPending}
              className="w-full"
            >
              {generateMutation.isPending ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Generating...
                </>
              ) : (
                <>
                  <Zap className="mr-2 h-4 w-4" />
                  Generate Transactions
                </>
              )}
            </Button>
          </CardContent>
        </Card>

        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Pattern Descriptions</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <div className="flex items-center gap-2 mb-1">
                  <CheckCircle className="h-4 w-4 text-green-500" />
                  <strong>Compliant Only</strong>
                </div>
                <p className="text-sm text-muted-foreground">
                  All transactions use allowed MCC codes (grocery, food, pharmacy). Perfect for testing normal operations.
                </p>
              </div>

              <div>
                <div className="flex items-center gap-2 mb-1">
                  <AlertTriangle className="h-4 w-4 text-yellow-500" />
                  <strong>MCC Violations</strong>
                </div>
                <p className="text-sm text-muted-foreground">
                  Transactions use restricted MCC codes (alcohol, gambling, tobacco). Tests compliance monitoring.
                </p>
              </div>

              <div>
                <div className="flex items-center gap-2 mb-1">
                  <XCircle className="h-4 w-4 text-red-500" />
                  <strong>Fraud Patterns</strong>
                </div>
                <p className="text-sm text-muted-foreground">
                  Suspicious transactions that trigger fraud alerts (unusual merchants, high-risk categories).
                </p>
              </div>

              <div>
                <div className="flex items-center gap-2 mb-1">
                  <Zap className="h-4 w-4" />
                  <strong>Mixed</strong>
                </div>
                <p className="text-sm text-muted-foreground">
                  Realistic mix: 70% compliant, 20% violations, 10% fraud. Best for comprehensive testing.
                </p>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Usage Tips</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm text-muted-foreground">
              <p>• Generated transactions appear in the Transaction Monitoring dashboard</p>
              <p>• Fraud patterns automatically create fraud alerts</p>
              <p>• Transaction dates are randomized within the last 30 days</p>
              <p>• Use this tool to populate test data before demos</p>
              <p>• All simulated transactions are marked with "SIM" prefix</p>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
