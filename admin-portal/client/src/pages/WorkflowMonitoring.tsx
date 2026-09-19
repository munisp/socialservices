import { useState } from "react";
import { trpc } from "@/lib/trpc";
import { useEffect, useState as useReactState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { 
  Activity, 
  CheckCircle2, 
  XCircle, 
  Clock, 
  AlertTriangle,
  TrendingUp,
  BarChart3,
  RefreshCw
} from "lucide-react";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";

export default function WorkflowMonitoring() {
  const [selectedWorkflow, setSelectedWorkflow] = useReactState<string | null>(null);
  const [showTemporalUI, setShowTemporalUI] = useReactState(false);
  const [temporalUIUrl, setTemporalUIUrl] = useReactState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [typeFilter, setTypeFilter] = useState<string>("all");

  const { data: statistics, refetch: refetchStats } = trpc.workflow.getStatistics.useQuery();
  const { data: workflowTypes } = trpc.workflow.getWorkflowTypes.useQuery();
  const { data: workflows, refetch: refetchWorkflows } = trpc.workflow.list.useQuery({
    status: statusFilter !== "all" ? statusFilter as any : undefined,
    workflowType: typeFilter !== "all" ? typeFilter : undefined,
    limit: 100,
  });
  const { data: alerts } = trpc.workflow.getAlerts.useQuery({ resolved: false });
  const { data: temporalConfig } = trpc.temporalUI.getConfig.useQuery();
  const { data: workflowDetails } = trpc.workflow.getById.useQuery(
    { workflowId: selectedWorkflow! },
    { enabled: !!selectedWorkflow }
  );

  const resolveAlertMutation = trpc.workflow.resolveAlert.useMutation({
    onSuccess: () => {
      refetchStats();
    },
  });

  const getStatusIcon = (status: string) => {
    switch (status) {
      case "completed":
        return <CheckCircle2 className="h-4 w-4 text-green-500" />;
      case "failed":
        return <XCircle className="h-4 w-4 text-red-500" />;
      case "running":
        return <Activity className="h-4 w-4 text-blue-500 animate-pulse" />;
      case "timeout":
        return <Clock className="h-4 w-4 text-orange-500" />;
      default:
        return <AlertTriangle className="h-4 w-4 text-gray-500" />;
    }
  };

  const getStatusBadge = (status: string) => {
    const variants: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
      completed: "default",
      failed: "destructive",
      running: "secondary",
      timeout: "outline",
      cancelled: "outline",
    };
    return <Badge variant={variants[status] || "outline"}>{status}</Badge>;
  };

  const formatDuration = (ms: number | null) => {
    if (!ms) return "N/A";
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
    return `${(ms / 60000).toFixed(1)}m`;
  };

  const formatWorkflowType = (type: string) => {
    return type.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
  };

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Workflow Monitoring</h1>
          <p className="text-muted-foreground">
            Monitor Temporal workflow executions and performance metrics
          </p>
        </div>
        <Button onClick={() => { refetchStats(); refetchWorkflows(); }} variant="outline">
          <RefreshCw className="h-4 w-4 mr-2" />
          Refresh
        </Button>
      </div>

      {/* Statistics Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Workflows</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{statistics?.totalWorkflows || 0}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Completed</CardTitle>
            <CheckCircle2 className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">
              {statistics?.stats.find((s) => s.status === "completed")?.count || 0}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Failed</CardTitle>
            <XCircle className="h-4 w-4 text-red-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-red-600">
              {statistics?.stats.find((s) => s.status === "failed")?.count || 0}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Running</CardTitle>
            <Activity className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">
              {statistics?.stats.find((s) => s.status === "running")?.count || 0}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Alerts */}
      {alerts && alerts.length > 0 && (
        <Card className="border-orange-200 bg-orange-50">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-orange-600" />
              Active Alerts ({alerts.length})
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              {alerts.slice(0, 5).map((alert) => (
                <div key={alert.id} className="flex items-center justify-between p-3 bg-white rounded-lg border">
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <Badge variant={alert.severity === "critical" ? "destructive" : "outline"}>
                        {alert.severity}
                      </Badge>
                      <span className="font-medium">{alert.alertType.replace(/_/g, " ")}</span>
                    </div>
                    <p className="text-sm text-muted-foreground mt-1">{alert.message}</p>
                  </div>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => resolveAlertMutation.mutate({ alertId: alert.id, resolvedBy: "admin" })}
                  >
                    Resolve
                  </Button>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Main Content */}
      <Tabs defaultValue="executions" className="space-y-4">
        <TabsList>
          <TabsTrigger value="executions">Workflow Executions</TabsTrigger>
          <TabsTrigger value="types">Workflow Types</TabsTrigger>
          <TabsTrigger value="failures">Recent Failures</TabsTrigger>
        </TabsList>

        <TabsContent value="executions" className="space-y-4">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>Workflow Executions</CardTitle>
                  <CardDescription>View and filter workflow execution history</CardDescription>
                </div>
                <div className="flex gap-2">
                  <Select value={statusFilter} onValueChange={setStatusFilter}>
                    <SelectTrigger className="w-[150px]">
                      <SelectValue placeholder="Filter by status" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">All Statuses</SelectItem>
                      <SelectItem value="running">Running</SelectItem>
                      <SelectItem value="completed">Completed</SelectItem>
                      <SelectItem value="failed">Failed</SelectItem>
                      <SelectItem value="timeout">Timeout</SelectItem>
                    </SelectContent>
                  </Select>
                  <Select value={typeFilter} onValueChange={setTypeFilter}>
                    <SelectTrigger className="w-[200px]">
                      <SelectValue placeholder="Filter by type" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">All Types</SelectItem>
                      {workflowTypes?.map((type) => (
                        <SelectItem key={type.workflowType} value={type.workflowType}>
                          {formatWorkflowType(type.workflowType)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Workflow ID</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Start Time</TableHead>
                    <TableHead>Duration</TableHead>
                    <TableHead>Initiated By</TableHead>
                    <TableHead>Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {workflows?.workflows.map((workflow) => (
                    <TableRow key={workflow.id}>
                      <TableCell className="font-mono text-sm">{workflow.workflowId.slice(0, 16)}...</TableCell>
                      <TableCell>{formatWorkflowType(workflow.workflowType)}</TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          {getStatusIcon(workflow.status)}
                          {getStatusBadge(workflow.status)}
                        </div>
                      </TableCell>
                      <TableCell>{new Date(workflow.startTime).toLocaleString()}</TableCell>
                      <TableCell>{formatDuration(workflow.duration)}</TableCell>
                      <TableCell>{workflow.initiatedBy || "System"}</TableCell>
                      <TableCell>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => setSelectedWorkflow(workflow.workflowId)}
                        >
                          View Details
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="types" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Workflow Types Overview</CardTitle>
              <CardDescription>Performance metrics by workflow type</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Workflow Type</TableHead>
                    <TableHead>Total Executions</TableHead>
                    <TableHead>Success Rate</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {workflowTypes?.map((type) => (
                    <TableRow key={type.workflowType}>
                      <TableCell className="font-medium">{formatWorkflowType(type.workflowType)}</TableCell>
                      <TableCell>{type.count}</TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <div className="w-24 bg-gray-200 rounded-full h-2">
                            <div
                              className="bg-green-500 h-2 rounded-full"
                              style={{ width: `${type.successRate}%` }}
                            />
                          </div>
                          <span className="text-sm">{type.successRate?.toFixed(1)}%</span>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="failures" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Recent Failures</CardTitle>
              <CardDescription>Last 10 failed workflow executions</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Workflow ID</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Failed At</TableHead>
                    <TableHead>Error</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {statistics?.recentFailures.map((workflow) => (
                    <TableRow key={workflow.id}>
                      <TableCell className="font-mono text-sm">{workflow.workflowId.slice(0, 16)}...</TableCell>
                      <TableCell>{formatWorkflowType(workflow.workflowType)}</TableCell>
                      <TableCell>{new Date(workflow.startTime).toLocaleString()}</TableCell>
                      <TableCell className="max-w-md truncate text-red-600">{workflow.error || "Unknown error"}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Workflow Details Dialog */}
      <Dialog open={!!selectedWorkflow} onOpenChange={() => setSelectedWorkflow(null)}>
        <DialogContent className="max-w-4xl max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Workflow Details</DialogTitle>
            <DialogDescription>
              Detailed information about workflow execution and activities
            </DialogDescription>
          </DialogHeader>
          {workflowDetails && (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Workflow ID</p>
                  <p className="font-mono text-sm">{workflowDetails.workflow.workflowId}</p>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Run ID</p>
                  <p className="font-mono text-sm">{workflowDetails.workflow.runId}</p>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Type</p>
                  <p>{formatWorkflowType(workflowDetails.workflow.workflowType)}</p>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Status</p>
                  {getStatusBadge(workflowDetails.workflow.status)}
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Duration</p>
                  <p>{formatDuration(workflowDetails.workflow.duration)}</p>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Initiated By</p>
                  <p>{workflowDetails.workflow.initiatedBy || "System"}</p>
                </div>
              </div>

              <div>
                <h3 className="font-semibold mb-2">Activities ({workflowDetails.activities.length})</h3>
                <div className="space-y-2">
                  {workflowDetails.activities.map((activity, index) => (
                    <div key={activity.id} className="border rounded-lg p-3">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-medium">#{index + 1}</span>
                          {getStatusIcon(activity.status)}
                          <span className="font-medium">{activity.activityType}</span>
                          {getStatusBadge(activity.status)}
                        </div>
                        <span className="text-sm text-muted-foreground">
                          {formatDuration(activity.duration)}
                        </span>
                      </div>
                      {activity.error && (
                        <p className="text-sm text-red-600 mt-2">{activity.error}</p>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
