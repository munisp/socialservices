import React, { useState, useEffect, useCallback } from 'react';
import { 
  Cloud, 
  CloudOff, 
  RefreshCw, 
  AlertTriangle, 
  CheckCircle, 
  Clock,
  Wifi,
  WifiOff,
  Database,
  Upload,
  Download
} from 'lucide-react';
import { Button } from '@/components/ui/button';
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
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from '@/components/ui/alert';
import { offlineSync, SyncResult } from '@/lib/offline/offlineSync';

interface SyncStats {
  beneficiaries: { total: number; pending: number; synced: number; conflict: number };
  households: { total: number; pending: number; synced: number; conflict: number };
  queueSize: number;
  conflicts: number;
  lastSync: string | null;
}

export function OfflineSyncStatus() {
  const [isOnline, setIsOnline] = useState(navigator.onLine);
  const [isSyncing, setIsSyncing] = useState(false);
  const [stats, setStats] = useState<SyncStats | null>(null);
  const [lastSyncResult, setLastSyncResult] = useState<SyncResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadStats = useCallback(async () => {
    try {
      const newStats = await offlineSync.getStats();
      setStats(newStats);
    } catch (err) {
      console.error('Failed to load sync stats:', err);
    }
  }, []);

  useEffect(() => {
    // Initialize offline sync
    offlineSync.init().then(() => {
      loadStats();
    });

    // Event listeners
    const handleOnline = () => {
      setIsOnline(true);
      loadStats();
    };

    const handleOffline = () => {
      setIsOnline(false);
    };

    const handleSyncStarted = () => {
      setIsSyncing(true);
      setError(null);
    };

    const handleSyncCompleted = (result: unknown) => {
      setIsSyncing(false);
      setLastSyncResult(result as SyncResult);
      loadStats();
    };

    const handleSyncFailed = (result: unknown) => {
      setIsSyncing(false);
      const syncResult = result as SyncResult;
      setError(syncResult.errors.join(', '));
      loadStats();
    };

    offlineSync.on('online', handleOnline);
    offlineSync.on('offline', handleOffline);
    offlineSync.on('syncStarted', handleSyncStarted);
    offlineSync.on('syncCompleted', handleSyncCompleted);
    offlineSync.on('syncFailed', handleSyncFailed);

    // Cleanup
    return () => {
      offlineSync.off('online', handleOnline);
      offlineSync.off('offline', handleOffline);
      offlineSync.off('syncStarted', handleSyncStarted);
      offlineSync.off('syncCompleted', handleSyncCompleted);
      offlineSync.off('syncFailed', handleSyncFailed);
    };
  }, [loadStats]);

  const handleSync = async () => {
    try {
      await offlineSync.sync();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sync failed');
    }
  };

  const formatLastSync = (timestamp: string | null) => {
    if (!timestamp) return 'Never';
    const date = new Date(timestamp);
    const now = new Date();
    const diff = now.getTime() - date.getTime();
    
    if (diff < 60000) return 'Just now';
    if (diff < 3600000) return `${Math.floor(diff / 60000)} min ago`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)} hours ago`;
    return date.toLocaleDateString();
  };

  const totalPending = stats 
    ? stats.beneficiaries.pending + stats.households.pending 
    : 0;

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="sm" className="relative">
          {isOnline ? (
            <Wifi className="h-4 w-4 text-green-500" />
          ) : (
            <WifiOff className="h-4 w-4 text-red-500" />
          )}
          {totalPending > 0 && (
            <Badge 
              variant="destructive" 
              className="absolute -top-1 -right-1 h-4 w-4 p-0 flex items-center justify-center text-xs"
            >
              {totalPending}
            </Badge>
          )}
          {stats?.conflicts && stats.conflicts > 0 && (
            <AlertTriangle className="h-3 w-3 text-yellow-500 absolute -bottom-1 -right-1" />
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80" align="end">
        <div className="space-y-4">
          {/* Connection Status */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              {isOnline ? (
                <>
                  <Cloud className="h-5 w-5 text-green-500" />
                  <span className="font-medium">Online</span>
                </>
              ) : (
                <>
                  <CloudOff className="h-5 w-5 text-red-500" />
                  <span className="font-medium">Offline</span>
                </>
              )}
            </div>
            <Button 
              size="sm" 
              variant="outline" 
              onClick={handleSync}
              disabled={!isOnline || isSyncing}
            >
              <RefreshCw className={`h-4 w-4 mr-1 ${isSyncing ? 'animate-spin' : ''}`} />
              {isSyncing ? 'Syncing...' : 'Sync Now'}
            </Button>
          </div>

          {/* Last Sync */}
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Clock className="h-4 w-4" />
            <span>Last sync: {formatLastSync(stats?.lastSync || null)}</span>
          </div>

          {/* Sync Progress */}
          {isSyncing && (
            <div className="space-y-2">
              <Progress value={50} className="h-2" />
              <p className="text-xs text-muted-foreground text-center">
                Syncing changes...
              </p>
            </div>
          )}

          {/* Error Alert */}
          {error && (
            <Alert variant="destructive">
              <AlertTriangle className="h-4 w-4" />
              <AlertTitle>Sync Error</AlertTitle>
              <AlertDescription className="text-xs">{error}</AlertDescription>
            </Alert>
          )}

          {/* Conflicts Alert */}
          {stats?.conflicts && stats.conflicts > 0 && (
            <Alert>
              <AlertTriangle className="h-4 w-4 text-yellow-500" />
              <AlertTitle>Conflicts Detected</AlertTitle>
              <AlertDescription className="text-xs">
                {stats.conflicts} conflict(s) need resolution
              </AlertDescription>
            </Alert>
          )}

          {/* Stats */}
          {stats && (
            <div className="space-y-3">
              <div className="text-sm font-medium">Local Data</div>
              
              {/* Beneficiaries */}
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">Beneficiaries</span>
                <div className="flex items-center gap-2">
                  <Badge variant="outline" className="text-xs">
                    {stats.beneficiaries.total} total
                  </Badge>
                  {stats.beneficiaries.pending > 0 && (
                    <Badge variant="secondary" className="text-xs">
                      <Upload className="h-3 w-3 mr-1" />
                      {stats.beneficiaries.pending}
                    </Badge>
                  )}
                </div>
              </div>

              {/* Households */}
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">Households</span>
                <div className="flex items-center gap-2">
                  <Badge variant="outline" className="text-xs">
                    {stats.households.total} total
                  </Badge>
                  {stats.households.pending > 0 && (
                    <Badge variant="secondary" className="text-xs">
                      <Upload className="h-3 w-3 mr-1" />
                      {stats.households.pending}
                    </Badge>
                  )}
                </div>
              </div>

              {/* Queue */}
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">Pending Changes</span>
                <Badge variant={stats.queueSize > 0 ? 'default' : 'outline'} className="text-xs">
                  {stats.queueSize}
                </Badge>
              </div>
            </div>
          )}

          {/* Last Sync Result */}
          {lastSyncResult && (
            <div className="pt-2 border-t">
              <div className="text-sm font-medium mb-2">Last Sync Result</div>
              <div className="grid grid-cols-3 gap-2 text-center">
                <div className="p-2 bg-green-50 rounded">
                  <CheckCircle className="h-4 w-4 text-green-500 mx-auto mb-1" />
                  <div className="text-xs font-medium">{lastSyncResult.synced}</div>
                  <div className="text-xs text-muted-foreground">Synced</div>
                </div>
                <div className="p-2 bg-red-50 rounded">
                  <AlertTriangle className="h-4 w-4 text-red-500 mx-auto mb-1" />
                  <div className="text-xs font-medium">{lastSyncResult.failed}</div>
                  <div className="text-xs text-muted-foreground">Failed</div>
                </div>
                <div className="p-2 bg-yellow-50 rounded">
                  <Database className="h-4 w-4 text-yellow-500 mx-auto mb-1" />
                  <div className="text-xs font-medium">{lastSyncResult.conflicts}</div>
                  <div className="text-xs text-muted-foreground">Conflicts</div>
                </div>
              </div>
            </div>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}

// Full page sync management component
export function OfflineSyncManager() {
  const [stats, setStats] = useState<SyncStats | null>(null);
  const [syncLog, setSyncLog] = useState<Array<{
    id: string;
    action: string;
    details: Record<string, unknown>;
    timestamp: number;
  }>>([]);
  const [isSyncing, setIsSyncing] = useState(false);

  useEffect(() => {
    offlineSync.init().then(async () => {
      const newStats = await offlineSync.getStats();
      setStats(newStats);
      const log = await offlineSync.getSyncLog(50);
      setSyncLog(log);
    });

    const handleSyncCompleted = async () => {
      setIsSyncing(false);
      const newStats = await offlineSync.getStats();
      setStats(newStats);
      const log = await offlineSync.getSyncLog(50);
      setSyncLog(log);
    };

    offlineSync.on('syncStarted', () => setIsSyncing(true));
    offlineSync.on('syncCompleted', handleSyncCompleted);
    offlineSync.on('syncFailed', handleSyncCompleted);

    return () => {
      offlineSync.off('syncStarted', () => setIsSyncing(true));
      offlineSync.off('syncCompleted', handleSyncCompleted);
      offlineSync.off('syncFailed', handleSyncCompleted);
    };
  }, []);

  const handleSync = async () => {
    await offlineSync.sync();
  };

  const handleClearQueue = async () => {
    await offlineSync.clearSyncQueue();
    const newStats = await offlineSync.getStats();
    setStats(newStats);
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold">Offline Sync Manager</h2>
          <p className="text-muted-foreground">
            Manage offline data synchronization and resolve conflicts
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={handleClearQueue}>
            Clear Queue
          </Button>
          <Button onClick={handleSync} disabled={isSyncing}>
            <RefreshCw className={`h-4 w-4 mr-2 ${isSyncing ? 'animate-spin' : ''}`} />
            {isSyncing ? 'Syncing...' : 'Sync Now'}
          </Button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Beneficiaries</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats?.beneficiaries.total || 0}</div>
            <div className="flex gap-2 mt-2">
              <Badge variant="outline" className="text-xs">
                <CheckCircle className="h-3 w-3 mr-1 text-green-500" />
                {stats?.beneficiaries.synced || 0}
              </Badge>
              <Badge variant="secondary" className="text-xs">
                <Clock className="h-3 w-3 mr-1" />
                {stats?.beneficiaries.pending || 0}
              </Badge>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Households</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats?.households.total || 0}</div>
            <div className="flex gap-2 mt-2">
              <Badge variant="outline" className="text-xs">
                <CheckCircle className="h-3 w-3 mr-1 text-green-500" />
                {stats?.households.synced || 0}
              </Badge>
              <Badge variant="secondary" className="text-xs">
                <Clock className="h-3 w-3 mr-1" />
                {stats?.households.pending || 0}
              </Badge>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Pending Changes</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats?.queueSize || 0}</div>
            <p className="text-xs text-muted-foreground mt-2">
              Changes waiting to sync
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Conflicts</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-yellow-600">{stats?.conflicts || 0}</div>
            <p className="text-xs text-muted-foreground mt-2">
              Require resolution
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Sync Log */}
      <Card>
        <CardHeader>
          <CardTitle>Sync Log</CardTitle>
          <CardDescription>Recent synchronization activity</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-2 max-h-96 overflow-y-auto">
            {syncLog.length === 0 ? (
              <p className="text-sm text-muted-foreground text-center py-4">
                No sync activity yet
              </p>
            ) : (
              syncLog.map((entry) => (
                <div 
                  key={entry.id} 
                  className="flex items-center justify-between p-2 bg-muted/50 rounded text-sm"
                >
                  <div className="flex items-center gap-2">
                    {entry.action === 'sync_completed' && (
                      <CheckCircle className="h-4 w-4 text-green-500" />
                    )}
                    {entry.action === 'sync_failed' && (
                      <AlertTriangle className="h-4 w-4 text-red-500" />
                    )}
                    {entry.action === 'sync_started' && (
                      <RefreshCw className="h-4 w-4 text-blue-500" />
                    )}
                    {entry.action === 'conflict_detected' && (
                      <AlertTriangle className="h-4 w-4 text-yellow-500" />
                    )}
                    {entry.action === 'conflict_resolved' && (
                      <CheckCircle className="h-4 w-4 text-green-500" />
                    )}
                    <span className="capitalize">{entry.action.replace(/_/g, ' ')}</span>
                  </div>
                  <span className="text-xs text-muted-foreground">
                    {new Date(entry.timestamp).toLocaleString()}
                  </span>
                </div>
              ))
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

export default OfflineSyncStatus;
