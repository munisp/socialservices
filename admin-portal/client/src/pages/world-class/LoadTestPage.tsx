import { LoadTestDashboard } from "@/components/loadtest/LoadTestDashboard";
import DashboardLayout from "@/components/DashboardLayout";

export default function LoadTestPage() {
  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Load Testing Dashboard</h1>
          <p className="text-muted-foreground">
            Configure and run load tests to verify platform performance at scale (up to 100M+ beneficiaries).
          </p>
        </div>
        <LoadTestDashboard />
      </div>
    </DashboardLayout>
  );
}
