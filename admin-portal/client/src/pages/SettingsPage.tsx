import DashboardLayout from "@/components/DashboardLayout";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { trpc } from "@/lib/trpc";
import { Settings, Flag, Calendar, Shield, Users, Loader2, Bell } from "lucide-react";
import { useState, useEffect } from "react";
import { toast } from "sonner";

export default function SettingsPage() {
  const { data: preferences, isLoading, refetch } = trpc.notificationPreferences.get.useQuery();
  const [localPrefs, setLocalPrefs] = useState({
    notifyOnRoleChange: true,
    notifyOnBatchOperations: true,
    notifyOnProgramChanges: true,
    notifyOnMccRuleChanges: false,
    emailAddress: "",
  });

  useEffect(() => {
    if (preferences) {
      setLocalPrefs({
        notifyOnRoleChange: preferences.notifyOnRoleChange,
        notifyOnBatchOperations: preferences.notifyOnBatchOperations,
        notifyOnProgramChanges: preferences.notifyOnProgramChanges,
        notifyOnMccRuleChanges: preferences.notifyOnMccRuleChanges,
        emailAddress: preferences.emailAddress || "",
      });
    }
  }, [preferences]);

  const updatePreferences = trpc.notificationPreferences.update.useMutation({
    onSuccess: () => {
      toast.success("Notification preferences saved");
      refetch();
    },
    onError: (error) => {
      toast.error(`Failed to save preferences: ${error.message}`);
    },
  });

  const handleSave = () => {
    updatePreferences.mutate({
      ...localPrefs,
      emailAddress: localPrefs.emailAddress || undefined,
    });
  };

  return (
    <DashboardLayout
      items={[
        { label: "Dashboard", href: "/", icon: Shield },
        { label: "Programs & MCC Rules", href: "/programs", icon: Settings },
        { label: "Feature Flags", href: "/feature-flags", icon: Flag },
        { label: "Disbursements", href: "/disbursements", icon: Calendar },
        { label: "Users", href: "/users", icon: Users },
      ]}
    >
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Settings</h1>
          <p className="text-muted-foreground mt-2">
            Configure your notification preferences and account settings
          </p>
        </div>

        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <Bell className="h-5 w-5" />
              <CardTitle>Email Notifications</CardTitle>
            </div>
            <CardDescription>
              Choose which events trigger email notifications
            </CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : (
              <div className="space-y-6">
                <div className="space-y-2">
                  <Label htmlFor="email-address">Notification Email Address</Label>
                  <Input
                    id="email-address"
                    type="email"
                    placeholder="your.email@example.com"
                    value={localPrefs.emailAddress}
                    onChange={(e) =>
                      setLocalPrefs({ ...localPrefs, emailAddress: e.target.value })
                    }
                  />
                  <p className="text-xs text-muted-foreground">
                    Leave empty to use your account email
                  </p>
                </div>

                <div className="space-y-4 pt-4 border-t">
                  <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label htmlFor="role-change">Role Changes</Label>
                      <p className="text-sm text-muted-foreground">
                        Notify when user roles are modified
                      </p>
                    </div>
                    <Switch
                      id="role-change"
                      checked={localPrefs.notifyOnRoleChange}
                      onCheckedChange={(checked) =>
                        setLocalPrefs({ ...localPrefs, notifyOnRoleChange: checked })
                      }
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label htmlFor="batch-operations">Batch Operations</Label>
                      <p className="text-sm text-muted-foreground">
                        Notify when bulk program or flag updates occur
                      </p>
                    </div>
                    <Switch
                      id="batch-operations"
                      checked={localPrefs.notifyOnBatchOperations}
                      onCheckedChange={(checked) =>
                        setLocalPrefs({ ...localPrefs, notifyOnBatchOperations: checked })
                      }
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label htmlFor="program-changes">Program Changes</Label>
                      <p className="text-sm text-muted-foreground">
                        Notify when benefit programs are created or modified
                      </p>
                    </div>
                    <Switch
                      id="program-changes"
                      checked={localPrefs.notifyOnProgramChanges}
                      onCheckedChange={(checked) =>
                        setLocalPrefs({ ...localPrefs, notifyOnProgramChanges: checked })
                      }
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label htmlFor="mcc-rule-changes">MCC Rule Changes</Label>
                      <p className="text-sm text-muted-foreground">
                        Notify when MCC rules are added or removed
                      </p>
                    </div>
                    <Switch
                      id="mcc-rule-changes"
                      checked={localPrefs.notifyOnMccRuleChanges}
                      onCheckedChange={(checked) =>
                        setLocalPrefs({ ...localPrefs, notifyOnMccRuleChanges: checked })
                      }
                    />
                  </div>
                </div>

                <div className="flex justify-end pt-4">
                  <Button onClick={handleSave} disabled={updatePreferences.isPending}>
                    {updatePreferences.isPending && (
                      <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                    )}
                    Save Preferences
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
