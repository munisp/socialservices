import { useState } from "react";
import { PMTSurveyForm, PMTScoreResult } from "@/components/pmt/PMTSurveyForm";
import DashboardLayout from "@/components/DashboardLayout";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export default function PMTPage() {
  const [householdId, setHouseholdId] = useState("");
  const [selectedHousehold, setSelectedHousehold] = useState<string | null>(null);
  const [pmtResult, setPmtResult] = useState<any>(null);

  const handleSelect = () => {
    if (householdId) {
      setSelectedHousehold(householdId);
      setPmtResult(null);
    }
  };

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Proxy Means Testing</h1>
          <p className="text-muted-foreground">
            Conduct household surveys and calculate PMT scores for eligibility determination.
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Select Household</CardTitle>
            <CardDescription>
              Enter a household ID to conduct a PMT survey or view existing scores.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex gap-4">
              <Input
                placeholder="Enter household ID..."
                value={householdId}
                onChange={(e) => setHouseholdId(e.target.value)}
                className="max-w-sm"
              />
              <Button onClick={handleSelect}>Select</Button>
            </div>
          </CardContent>
        </Card>

        {selectedHousehold && (
          <Tabs defaultValue="survey" className="space-y-4">
            <TabsList>
              <TabsTrigger value="survey">New Survey</TabsTrigger>
              <TabsTrigger value="result" disabled={!pmtResult}>
                Score Result
              </TabsTrigger>
            </TabsList>
            <TabsContent value="survey">
              <PMTSurveyForm
                householdId={selectedHousehold}
                onComplete={(result) => {
                  setPmtResult(result);
                }}
              />
            </TabsContent>
            <TabsContent value="result">
              {pmtResult && <PMTScoreResult score={pmtResult} />}
            </TabsContent>
          </Tabs>
        )}
      </div>
    </DashboardLayout>
  );
}
