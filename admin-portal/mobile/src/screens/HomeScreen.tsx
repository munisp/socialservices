/**
 * Home Screen - Main dashboard for field workers
 */

import React, { useEffect, useState, useCallback } from 'react';
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
import { useNavigation } from '@react-navigation/native';
import { useAppStore } from '../store/appStore';
import { syncService } from '../services/syncService';
import { getSyncStats, getAllBeneficiaries, getAllHouseholds } from '../services/database';

interface StatCardProps {
  icon: string;
  label: string;
  value: string | number;
  color: string;
  onPress?: () => void;
}

function StatCard({ icon, label, value, color, onPress }: StatCardProps) {
  return (
    <TouchableOpacity style={styles.statCard} onPress={onPress} disabled={!onPress}>
      <View style={[styles.statIconContainer, { backgroundColor: color + '20' }]}>
        <Icon name={icon} size={24} color={color} />
      </View>
      <Text style={styles.statValue}>{value}</Text>
      <Text style={styles.statLabel}>{label}</Text>
    </TouchableOpacity>
  );
}

interface QuickActionProps {
  icon: string;
  label: string;
  color: string;
  onPress: () => void;
}

function QuickAction({ icon, label, color, onPress }: QuickActionProps) {
  return (
    <TouchableOpacity style={styles.quickAction} onPress={onPress}>
      <View style={[styles.quickActionIcon, { backgroundColor: color }]}>
        <Icon name={icon} size={28} color="#fff" />
      </View>
      <Text style={styles.quickActionLabel}>{label}</Text>
    </TouchableOpacity>
  );
}

