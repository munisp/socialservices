import { useState } from "react";
import { NationalIDVerification, VerificationHistory } from "@/components/identity/NationalIDVerification";
import DashboardLayout from "@/components/DashboardLayout";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export default function NationalIDPage() {
  const [beneficiaryId, setBeneficiaryId] = useState("");
  const [selectedBeneficiary, setSelectedBeneficiary] = useState<string | null>(null);

  const handleVerify = () => {
    if (beneficiaryId) {
      setSelectedBeneficiary(beneficiaryId);
    }
  };

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">National ID Verification</h1>
          <p className="text-muted-foreground">
            Verify beneficiary identity using national ID federation with 11+ identity providers.
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Select Beneficiary</CardTitle>
            <CardDescription>
              Enter a beneficiary ID to verify their identity or view verification history.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex gap-4">
              <Input
                placeholder="Enter beneficiary ID..."
                value={beneficiaryId}
                onChange={(e) => setBeneficiaryId(e.target.value)}
                className="max-w-sm"
              />
              <Button onClick={handleVerify}>Select</Button>
            </div>
          </CardContent>
        </Card>

        {selectedBeneficiary && (
          <Tabs defaultValue="verify" className="space-y-4">
            <TabsList>
              <TabsTrigger value="verify">New Verification</TabsTrigger>
              <TabsTrigger value="history">Verification History</TabsTrigger>
            </TabsList>
            <TabsContent value="verify">
              <NationalIDVerification
                beneficiaryId={selectedBeneficiary}
                onVerificationComplete={(result) => {
                  console.log("Verification complete:", result);
                }}
              />
            </TabsContent>
            <TabsContent value="history">
              <VerificationHistory beneficiaryId={selectedBeneficiary} />
            </TabsContent>
          </Tabs>
        )}
      </div>
    </DashboardLayout>
  );
}
