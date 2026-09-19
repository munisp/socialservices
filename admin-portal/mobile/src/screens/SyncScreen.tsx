/**
 * Sync Screen - Manage offline sync and view sync status
 */

import React, { useState, useEffect, useCallback } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
  RefreshControl,
  Alert,
} from 'react-native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import { useAppStore } from '../store/appStore';
import { syncService, SyncResult } from '../services/syncService';
import { getSyncStats, getSyncQueue, SyncQueueItem } from '../services/database';

export function SyncScreen() {
  const { syncState, setSyncState } = useAppStore();
  const [refreshing, setRefreshing] = useState(false);
  const [lastSyncResult, setLastSyncResult] = useState<SyncResult | null>(null);
  const [queueItems, setQueueItems] = useState<SyncQueueItem[]>([]);
  const [stats, setStats] = useState({
    beneficiaries: { total: 0, pending: 0, synced: 0 },
    households: { total: 0, pending: 0, synced: 0 },
    queueSize: 0,
  });

  const loadData = useCallback(async () => {
    const [syncStats, queue, status] = await Promise.all([
      getSyncStats(),
      getSyncQueue(),
      syncService.getStatus(),
    ]);

    setStats(syncStats);
    setQueueItems(queue.slice(0, 20));
    setSyncState({
      isOnline: status.isOnline,
      isSyncing: status.isSyncing,
      lastSync: status.lastSync,
      pendingChanges: status.pendingChanges,
    });
  }, [setSyncState]);

  useEffect(() => {
    loadData();

    const unsubscribe = syncService.subscribe((event, data) => {
      if (event === 'syncCompleted' || event === 'syncFailed') {
        setLastSyncResult(data as SyncResult);
        loadData();
      }
      if (event === 'online' || event === 'offline') {
        setSyncState({ isOnline: event === 'online' });
      }
    });

    return () => unsubscribe();
  }, [loadData, setSyncState]);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await loadData();
    setRefreshing(false);
  }, [loadData]);

  const handleSync = async () => {
    if (!syncState.isOnline) {
      Alert.alert('Offline', 'You are currently offline. Please connect to sync.');
      return;
    }

    const result = await syncService.sync();
    setLastSyncResult(result);

    if (result.success) {
      Alert.alert(
        'Sync Complete',
        `Synced: ${result.synced}\nFailed: ${result.failed}\nConflicts: ${result.conflicts}`
      );
    } else {
      Alert.alert('Sync Failed', result.errors.join('\n'));
    }
  };

  const formatDate = (dateStr: string | null) => {
    if (!dateStr) return 'Never';
    const date = new Date(dateStr);
    return date.toLocaleString();
  };

  return (
    <ScrollView
      style={styles.container}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
      }
    >
      {/* Connection Status */}
      <View style={[styles.statusCard, syncState.isOnline ? styles.statusOnline : styles.statusOffline]}>
        <Icon
          name={syncState.isOnline ? 'wifi' : 'wifi-off'}
          size={32}
          color="#fff"
        />
        <View style={styles.statusInfo}>
          <Text style={styles.statusTitle}>
            {syncState.isOnline ? 'Online' : 'Offline'}
          </Text>
          <Text style={styles.statusSubtitle}>
            {syncState.isOnline
              ? 'Connected to server'
              : 'Changes will sync when connected'}
          </Text>
        </View>
      </View>

      {/* Sync Button */}
      <TouchableOpacity
        style={[styles.syncButton, syncState.isSyncing && styles.syncButtonDisabled]}
        onPress={handleSync}
        disabled={syncState.isSyncing || !syncState.isOnline}
      >
        <Icon
          name={syncState.isSyncing ? 'sync' : 'cloud-sync'}
          size={24}
          color="#fff"
        />
        <Text style={styles.syncButtonText}>
          {syncState.isSyncing ? 'Syncing...' : 'Sync Now'}
        </Text>
      </TouchableOpacity>

      {/* Last Sync */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Last Sync</Text>
        <View style={styles.infoCard}>
          <View style={styles.infoRow}>
            <Icon name="clock-outline" size={20} color="#666" />
            <Text style={styles.infoText}>{formatDate(syncState.lastSync)}</Text>
          </View>
          {lastSyncResult && (
            <>
              <View style={styles.infoRow}>
                <Icon name="check-circle" size={20} color="#4CAF50" />
                <Text style={styles.infoText}>{lastSyncResult.synced} synced</Text>
              </View>
              {lastSyncResult.failed > 0 && (
                <View style={styles.infoRow}>
                  <Icon name="alert-circle" size={20} color="#f44336" />
                  <Text style={styles.infoText}>{lastSyncResult.failed} failed</Text>
                </View>
              )}
              {lastSyncResult.conflicts > 0 && (
                <View style={styles.infoRow}>
                  <Icon name="alert" size={20} color="#FF9800" />
                  <Text style={styles.infoText}>{lastSyncResult.conflicts} conflicts</Text>
                </View>
              )}
            </>
          )}
        </View>
      </View>

      {/* Local Data Stats */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Local Data</Text>
        <View style={styles.statsGrid}>
          <View style={styles.statCard}>
            <Icon name="account-group" size={24} color="#2196F3" />
            <Text style={styles.statValue}>{stats.beneficiaries.total}</Text>
            <Text style={styles.statLabel}>Beneficiaries</Text>
            <View style={styles.statDetails}>
              <Text style={styles.statDetailText}>
                {stats.beneficiaries.synced} synced
              </Text>
              <Text style={styles.statDetailText}>
                {stats.beneficiaries.pending} pending
              </Text>
            </View>
          </View>
          <View style={styles.statCard}>
            <Icon name="home-group" size={24} color="#9C27B0" />
            <Text style={styles.statValue}>{stats.households.total}</Text>
            <Text style={styles.statLabel}>Households</Text>
            <View style={styles.statDetails}>
              <Text style={styles.statDetailText}>
                {stats.households.synced} synced
              </Text>
              <Text style={styles.statDetailText}>
                {stats.households.pending} pending
              </Text>
            </View>
          </View>
        </View>
      </View>

      {/* Pending Queue */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>
          Pending Changes ({stats.queueSize})
        </Text>
        {queueItems.length === 0 ? (
          <View style={styles.emptyQueue}>
            <Icon name="check-circle" size={48} color="#4CAF50" />
            <Text style={styles.emptyQueueText}>All changes synced</Text>
          </View>
        ) : (
          <View style={styles.queueList}>
            {queueItems.map((item) => (
              <View key={item.id} style={styles.queueItem}>
                <Icon
                  name={item.entityType === 'beneficiary' ? 'account' : 'home'}
                  size={20}
                  color="#666"
                />
                <View style={styles.queueItemInfo}>
                  <Text style={styles.queueItemTitle}>
                    {item.operation} {item.entityType}
                  </Text>
                  <Text style={styles.queueItemId}>{item.entityId.slice(0, 20)}...</Text>
                </View>
                <View
                  style={[
                    styles.queueItemStatus,
                    item.status === 'pending' && styles.queueItemPending,
                    item.status === 'failed' && styles.queueItemFailed,
                  ]}
                >
                  <Text style={styles.queueItemStatusText}>{item.status}</Text>
                </View>
              </View>
            ))}
          </View>
        )}
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  statusCard: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 20,
    margin: 16,
    borderRadius: 12,
  },
  statusOnline: {
    backgroundColor: '#4CAF50',
  },
  statusOffline: {
    backgroundColor: '#FF9800',
  },
  statusInfo: {
    marginLeft: 16,
  },
  statusTitle: {
    fontSize: 20,
    fontWeight: '600',
    color: '#fff',
  },
  statusSubtitle: {
    fontSize: 14,
    color: 'rgba(255,255,255,0.8)',
    marginTop: 2,
  },
  syncButton: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#2196F3',
    marginHorizontal: 16,
    padding: 16,
    borderRadius: 12,
    gap: 8,
  },
  syncButtonDisabled: {
    opacity: 0.6,
  },
  syncButtonText: {
    fontSize: 18,
    fontWeight: '600',
    color: '#fff',
  },
  section: {
    marginTop: 24,
    paddingHorizontal: 16,
  },
  sectionTitle: {
    fontSize: 16,
    fontWeight: '600',
    color: '#333',
    marginBottom: 12,
  },
  infoCard: {
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 16,
  },
  infoRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 12,
    paddingVertical: 8,
  },
  infoText: {
    fontSize: 14,
    color: '#333',
  },
  statsGrid: {
    flexDirection: 'row',
    gap: 12,
  },
  statCard: {
    flex: 1,
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 16,
    alignItems: 'center',
  },
  statValue: {
    fontSize: 28,
    fontWeight: 'bold',
    color: '#333',
    marginTop: 8,
  },
  statLabel: {
    fontSize: 14,
    color: '#666',
    marginTop: 4,
  },
  statDetails: {
    marginTop: 8,
    alignItems: 'center',
  },
  statDetailText: {
    fontSize: 12,
    color: '#999',
  },
  emptyQueue: {
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 32,
    alignItems: 'center',
  },
  emptyQueueText: {
    fontSize: 16,
    color: '#4CAF50',
    marginTop: 12,
  },
  queueList: {
    backgroundColor: '#fff',
    borderRadius: 12,
    overflow: 'hidden',
  },
  queueItem: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 12,
    borderBottomWidth: 1,
    borderBottomColor: '#f0f0f0',
  },
  queueItemInfo: {
    flex: 1,
    marginLeft: 12,
  },
  queueItemTitle: {
    fontSize: 14,
    fontWeight: '500',
    color: '#333',
    textTransform: 'capitalize',
  },
  queueItemId: {
    fontSize: 12,
    color: '#999',
    marginTop: 2,
  },
  queueItemStatus: {
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: 4,
    backgroundColor: '#E0E0E0',
  },
  queueItemPending: {
    backgroundColor: '#FFF3E0',
  },
  queueItemFailed: {
    backgroundColor: '#FFEBEE',
  },
  queueItemStatusText: {
    fontSize: 12,
    fontWeight: '500',
    textTransform: 'capitalize',
  },
});

export default SyncScreen;
