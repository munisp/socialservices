import { useEffect, useState } from "react";
import { trpc } from "@/lib/trpc";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  Server,
  Database,
  Activity,
  AlertCircle,
  CheckCircle2,
  XCircle,
  RefreshCw,
  Bell,
  BellOff,
} from "lucide-react";

type HealthStatus = "healthy" | "degraded" | "unhealthy";

interface MiddlewareHealth {
  component: string;
  status: HealthStatus;
  responseTime: number;
  errorMessage?: string;
  checkedAt: Date;
}

const componentIcons: Record<string, any> = {
  redis: Database,
  apisix: Server,
  kafka: Activity,
  fluvio: Activity,
  keycloak: Server,
  permify: Server,
  dapr: Server,
  temporal: Activity,
};

const componentNames: Record<string, string> = {
  redis: "Redis Cache",
  apisix: "APISIX Gateway",
  kafka: "Kafka Streaming",
  fluvio: "Fluvio Streaming",
  keycloak: "Keycloak SSO",
  permify: "Permify Authorization",
  dapr: "Dapr Runtime",
  temporal: "Temporal Workflows",
};

export default function MiddlewareDashboard() {
  const [autoRefresh, setAutoRefresh] = useState(true);

  const { data: healthStatus, refetch: refetchHealth } = trpc.middleware.getHealthStatus.useQuery(
    undefined,
    {
      refetchInterval: autoRefresh ? 30000 : false, // Refresh every 30 seconds
    }
  );

  const { data: activeAlerts, refetch: refetchAlerts } = trpc.middleware.getActiveAlerts.useQuery(
    undefined,
    {
      refetchInterval: autoRefresh ? 30000 : false,
    }
  );

  const checkHealthMutation = trpc.middleware.checkHealth.useMutation({
    onSuccess: () => {
      refetchHealth();
    },
  });

  const acknowledgeAlertMutation = trpc.middleware.acknowledgeAlert.useMutation({
    onSuccess: () => {
      refetchAlerts();
    },
  });

  const resolveAlertMutation = trpc.middleware.resolveAlert.useMutation({
    onSuccess: () => {
      refetchAlerts();
    },
  });

  const getStatusColor = (status: HealthStatus) => {
    switch (status) {
      case "healthy":
        return "text-green-600";
      case "degraded":
        return "text-yellow-600";
      case "unhealthy":
        return "text-red-600";
      default:
        return "text-gray-600";
    }
  };

  const getStatusIcon = (status: HealthStatus) => {
    switch (status) {
      case "healthy":
        return <CheckCircle2 className="h-5 w-5 text-green-600" />;
      case "degraded":
        return <AlertCircle className="h-5 w-5 text-yellow-600" />;
      case "unhealthy":
        return <XCircle className="h-5 w-5 text-red-600" />;
      default:
        return <XCircle className="h-5 w-5 text-gray-600" />;
    }
  };

  const getStatusBadge = (status: HealthStatus) => {
    const variant =
      status === "healthy" ? "default" : status === "degraded" ? "secondary" : "destructive";
    return (
      <Badge variant={variant} className="capitalize">
        {status}
      </Badge>
    );
  };

  const healthArray = healthStatus ? Object.values(healthStatus) : [];
  const healthyCount = healthArray.filter((h) => h.status === "healthy").length;
  const totalCount = healthArray.length;

  return (
    <div className="min-h-screen bg-gray-50 p-6">
      <div className="max-w-7xl mx-auto space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold text-gray-900">Middleware Dashboard</h1>
            <p className="text-gray-600 mt-1">
              Unified monitoring for all 8 middleware components
            </p>
          </div>
          <div className="flex items-center gap-3">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setAutoRefresh(!autoRefresh)}
            >
              {autoRefresh ? (
                <>
                  <Bell className="h-4 w-4 mr-2" />
                  Auto-refresh On
                </>
              ) : (
                <>
                  <BellOff className="h-4 w-4 mr-2" />
                  Auto-refresh Off
                </>
              )}
            </Button>
            <Button
              onClick={() => checkHealthMutation.mutate()}
              disabled={checkHealthMutation.isPending}
            >
              <RefreshCw
                className={`h-4 w-4 mr-2 ${checkHealthMutation.isPending ? "animate-spin" : ""}`}
              />
              Check Health
            </Button>
          </div>
        </div>

        {/* Overall Status */}
        <Card>
          <CardHeader>
            <CardTitle>System Overview</CardTitle>
            <CardDescription>
              {healthyCount} of {totalCount} components healthy
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-4">
              <div className="flex-1">
                <div className="h-4 bg-gray-200 rounded-full overflow-hidden">
                  <div
                    className="h-full bg-green-600 transition-all"
                    style={{ width: `${(healthyCount / totalCount) * 100}%` }}
                  />
                </div>
              </div>
              <div className="text-2xl font-bold">
                {Math.round((healthyCount / totalCount) * 100)}%
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Active Alerts */}
        {activeAlerts && activeAlerts.length > 0 && (
          <Card className="border-red-200 bg-red-50">
            <CardHeader>
              <CardTitle className="text-red-900 flex items-center gap-2">
                <AlertCircle className="h-5 w-5" />
                Active Alerts ({activeAlerts.length})
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {activeAlerts.map((alert) => (
                <Alert key={alert.id} variant="destructive">
                  <AlertDescription className="flex items-center justify-between">
                    <div>
                      <div className="font-semibold">{alert.message}</div>
                      <div className="text-sm mt-1">
                        {componentNames[alert.component]} • {alert.alertType} •{" "}
                        {new Date(alert.triggeredAt).toLocaleString()}
                      </div>
                    </div>
                    <div className="flex gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() =>
                          acknowledgeAlertMutation.mutate({ alertId: alert.id })
                        }
                      >
                        Acknowledge
                      </Button>
                      <Button
                        size="sm"
                        onClick={() => resolveAlertMutation.mutate({ alertId: alert.id })}
                      >
                        Resolve
                      </Button>
                    </div>
                  </AlertDescription>
                </Alert>
              ))}
            </CardContent>
          </Card>
        )}

        {/* Middleware Components Grid */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {healthArray.map((health) => {
            const Icon = componentIcons[health.component] || Server;
            return (
              <Card key={health.component} className="hover:shadow-lg transition-shadow">
                <CardHeader className="pb-3">
                  <div className="flex items-center justify-between">
                    <Icon className="h-8 w-8 text-blue-600" />
                    {getStatusIcon(health.status)}
                  </div>
                  <CardTitle className="text-lg mt-2">
                    {componentNames[health.component]}
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-gray-600">Status</span>
                    {getStatusBadge(health.status)}
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-gray-600">Response Time</span>
                    <span className="text-sm font-medium">{health.responseTime}ms</span>
                  </div>
                  {health.errorMessage && (
                    <div className="text-xs text-red-600 bg-red-50 p-2 rounded">
                      {health.errorMessage}
                    </div>
                  )}
                  <div className="text-xs text-gray-500">
                    Last checked: {new Date(health.checkedAt).toLocaleTimeString()}
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>

        {/* Component Details */}
        <Card>
          <CardHeader>
            <CardTitle>Component Details</CardTitle>
            <CardDescription>Detailed information about each middleware component</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {healthArray.map((health) => (
                <div
                  key={health.component}
                  className="flex items-center justify-between p-4 border rounded-lg"
                >
                  <div className="flex items-center gap-4">
                    {getStatusIcon(health.status)}
                    <div>
                      <div className="font-medium">{componentNames[health.component]}</div>
                      <div className="text-sm text-gray-600">{health.component}</div>
                    </div>
                  </div>
                  <div className="flex items-center gap-6">
                    <div className="text-right">
                      <div className="text-sm text-gray-600">Response Time</div>
                      <div className="font-medium">{health.responseTime}ms</div>
                    </div>
                    <div className="text-right">
                      <div className="text-sm text-gray-600">Status</div>
                      <div className={`font-medium capitalize ${getStatusColor(health.status)}`}>
                        {health.status}
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
