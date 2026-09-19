import React, { useState, useEffect, useCallback } from 'react';
import {
  Activity,
  Play,
  Pause,
  Square,
  RefreshCw,
  Clock,
  Zap,
  AlertTriangle,
  CheckCircle,
  XCircle,
  TrendingUp,
  TrendingDown,
  BarChart3,
  LineChart,
  Users,
  Server,
  Database,
  Cpu,
  HardDrive,
  Wifi,
  Target,
  Settings,
  Download,
  FileText,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Slider } from '@/components/ui/slider';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from '@/components/ui/alert';
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

// Types
type ScaleLevel = 'small' | 'medium' | 'large' | 'xlarge' | 'country';
type TestType = 'smoke' | 'load' | 'stress' | 'spike' | 'soak' | 'breakpoint' | 'scalability';
type TestScenario = 'beneficiary_enrollment' | 'beneficiary_search' | 'disbursement' | 'workflow_execution' | 'reporting' | 'mixed_workload' | 'offline_sync' | 'cross_sector_query';
type TestStatus = 'idle' | 'configuring' | 'running' | 'completed' | 'failed' | 'cancelled';

interface LoadTestConfig {
  id: string;
  name: string;
  description: string;
  testType: TestType;
  scenarios: TestScenario[];
  scaleLevel: ScaleLevel;
  duration: number; // seconds
  rampUpTime: number;
  rampDownTime: number;
  targetRps: number;
  maxConcurrency: number;
  thinkTime: number;
  dataSetSize: number;
  slos: SLOConfig;
}

interface SLOConfig {
  p50LatencyMs: number;
  p95LatencyMs: number;
  p99LatencyMs: number;
  maxLatencyMs: number;
  errorRatePercent: number;
  availabilityPercent: number;
  throughputRps: number;
}

interface LatencyStats {
  min: number;
  max: number;
  mean: number;
  median: number;
  p50: number;
  p75: number;
  p90: number;
  p95: number;
  p99: number;
  stdDev: number;
}

interface ScenarioResult {
  scenario: TestScenario;
  totalRequests: number;
  successfulRequests: number;
  failedRequests: number;
  errorRate: number;
  throughputRps: number;
  latencies?: LatencyStats;
}

interface SLOResults {
  p50Met: boolean;
  p95Met: boolean;
  p99Met: boolean;
  maxLatencyMet: boolean;
  errorRateMet: boolean;
  availabilityMet: boolean;
  throughputMet: boolean;
  overallCompliant: boolean;
  complianceScore: number;
}

interface ResourceMetrics {
  cpuUsagePercent: number;
  memoryUsagePercent: number;
  diskIops: number;
  networkBytesIn: number;
  networkBytesOut: number;
  dbConnectionsUsed: number;
  dbQueryLatencyMs: number;
}

interface TestError {
  timestamp: string;
  scenario: TestScenario;
  endpoint: string;
  statusCode: number;
  errorType: string;
  message: string;
}

interface LoadTestResult {
  id: string;
  configId: string;
  status: TestStatus;
  startTime: string;
  endTime?: string;
  duration: number;
  totalRequests: number;
  successfulRequests: number;
  failedRequests: number;
  errorRate: number;
  throughputRps: number;
  latencies?: LatencyStats;
  scenarioResults: Record<TestScenario, ScenarioResult>;
  sloResults?: SLOResults;
  resourceMetrics?: ResourceMetrics;
  errors: TestError[];
}

interface RealTimeMetrics {
  timestamp: number;
  requestsPerSecond: number;
  activeUsers: number;
  avgLatencyMs: number;
  errorRate: number;
  cpuUsage: number;
  memoryUsage: number;
}

// Scale presets
const SCALE_PRESETS: Record<ScaleLevel, { name: string; description: string; dataSize: string; rps: number; concurrency: number }> = {
  small: { name: 'Small', description: '10K beneficiaries', dataSize: '10,000', rps: 100, concurrency: 10 },
  medium: { name: 'Medium', description: '100K beneficiaries', dataSize: '100,000', rps: 500, concurrency: 50 },
  large: { name: 'Large', description: '1M beneficiaries', dataSize: '1,000,000', rps: 2000, concurrency: 200 },
  xlarge: { name: 'XLarge', description: '10M beneficiaries', dataSize: '10,000,000', rps: 10000, concurrency: 1000 },
  country: { name: 'Country', description: '100M+ beneficiaries', dataSize: '100,000,000', rps: 50000, concurrency: 5000 },
};