export function HomeScreen() {
  const navigation = useNavigation<any>();
  const { user, syncState, setSyncState, setBeneficiaries, setHouseholds } = useAppStore();
  const [refreshing, setRefreshing] = useState(false);
  const [stats, setStats] = useState({
    beneficiaries: 0,
    households: 0,
    pendingSync: 0,
    todayEnrollments: 0,
  });

  const loadData = useCallback(async () => {
    try {
      const [syncStats, beneficiaries, households] = await Promise.all([
        getSyncStats(),
        getAllBeneficiaries(),
        getAllHouseholds(),
      ]);

      setBeneficiaries(beneficiaries);
      setHouseholds(households);

      // Count today's enrollments
      const today = new Date();
      today.setHours(0, 0, 0, 0);
      const todayEnrollments = beneficiaries.filter(
        (b) => b.createdAt >= today.getTime()
      ).length;

      setStats({
        beneficiaries: syncStats.beneficiaries.total,
        households: syncStats.households.total,
        pendingSync: syncStats.queueSize,
        todayEnrollments,
      });

      setSyncState({
        pendingChanges: syncStats.queueSize,
      });
    } catch (error) {
      console.error('Failed to load data:', error);
    }
  }, [setBeneficiaries, setHouseholds, setSyncState]);

  useEffect(() => {
    loadData();

    // Subscribe to sync events
    const unsubscribe = syncService.subscribe((event, data) => {
      if (event === 'online' || event === 'offline') {
        setSyncState({ isOnline: event === 'online' });
      } else if (event === 'syncStarted') {
        setSyncState({ isSyncing: true });
      } else if (event === 'syncCompleted' || event === 'syncFailed') {
        setSyncState({ isSyncing: false });
        loadData();
      }
    });

    return () => unsubscribe();
  }, [loadData, setSyncState]);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await loadData();
    if (syncState.isOnline) {
      await syncService.sync();
    }
    setRefreshing(false);
  }, [loadData, syncState.isOnline]);

  const handleSync = async () => {
    if (!syncState.isOnline) {
      Alert.alert('Offline', 'You are currently offline. Data will sync when connection is restored.');
      return;
    }

    const result = await syncService.sync();
    if (result.success) {
      Alert.alert('Sync Complete', `Synced ${result.synced} records.`);
    } else {
      Alert.alert('Sync Failed', result.errors.join('\n'));
    }
  };

  return (
    <ScrollView
      style={styles.container}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
      }
    >
      {/* Header */}
      <View style={styles.header}>
        <View>
          <Text style={styles.greeting}>Hello, {user?.name || 'Field Worker'}</Text>
          <Text style={styles.date}>{new Date().toLocaleDateString('en-US', {
            weekday: 'long',
            year: 'numeric',
            month: 'long',
            day: 'numeric',
          })}</Text>
        </View>
        <TouchableOpacity onPress={handleSync} style={styles.syncButton}>
          <Icon
            name={syncState.isSyncing ? 'sync' : syncState.isOnline ? 'cloud-check' : 'cloud-off-outline'}
            size={24}
            color={syncState.isOnline ? '#4CAF50' : '#FF9800'}
          />
          {stats.pendingSync > 0 && (
            <View style={styles.syncBadge}>
              <Text style={styles.syncBadgeText}>{stats.pendingSync}</Text>
            </View>
          )}
        </TouchableOpacity>
      </View>

      {/* Connection Status */}
      {!syncState.isOnline && (
        <View style={styles.offlineBanner}>
          <Icon name="wifi-off" size={16} color="#fff" />
          <Text style={styles.offlineBannerText}>
            You are offline. Changes will sync when connected.
          </Text>
        </View>
      )}

      {/* Stats */}
      <View style={styles.statsContainer}>
        <StatCard
          icon="account-group"
          label="Beneficiaries"
          value={stats.beneficiaries}
          color="#2196F3"
          onPress={() => navigation.navigate('Beneficiaries')}
        />
        <StatCard
          icon="home-group"
          label="Households"
          value={stats.households}
          color="#9C27B0"
          onPress={() => navigation.navigate('Households')}
        />
        <StatCard
          icon="account-plus"
          label="Today"
          value={stats.todayEnrollments}
          color="#4CAF50"
        />
        <StatCard
          icon="cloud-sync"
          label="Pending"
          value={stats.pendingSync}
          color="#FF9800"
          onPress={handleSync}
        />
      </View>

      {/* Quick Actions */}
      <Text style={styles.sectionTitle}>Quick Actions</Text>
      <View style={styles.quickActionsContainer}>
        <QuickAction
          icon="account-plus"
          label="New Beneficiary"
          color="#2196F3"
          onPress={() => navigation.navigate('EnrollBeneficiary')}
        />
        <QuickAction
          icon="home-plus"
          label="New Household"
          color="#9C27B0"
          onPress={() => navigation.navigate('EnrollHousehold')}
        />
        <QuickAction
          icon="clipboard-list"
          label="PMT Survey"
          color="#4CAF50"
          onPress={() => navigation.navigate('PMTSurvey')}
        />
        <QuickAction
          icon="card-account-details"
          label="Verify ID"
          color="#FF5722"
          onPress={() => navigation.navigate('VerifyID')}
        />
      </View>

      {/* Recent Activity */}
      <Text style={styles.sectionTitle}>Recent Activity</Text>
      <View style={styles.activityContainer}>
        <View style={styles.activityItem}>
          <Icon name="account-check" size={20} color="#4CAF50" />
          <View style={styles.activityContent}>
            <Text style={styles.activityText}>Beneficiary enrolled</Text>
            <Text style={styles.activityTime}>2 minutes ago</Text>
          </View>
        </View>
        <View style={styles.activityItem}>
          <Icon name="cloud-upload" size={20} color="#2196F3" />
          <View style={styles.activityContent}>
            <Text style={styles.activityText}>Data synced to server</Text>
            <Text style={styles.activityTime}>15 minutes ago</Text>
          </View>
        </View>
        <View style={styles.activityItem}>
          <Icon name="clipboard-check" size={20} color="#9C27B0" />
          <View style={styles.activityContent}>
            <Text style={styles.activityText}>PMT survey completed</Text>
            <Text style={styles.activityTime}>1 hour ago</Text>
          </View>
        </View>
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: 20,
    backgroundColor: '#fff',
  },
  greeting: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#333',
  },
  date: {
    fontSize: 14,
    color: '#666',
    marginTop: 4,
  },
  syncButton: {
    position: 'relative',
    padding: 8,
  },
  syncBadge: {
    position: 'absolute',
    top: 0,
    right: 0,
    backgroundColor: '#FF5722',
    borderRadius: 10,
    minWidth: 20,
    height: 20,
    justifyContent: 'center',
    alignItems: 'center',
  },
  syncBadgeText: {
    color: '#fff',
    fontSize: 12,
    fontWeight: 'bold',
  },
  offlineBanner: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#FF9800',
    padding: 10,
    gap: 8,
  },
  offlineBannerText: {
    color: '#fff',
    fontSize: 14,
  },
  statsContainer: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    padding: 10,
    gap: 10,
  },
  statCard: {
    flex: 1,
    minWidth: '45%',
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 16,
    alignItems: 'center',
  },
  statIconContainer: {
    width: 48,
    height: 48,
    borderRadius: 24,
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 8,
  },
  statValue: {
    fontSize: 28,
    fontWeight: 'bold',
    color: '#333',
  },
  statLabel: {
    fontSize: 14,
    color: '#666',
    marginTop: 4,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: '#333',
    paddingHorizontal: 20,
    paddingTop: 20,
    paddingBottom: 10,
  },
  quickActionsContainer: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    paddingHorizontal: 10,
    gap: 10,
  },
  quickAction: {
    flex: 1,
    minWidth: '45%',
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 16,
    alignItems: 'center',
  },
  quickActionIcon: {
    width: 56,
    height: 56,
    borderRadius: 28,
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 8,
  },
  quickActionLabel: {
    fontSize: 14,
    fontWeight: '500',
    color: '#333',
    textAlign: 'center',
  },
  activityContainer: {
    backgroundColor: '#fff',
    marginHorizontal: 10,
    borderRadius: 12,
    padding: 16,
    marginBottom: 20,
  },
  activityItem: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 12,
    borderBottomWidth: 1,
    borderBottomColor: '#f0f0f0',
  },
  activityContent: {
    marginLeft: 12,
    flex: 1,
  },
  activityText: {
    fontSize: 14,
    color: '#333',
  },
  activityTime: {
    fontSize: 12,
    color: '#999',
    marginTop: 2,
  },
});

export default HomeScreen;
