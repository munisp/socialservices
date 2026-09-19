import { OfflineSyncManager } from "@/components/offline/OfflineSyncStatus";
import DashboardLayout from "@/components/DashboardLayout";

export default function OfflineSyncPage() {
  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Offline Sync Management</h1>
          <p className="text-muted-foreground">
            Manage offline data synchronization, view sync history, and resolve conflicts.
          </p>
        </div>
        <OfflineSyncManager />
      </div>
    </DashboardLayout>
  );
}