const TEST_TYPES: Record<TestType, { name: string; description: string }> = {
  smoke: { name: 'Smoke Test', description: 'Quick validation with minimal load' },
  load: { name: 'Load Test', description: 'Normal expected load' },
  stress: { name: 'Stress Test', description: 'Beyond normal capacity' },
  spike: { name: 'Spike Test', description: 'Sudden traffic spikes' },
  soak: { name: 'Soak Test', description: 'Extended duration test' },
  breakpoint: { name: 'Breakpoint Test', description: 'Find system limits' },
  scalability: { name: 'Scalability Test', description: 'Test horizontal scaling' },
};

const SCENARIOS: Record<TestScenario, { name: string; description: string }> = {
  beneficiary_enrollment: { name: 'Beneficiary Enrollment', description: 'New beneficiary registration' },
  beneficiary_search: { name: 'Beneficiary Search', description: 'Search and filter beneficiaries' },
  disbursement: { name: 'Disbursement', description: 'Payment processing' },
  workflow_execution: { name: 'Workflow Execution', description: 'Temporal workflow operations' },
  reporting: { name: 'Reporting', description: 'Report generation queries' },
  mixed_workload: { name: 'Mixed Workload', description: 'Combination of operations' },
  offline_sync: { name: 'Offline Sync', description: 'Mobile sync operations' },
  cross_sector_query: { name: 'Cross-Sector Query', description: 'Interoperability queries' },
};

