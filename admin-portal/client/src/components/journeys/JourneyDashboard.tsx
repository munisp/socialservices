/**
 * Journey Dashboard Component
 * Provides UI for viewing and starting all 30 user journeys
 */

import React, { useState, useEffect } from 'react';
import {
  Play,
  Clock,
  CheckCircle,
  XCircle,
  AlertTriangle,
  RefreshCw,
  Search,
  Filter,
  ChevronRight,
  Users,
  CreditCard,
  MessageSquare,
  UserCog,
  Shield,
  BarChart3,
  AlertOctagon,
  Workflow,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import {
  Card,
  CardContent,
  CardDescription,
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
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
} from '@/components/ui/dialog';
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs';
import { trpc } from '@/lib/trpc';

// Journey category icons
const categoryIcons: Record<string, React.ReactNode> = {
  enrollment: <Users className="h-5 w-5" />,
  payments: <CreditCard className="h-5 w-5" />,
  grievance: <MessageSquare className="h-5 w-5" />,
  lifecycle: <UserCog className="h-5 w-5" />,
  admin: <Shield className="h-5 w-5" />,
  reporting: <BarChart3 className="h-5 w-5" />,
  fraud: <AlertOctagon className="h-5 w-5" />,
};

// Journey category colors
const categoryColors: Record<string, string> = {
  enrollment: 'bg-blue-100 text-blue-800 border-blue-200',
  payments: 'bg-green-100 text-green-800 border-green-200',
  grievance: 'bg-yellow-100 text-yellow-800 border-yellow-200',
  lifecycle: 'bg-purple-100 text-purple-800 border-purple-200',
  admin: 'bg-red-100 text-red-800 border-red-200',
  reporting: 'bg-indigo-100 text-indigo-800 border-indigo-200',
  fraud: 'bg-orange-100 text-orange-800 border-orange-200',
};

interface JourneyContract {
  journeyKey: string;
  name: string;
  category: string;
  description: string;
  uiEntryPoints: string[];
  bffEndpoint: string;
  orchestratorPath: string;
  temporalWorkflow: string;
  requiredPermissions: string[];
  middlewareHooks: string[];
}

interface JourneyRun {
  journeyRunId: string;
  journeyKey: string;
  status: 'running' | 'completed' | 'failed' | 'pending';
  startedAt: string;
  completedAt?: string;
  progress?: number;
  currentStep?: string;
}

export function JourneyDashboard() {
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  const [selectedJourney, setSelectedJourney] = useState<JourneyContract | null>(null);
  const [recentRuns, setRecentRuns] = useState<JourneyRun[]>([]);
  const [isStartDialogOpen, setIsStartDialogOpen] = useState(false);

  // Fetch journey contracts
  const { data: contracts, isLoading, refetch } = trpc.worldClass.journeys.getContracts.useQuery();

  // Fetch categories
  const { data: categories } = trpc.worldClass.journeys.getCategories.useQuery();

  // Filter journeys based on search and category
  const filteredJourneys = contracts?.filter((journey: JourneyContract) => {
    const matchesSearch = journey.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      journey.journeyKey.toLowerCase().includes(searchQuery.toLowerCase());
    const matchesCategory = selectedCategory === 'all' || journey.category === selectedCategory;
    return matchesSearch && matchesCategory;
  }) || [];

  // Group journeys by category
  const journeysByCategory = filteredJourneys.reduce((acc: Record<string, JourneyContract[]>, journey: JourneyContract) => {
    if (!acc[journey.category]) {
      acc[journey.category] = [];
    }
    acc[journey.category].push(journey);
    return acc;
  }, {});

  const handleStartJourney = (journey: JourneyContract) => {
    setSelectedJourney(journey);
    setIsStartDialogOpen(true);
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'running':
        return <RefreshCw className="h-4 w-4 text-blue-500 animate-spin" />;
      case 'completed':
        return <CheckCircle className="h-4 w-4 text-green-500" />;
      case 'failed':
        return <XCircle className="h-4 w-4 text-red-500" />;
      case 'pending':
        return <Clock className="h-4 w-4 text-yellow-500" />;
      default:
        return null;
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Workflow className="h-6 w-6" />
            Journey Orchestration
          </h1>
          <p className="text-muted-foreground">
            Start and monitor user journeys across the platform
          </p>
        </div>
        <Button onClick={() => refetch()}>
          <RefreshCw className="h-4 w-4 mr-2" />
          Refresh
        </Button>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Total Journeys</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{contracts?.length || 0}</div>
            <p className="text-xs text-muted-foreground">Available workflows</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Running</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">
              {recentRuns.filter(r => r.status === 'running').length}
            </div>
            <p className="text-xs text-muted-foreground">Active journeys</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Completed Today</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">
              {recentRuns.filter(r => r.status === 'completed').length}
            </div>
            <p className="text-xs text-muted-foreground">Successful runs</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Failed</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-red-600">
              {recentRuns.filter(r => r.status === 'failed').length}
            </div>
            <p className="text-xs text-muted-foreground">Require attention</p>
          </CardContent>
        </Card>
      </div>

      {/* Search and Filter */}
      <div className="flex gap-4">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search journeys..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-10"
          />
        </div>
        <Select value={selectedCategory} onValueChange={setSelectedCategory}>
          <SelectTrigger className="w-48">
            <Filter className="h-4 w-4 mr-2" />
            <SelectValue placeholder="All Categories" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Categories</SelectItem>
            {Object.keys(categoryIcons).map((category) => (
              <SelectItem key={category} value={category}>
                <div className="flex items-center gap-2">
                  {categoryIcons[category]}
                  <span className="capitalize">{category}</span>
                </div>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Journey Tabs */}
      <Tabs defaultValue="catalog" className="space-y-4">
        <TabsList>
          <TabsTrigger value="catalog">Journey Catalog</TabsTrigger>
          <TabsTrigger value="recent">Recent Runs</TabsTrigger>
          <TabsTrigger value="queued">Queued (Offline)</TabsTrigger>
        </TabsList>

        <TabsContent value="catalog" className="space-y-6">
          {isLoading ? (
            <div className="flex items-center justify-center py-12">
              <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          ) : (
            (Object.entries(journeysByCategory) as Array<[string, JourneyContract[]]>).map(([category, journeys]) => (
              <div key={category} className="space-y-3">
                <div className="flex items-center gap-2">
                  <div className={`p-2 rounded-lg ${categoryColors[category]}`}>
                    {categoryIcons[category]}
                  </div>
                  <h2 className="text-lg font-semibold capitalize">{category}</h2>
                  <Badge variant="outline">{journeys.length}</Badge>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                  {journeys.map((journey: JourneyContract) => (
                    <JourneyCard
                      key={journey.journeyKey}
                      journey={journey}
                      onStart={() => handleStartJourney(journey)}
                    />
                  ))}
                </div>
              </div>
            ))
          )}
        </TabsContent>

        <TabsContent value="recent">
          <RecentJourneyRuns runs={recentRuns} />
        </TabsContent>

        <TabsContent value="queued">
          <QueuedJourneys />
        </TabsContent>
      </Tabs>

      {/* Start Journey Dialog */}
      {selectedJourney && (
        <JourneyStartDialog
          journey={selectedJourney}
          open={isStartDialogOpen}
          onOpenChange={setIsStartDialogOpen}
        />
      )}
    </div>
  );
}

// Journey Card Component
function JourneyCard({ journey, onStart }: { journey: JourneyContract; onStart: () => void }) {
  return (
    <Card className="hover:shadow-md transition-shadow">
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between">
          <CardTitle className="text-sm font-medium">{journey.name}</CardTitle>
          <Badge variant="outline" className={categoryColors[journey.category]}>
            {journey.category}
          </Badge>
        </div>
        <CardDescription className="text-xs line-clamp-2">
          {journey.description || `Workflow: ${journey.temporalWorkflow}`}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          <div className="flex flex-wrap gap-1">
            {journey.middlewareHooks.slice(0, 4).map((hook) => (
              <Badge key={hook} variant="secondary" className="text-xs">
                {hook}
              </Badge>
            ))}
            {journey.middlewareHooks.length > 4 && (
              <Badge variant="secondary" className="text-xs">
                +{journey.middlewareHooks.length - 4}
              </Badge>
            )}
          </div>
          <Button size="sm" className="w-full" onClick={onStart}>
            <Play className="h-4 w-4 mr-2" />
            Start Journey
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

// Recent Journey Runs Component
function RecentJourneyRuns({ runs }: { runs: JourneyRun[] }) {
  if (runs.length === 0) {
    return (
      <Card>
        <CardContent className="py-12 text-center">
          <Clock className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
          <p className="text-muted-foreground">No recent journey runs</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-2">
      {runs.map((run) => (
        <Card key={run.journeyRunId}>
          <CardContent className="py-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                {run.status === 'running' ? (
                  <RefreshCw className="h-5 w-5 text-blue-500 animate-spin" />
                ) : run.status === 'completed' ? (
                  <CheckCircle className="h-5 w-5 text-green-500" />
                ) : run.status === 'failed' ? (
                  <XCircle className="h-5 w-5 text-red-500" />
                ) : (
                  <Clock className="h-5 w-5 text-yellow-500" />
                )}
                <div>
                  <p className="font-medium">{run.journeyKey}</p>
                  <p className="text-xs text-muted-foreground">
                    Started {new Date(run.startedAt).toLocaleString()}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-4">
                {run.progress !== undefined && run.status === 'running' && (
                  <div className="w-32">
                    <Progress value={run.progress} className="h-2" />
                    <p className="text-xs text-muted-foreground text-center mt-1">
                      {run.currentStep || `${run.progress}%`}
                    </p>
                  </div>
                )}
                <Badge variant={
                  run.status === 'completed' ? 'default' :
                  run.status === 'failed' ? 'destructive' :
                  run.status === 'running' ? 'secondary' : 'outline'
                }>
                  {run.status}
                </Badge>
                <Button variant="ghost" size="sm">
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

// Queued Journeys Component (for offline support)
function QueuedJourneys() {
  const [queuedJourneys, setQueuedJourneys] = useState<Array<{
    id: string;
    journeyKey: string;
    queuedAt: string;
    attempts: number;
  }>>([]);

  useEffect(() => {
    // Get queued journeys from service worker
    if ('serviceWorker' in navigator && navigator.serviceWorker.controller) {
      const messageChannel = new MessageChannel();
      messageChannel.port1.onmessage = (event) => {
        setQueuedJourneys(event.data.queue || []);
      };
      navigator.serviceWorker.controller.postMessage(
        { type: 'GET_JOURNEY_QUEUE' },
        [messageChannel.port2]
      );
    }
  }, []);

  const handleSyncNow = () => {
    if ('serviceWorker' in navigator && navigator.serviceWorker.controller) {
      navigator.serviceWorker.controller.postMessage({ type: 'SYNC_JOURNEYS' });
    }
  };

  if (queuedJourneys.length === 0) {
    return (
      <Card>
        <CardContent className="py-12 text-center">
          <CheckCircle className="h-12 w-12 mx-auto text-green-500 mb-4" />
          <p className="text-muted-foreground">No journeys queued for sync</p>
          <p className="text-xs text-muted-foreground mt-2">
            Journeys started while offline will appear here
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <p className="text-sm text-muted-foreground">
          {queuedJourneys.length} journey(s) waiting to sync
        </p>
        <Button size="sm" onClick={handleSyncNow}>
          <RefreshCw className="h-4 w-4 mr-2" />
          Sync Now
        </Button>
      </div>
      <div className="space-y-2">
        {queuedJourneys.map((item) => (
          <Card key={item.id}>
            <CardContent className="py-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Clock className="h-5 w-5 text-yellow-500" />
                  <div>
                    <p className="font-medium">{item.journeyKey}</p>
                    <p className="text-xs text-muted-foreground">
                      Queued {new Date(item.queuedAt).toLocaleString()}
                    </p>
                  </div>
                </div>
                <Badge variant="outline">
                  {item.attempts > 0 ? `${item.attempts} attempts` : 'Pending'}
                </Badge>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}

// Journey Start Dialog Component
function JourneyStartDialog({
  journey,
  open,
  onOpenChange,
}: {
  journey: JourneyContract;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [isStarting, setIsStarting] = useState(false);

  const handleStart = async () => {
    setIsStarting(true);
    try {
      // Start the journey via tRPC
      // The actual mutation will depend on the journey type
      console.log('Starting journey:', journey.journeyKey);
      
      // For offline support, queue if not online
      if (!navigator.onLine && 'serviceWorker' in navigator && navigator.serviceWorker.controller) {
        const messageChannel = new MessageChannel();
        messageChannel.port1.onmessage = (event) => {
          if (event.data.queued) {
            alert(`Journey queued for sync: ${event.data.id}`);
          }
        };
        navigator.serviceWorker.controller.postMessage(
          { type: 'QUEUE_JOURNEY_START', journeyKey: journey.journeyKey, input: {} },
          [messageChannel.port2]
        );
      }
      
      onOpenChange(false);
    } catch (error) {
      console.error('Failed to start journey:', error);
    } finally {
      setIsStarting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {categoryIcons[journey.category]}
            {journey.name}
          </DialogTitle>
          <DialogDescription>
            Start this journey workflow
          </DialogDescription>
        </DialogHeader>
        
        <div className="space-y-4">
          <div>
            <h4 className="text-sm font-medium mb-2">Workflow Details</h4>
            <div className="grid grid-cols-2 gap-2 text-sm">
              <div className="text-muted-foreground">Temporal Workflow:</div>
              <div className="font-mono text-xs">{journey.temporalWorkflow}</div>
              <div className="text-muted-foreground">API Endpoint:</div>
              <div className="font-mono text-xs">{journey.orchestratorPath}</div>
            </div>
          </div>
          
          <div>
            <h4 className="text-sm font-medium mb-2">Required Permissions</h4>
            <div className="flex flex-wrap gap-1">
              {journey.requiredPermissions.map((perm) => (
                <Badge key={perm} variant="outline" className="text-xs">
                  {perm}
                </Badge>
              ))}
            </div>
          </div>
          
          <div>
            <h4 className="text-sm font-medium mb-2">Middleware Integration</h4>
            <div className="flex flex-wrap gap-1">
              {journey.middlewareHooks.map((hook) => (
                <Badge key={hook} variant="secondary" className="text-xs">
                  {hook}
                </Badge>
              ))}
            </div>
          </div>
        </div>
        
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleStart} disabled={isStarting}>
            {isStarting ? (
              <>
                <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                Starting...
              </>
            ) : (
              <>
                <Play className="h-4 w-4 mr-2" />
                Start Journey
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default JourneyDashboard;
