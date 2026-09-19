import { useState } from "react";
import { CrossSectorProfileView, ConsentManagement } from "@/components/interop/CrossSectorProfile";
import DashboardLayout from "@/components/DashboardLayout";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export default function InteropPage() {
  const [beneficiaryId, setBeneficiaryId] = useState("");
  const [selectedBeneficiary, setSelectedBeneficiary] = useState<string | null>(null);

  const handleSelect = () => {
    if (beneficiaryId) {
      setSelectedBeneficiary(beneficiaryId);
    }
  };

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Cross-Sector Interoperability</h1>
          <p className="text-muted-foreground">
            View cross-sector data profiles and manage data sharing consents across health, education, tax, and labor sectors.
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Select Beneficiary</CardTitle>
            <CardDescription>
              Enter a beneficiary ID to view their cross-sector profile and manage consents.
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
              <Button onClick={handleSelect}>Select</Button>
            </div>
          </CardContent>
        </Card>

        {selectedBeneficiary && (
          <Tabs defaultValue="profile" className="space-y-4">
            <TabsList>
              <TabsTrigger value="profile">Cross-Sector Profile</TabsTrigger>
              <TabsTrigger value="consents">Consent Management</TabsTrigger>
            </TabsList>
            <TabsContent value="profile">
              <CrossSectorProfileView beneficiaryId={selectedBeneficiary} />
            </TabsContent>
            <TabsContent value="consents">
              <ConsentManagement beneficiaryId={selectedBeneficiary} />
            </TabsContent>
          </Tabs>
        )}
      </div>
    </DashboardLayout>
  );
}