// Main Dashboard Component
export function LoadTestDashboard() {
  const [activeTest, setActiveTest] = useState<LoadTestResult | null>(null);
  const [testHistory, setTestHistory] = useState<LoadTestResult[]>([]);
  const [realTimeMetrics, setRealTimeMetrics] = useState<RealTimeMetrics[]>([]);
  const [showConfigDialog, setShowConfigDialog] = useState(false);
  const [isLoading, setIsLoading] = useState(false);

  // Load test history on mount
  useEffect(() => {
    loadTestHistory();
  }, []);

  // Real-time metrics polling when test is running
  useEffect(() => {
    if (activeTest?.status === 'running') {
      const interval = setInterval(() => {
        fetchRealTimeMetrics();
      }, 1000);
      return () => clearInterval(interval);
    }
  }, [activeTest?.status]);

  const loadTestHistory = async () => {
    try {
      const response = await fetch('/api/loadtest/history');
      if (response.ok) {
        const data = await response.json();
        setTestHistory(data.results || []);
      }
    } catch (err) {
      console.error('Failed to load test history:', err);
    }
  };

  const fetchRealTimeMetrics = async () => {
    if (!activeTest) return;
    
    try {
      const response = await fetch(`/api/loadtest/${activeTest.id}/metrics`);
      if (response.ok) {
        const metrics = await response.json();
        setRealTimeMetrics(prev => [...prev.slice(-60), metrics]);
        
        // Update active test status
        const statusResponse = await fetch(`/api/loadtest/${activeTest.id}/status`);
        if (statusResponse.ok) {
          const status = await statusResponse.json();
          setActiveTest(prev => prev ? { ...prev, ...status } : null);
        }
      }
    } catch (err) {
      console.error('Failed to fetch metrics:', err);
    }
  };

  const startTest = async (config: LoadTestConfig) => {
    setIsLoading(true);
    try {
      const response = await fetch('/api/loadtest/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      });
      
      if (response.ok) {
        const result = await response.json();
        setActiveTest(result);
        setRealTimeMetrics([]);
        setShowConfigDialog(false);
      }
    } catch (err) {
      console.error('Failed to start test:', err);
    } finally {
      setIsLoading(false);
    }
  };

  const stopTest = async () => {
    if (!activeTest) return;
    
    try {
      await fetch(`/api/loadtest/${activeTest.id}/stop`, { method: 'POST' });
      setActiveTest(prev => prev ? { ...prev, status: 'cancelled' } : null);
    } catch (err) {
      console.error('Failed to stop test:', err);
    }
  };

  const downloadReport = async (testId: string) => {
    window.open(`/api/loadtest/${testId}/report`, '_blank');
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold flex items-center gap-2">
            <Activity className="h-6 w-6 text-primary" />
            Load Testing Dashboard
          </h2>
          <p className="text-muted-foreground">
            Performance testing for 100M+ beneficiary scale
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={loadTestHistory}>
            <RefreshCw className="h-4 w-4 mr-2" />
            Refresh
          </Button>
          <Button onClick={() => setShowConfigDialog(true)}>
            <Play className="h-4 w-4 mr-2" />
            New Test
          </Button>
        </div>
      </div>

      {/* Active Test */}
      {activeTest && (
        <ActiveTestPanel
          test={activeTest}
          metrics={realTimeMetrics}
          onStop={stopTest}
        />
      )}

      {/* Quick Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Total Tests</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{testHistory.length}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Passed</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">
              {testHistory.filter(t => t.sloResults?.overallCompliant).length}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Failed</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-red-600">
              {testHistory.filter(t => t.status === 'failed' || (t.sloResults && !t.sloResults.overallCompliant)).length}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Avg Compliance</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {testHistory.length > 0
                ? (testHistory.reduce((sum, t) => sum + (t.sloResults?.complianceScore || 0), 0) / testHistory.length).toFixed(0)
                : 0}%
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Test History */}
      <Card>
        <CardHeader>
          <CardTitle>Test History</CardTitle>
          <CardDescription>Recent load test results</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Test</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Scale</TableHead>
                <TableHead>Duration</TableHead>
                <TableHead>Throughput</TableHead>
                <TableHead>Error Rate</TableHead>
                <TableHead>SLO</TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {testHistory.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className="text-center text-muted-foreground">
                    No test results yet
                  </TableCell>
                </TableRow>
              ) : (
                testHistory.map((test) => (
                  <TableRow key={test.id}>
                    <TableCell>
                      <div className="font-medium">{test.id.slice(0, 8)}</div>
                      <div className="text-xs text-muted-foreground">
                        {new Date(test.startTime).toLocaleString()}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{test.configId}</Badge>
                    </TableCell>
                    <TableCell>-</TableCell>
                    <TableCell>{formatDuration(test.duration)}</TableCell>
                    <TableCell>{test.throughputRps.toFixed(0)} RPS</TableCell>
                    <TableCell>
                      <Badge variant={test.errorRate < 1 ? 'default' : 'destructive'}>
                        {test.errorRate.toFixed(2)}%
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {test.sloResults ? (
                        <Badge variant={test.sloResults.overallCompliant ? 'default' : 'destructive'}>
                          {test.sloResults.complianceScore.toFixed(0)}%
                        </Badge>
                      ) : (
                        <Badge variant="secondary">N/A</Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => downloadReport(test.id)}
                      >
                        <Download className="h-4 w-4" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Config Dialog */}
      <TestConfigDialog
        open={showConfigDialog}
        onOpenChange={setShowConfigDialog}
        onStart={startTest}
        isLoading={isLoading}
      />
    </div>
  );
}

// Active Test Panel
function ActiveTestPanel({
  test,
  metrics,
  onStop,
}: {
  test: LoadTestResult;
  metrics: RealTimeMetrics[];
  onStop: () => void;
}) {
  const latestMetrics = metrics[metrics.length - 1];
  const isRunning = test.status === 'running';

  return (
    <Card className="border-primary">
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {isRunning ? (
              <Activity className="h-5 w-5 text-primary animate-pulse" />
            ) : test.status === 'completed' ? (
              <CheckCircle className="h-5 w-5 text-green-500" />
            ) : (
              <XCircle className="h-5 w-5 text-red-500" />
            )}
            <CardTitle>
              {isRunning ? 'Test Running' : `Test ${test.status}`}
            </CardTitle>
          </div>
          {isRunning && (
            <Button variant="destructive" size="sm" onClick={onStop}>
              <Square className="h-4 w-4 mr-1" />
              Stop Test
            </Button>
          )}
        </div>
        <CardDescription>
          Started: {new Date(test.startTime).toLocaleString()}
          {test.endTime && ` | Ended: ${new Date(test.endTime).toLocaleString()}`}
        </CardDescription>
      </CardHeader>

      <CardContent>
        {/* Real-time metrics */}
        <div className="grid grid-cols-2 md:grid-cols-6 gap-4 mb-6">
          <MetricCard
            icon={Zap}
            label="Requests/sec"
            value={latestMetrics?.requestsPerSecond.toFixed(0) || test.throughputRps.toFixed(0)}
            trend={metrics.length > 1 ? (latestMetrics?.requestsPerSecond || 0) - (metrics[metrics.length - 2]?.requestsPerSecond || 0) : 0}
          />
          <MetricCard
            icon={Users}
            label="Active Users"
            value={latestMetrics?.activeUsers.toString() || '-'}
          />
          <MetricCard
            icon={Clock}
            label="Avg Latency"
            value={`${(latestMetrics?.avgLatencyMs || test.latencies?.mean || 0).toFixed(0)}ms`}
          />
          <MetricCard
            icon={AlertTriangle}
            label="Error Rate"
            value={`${(latestMetrics?.errorRate || test.errorRate).toFixed(2)}%`}
            variant={test.errorRate > 1 ? 'destructive' : 'default'}
          />
          <MetricCard
            icon={Cpu}
            label="CPU Usage"
            value={`${(latestMetrics?.cpuUsage || test.resourceMetrics?.cpuUsagePercent || 0).toFixed(0)}%`}
          />
          <MetricCard
            icon={HardDrive}
            label="Memory"
            value={`${(latestMetrics?.memoryUsage || test.resourceMetrics?.memoryUsagePercent || 0).toFixed(0)}%`}
          />
        </div>

        {/* Progress */}
        {isRunning && (
          <div className="space-y-2 mb-6">
            <div className="flex justify-between text-sm">
              <span>Progress</span>
              <span>{test.totalRequests.toLocaleString()} requests</span>
            </div>
            <Progress value={50} className="h-2" />
          </div>
        )}

        {/* SLO Results */}
        {test.sloResults && (
          <div className="space-y-4">
            <h4 className="font-medium flex items-center gap-2">
              <Target className="h-4 w-4" />
              SLO Compliance
            </h4>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <SLOIndicator label="P50 Latency" met={test.sloResults.p50Met} />
              <SLOIndicator label="P95 Latency" met={test.sloResults.p95Met} />
              <SLOIndicator label="P99 Latency" met={test.sloResults.p99Met} />
              <SLOIndicator label="Error Rate" met={test.sloResults.errorRateMet} />
              <SLOIndicator label="Availability" met={test.sloResults.availabilityMet} />
              <SLOIndicator label="Throughput" met={test.sloResults.throughputMet} />
              <div className="col-span-2 p-3 bg-muted rounded-lg text-center">
                <div className="text-2xl font-bold">
                  {test.sloResults.complianceScore.toFixed(0)}%
                </div>
                <div className="text-xs text-muted-foreground">Overall Compliance</div>
              </div>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// Metric Card
function MetricCard({
  icon: Icon,
  label,
  value,
  trend,
  variant = 'default',
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string;
  trend?: number;
  variant?: 'default' | 'destructive';
}) {
  return (
    <div className="p-3 bg-muted/50 rounded-lg">
      <div className="flex items-center gap-2 mb-1">
        <Icon className={`h-4 w-4 ${variant === 'destructive' ? 'text-red-500' : 'text-muted-foreground'}`} />
        <span className="text-xs text-muted-foreground">{label}</span>
      </div>
      <div className="flex items-center gap-2">
        <span className={`text-lg font-bold ${variant === 'destructive' ? 'text-red-500' : ''}`}>
          {value}
        </span>
        {trend !== undefined && trend !== 0 && (
          trend > 0 ? (
            <TrendingUp className="h-4 w-4 text-green-500" />
          ) : (
            <TrendingDown className="h-4 w-4 text-red-500" />
          )
        )}
      </div>
    </div>
  );
}

// SLO Indicator
function SLOIndicator({ label, met }: { label: string; met: boolean }) {
  return (
    <div className={`p-2 rounded-lg flex items-center gap-2 ${met ? 'bg-green-50' : 'bg-red-50'}`}>
      {met ? (
        <CheckCircle className="h-4 w-4 text-green-500" />
      ) : (
        <XCircle className="h-4 w-4 text-red-500" />
      )}
      <span className="text-sm">{label}</span>
    </div>
  );
}

// Test Config Dialog
function TestConfigDialog({
  open,
  onOpenChange,
  onStart,
  isLoading,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onStart: (config: LoadTestConfig) => void;
  isLoading: boolean;
}) {
  const [config, setConfig] = useState<Partial<LoadTestConfig>>({
    name: '',
    testType: 'load',
    scaleLevel: 'small',
    scenarios: ['mixed_workload'],
    duration: 300,
    rampUpTime: 60,
    rampDownTime: 30,
    targetRps: 100,
    maxConcurrency: 10,
    thinkTime: 1,
    slos: {
      p50LatencyMs: 100,
      p95LatencyMs: 500,
      p99LatencyMs: 1000,
      maxLatencyMs: 5000,
      errorRatePercent: 1,
      availabilityPercent: 99.9,
      throughputRps: 50,
    },
  });

  const handleScaleChange = (scale: ScaleLevel) => {
    const preset = SCALE_PRESETS[scale];
    setConfig(prev => ({
      ...prev,
      scaleLevel: scale,
      targetRps: preset.rps,
      maxConcurrency: preset.concurrency,
    }));
  };

  const handleStart = () => {
    const fullConfig: LoadTestConfig = {
      id: `LT-${Date.now()}`,
      name: config.name || `Load Test ${new Date().toISOString()}`,
      description: '',
      testType: config.testType || 'load',
      scenarios: config.scenarios || ['mixed_workload'],
      scaleLevel: config.scaleLevel || 'small',
      duration: config.duration || 300,
      rampUpTime: config.rampUpTime || 60,
      rampDownTime: config.rampDownTime || 30,
      targetRps: config.targetRps || 100,
      maxConcurrency: config.maxConcurrency || 10,
      thinkTime: config.thinkTime || 1,
      dataSetSize: parseInt(SCALE_PRESETS[config.scaleLevel || 'small'].dataSize.replace(/,/g, '')),
      slos: config.slos || {
        p50LatencyMs: 100,
        p95LatencyMs: 500,
        p99LatencyMs: 1000,
        maxLatencyMs: 5000,
        errorRatePercent: 1,
        availabilityPercent: 99.9,
        throughputRps: 50,
      },
    };
    onStart(fullConfig);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Settings className="h-5 w-5" />
            Configure Load Test
          </DialogTitle>
          <DialogDescription>
            Set up a new load test with custom parameters
          </DialogDescription>
        </DialogHeader>

        <Tabs defaultValue="basic" className="mt-4">
          <TabsList className="grid w-full grid-cols-3">
            <TabsTrigger value="basic">Basic</TabsTrigger>
            <TabsTrigger value="scenarios">Scenarios</TabsTrigger>
            <TabsTrigger value="slos">SLOs</TabsTrigger>
          </TabsList>

          <TabsContent value="basic" className="space-y-4 mt-4">
            <div className="space-y-2">
              <Label>Test Name</Label>
              <Input
                value={config.name}
                onChange={(e) => setConfig(prev => ({ ...prev, name: e.target.value }))}
                placeholder="My Load Test"
              />
            </div>

            <div className="space-y-2">
              <Label>Test Type</Label>
              <Select
                value={config.testType}
                onValueChange={(v) => setConfig(prev => ({ ...prev, testType: v as TestType }))}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Object.entries(TEST_TYPES).map(([key, { name, description }]) => (
                    <SelectItem key={key} value={key}>
                      <div>
                        <div className="font-medium">{name}</div>
                        <div className="text-xs text-muted-foreground">{description}</div>
                      </div>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>Scale Level</Label>
              <div className="grid grid-cols-5 gap-2">
                {Object.entries(SCALE_PRESETS).map(([key, preset]) => (
                  <Button
                    key={key}
                    variant={config.scaleLevel === key ? 'default' : 'outline'}
                    className="h-auto py-3 flex flex-col"
                    onClick={() => handleScaleChange(key as ScaleLevel)}
                  >
                    <span className="font-medium">{preset.name}</span>
                    <span className="text-xs opacity-70">{preset.dataSize}</span>
                  </Button>
                ))}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Duration (seconds)</Label>
                <Input
                  type="number"
                  value={config.duration}
                  onChange={(e) => setConfig(prev => ({ ...prev, duration: parseInt(e.target.value) }))}
                />
              </div>
              <div className="space-y-2">
                <Label>Ramp Up (seconds)</Label>
                <Input
                  type="number"
                  value={config.rampUpTime}
                  onChange={(e) => setConfig(prev => ({ ...prev, rampUpTime: parseInt(e.target.value) }))}
                />
              </div>
              <div className="space-y-2">
                <Label>Target RPS</Label>
                <Input
                  type="number"
                  value={config.targetRps}
                  onChange={(e) => setConfig(prev => ({ ...prev, targetRps: parseInt(e.target.value) }))}
                />
              </div>
              <div className="space-y-2">
                <Label>Max Concurrency</Label>
                <Input
                  type="number"
                  value={config.maxConcurrency}
                  onChange={(e) => setConfig(prev => ({ ...prev, maxConcurrency: parseInt(e.target.value) }))}
                />
              </div>
            </div>
          </TabsContent>

          <TabsContent value="scenarios" className="space-y-4 mt-4">
            <Label>Test Scenarios</Label>
            <div className="grid grid-cols-2 gap-3">
              {Object.entries(SCENARIOS).map(([key, { name, description }]) => (
                <div
                  key={key}
                  className={`p-3 border rounded-lg cursor-pointer transition-colors ${
                    config.scenarios?.includes(key as TestScenario)
                      ? 'border-primary bg-primary/5'
                      : 'hover:border-muted-foreground'
                  }`}
                  onClick={() => {
                    const scenarios = config.scenarios || [];
                    if (scenarios.includes(key as TestScenario)) {
                      setConfig(prev => ({
                        ...prev,
                        scenarios: scenarios.filter(s => s !== key),
                      }));
                    } else {
                      setConfig(prev => ({
                        ...prev,
                        scenarios: [...scenarios, key as TestScenario],
                      }));
                    }
                  }}
                >
                  <div className="font-medium">{name}</div>
                  <div className="text-xs text-muted-foreground">{description}</div>
                </div>
              ))}
            </div>
          </TabsContent>

          <TabsContent value="slos" className="space-y-4 mt-4">
            <Alert>
              <Target className="h-4 w-4" />
              <AlertTitle>Service Level Objectives</AlertTitle>
              <AlertDescription>
                Define the performance targets for this test
              </AlertDescription>
            </Alert>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>P50 Latency (ms)</Label>
                <Input
                  type="number"
                  value={config.slos?.p50LatencyMs}
                  onChange={(e) => setConfig(prev => ({
                    ...prev,
                    slos: { ...prev.slos!, p50LatencyMs: parseInt(e.target.value) },
                  }))}
                />
              </div>
              <div className="space-y-2">
                <Label>P95 Latency (ms)</Label>
                <Input
                  type="number"
                  value={config.slos?.p95LatencyMs}
                  onChange={(e) => setConfig(prev => ({
                    ...prev,
                    slos: { ...prev.slos!, p95LatencyMs: parseInt(e.target.value) },
                  }))}
                />
              </div>
              <div className="space-y-2">
                <Label>P99 Latency (ms)</Label>
                <Input
                  type="number"
                  value={config.slos?.p99LatencyMs}
                  onChange={(e) => setConfig(prev => ({
                    ...prev,
                    slos: { ...prev.slos!, p99LatencyMs: parseInt(e.target.value) },
                  }))}
                />
              </div>
              <div className="space-y-2">
                <Label>Max Error Rate (%)</Label>
                <Input
                  type="number"
                  step="0.1"
                  value={config.slos?.errorRatePercent}
                  onChange={(e) => setConfig(prev => ({
                    ...prev,
                    slos: { ...prev.slos!, errorRatePercent: parseFloat(e.target.value) },
                  }))}
                />
              </div>
              <div className="space-y-2">
                <Label>Min Availability (%)</Label>
                <Input
                  type="number"
                  step="0.1"
                  value={config.slos?.availabilityPercent}
                  onChange={(e) => setConfig(prev => ({
                    ...prev,
                    slos: { ...prev.slos!, availabilityPercent: parseFloat(e.target.value) },
                  }))}
                />
              </div>
              <div className="space-y-2">
                <Label>Min Throughput (RPS)</Label>
                <Input
                  type="number"
                  value={config.slos?.throughputRps}
                  onChange={(e) => setConfig(prev => ({
                    ...prev,
                    slos: { ...prev.slos!, throughputRps: parseInt(e.target.value) },
                  }))}
                />
              </div>
            </div>
          </TabsContent>
        </Tabs>

        <DialogFooter className="mt-6">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleStart} disabled={isLoading}>
            {isLoading ? (
              <>
                <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                Starting...
              </>
            ) : (
              <>
                <Play className="h-4 w-4 mr-2" />
                Start Test
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// Utility functions
function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`;
}

export default LoadTestDashboard;
