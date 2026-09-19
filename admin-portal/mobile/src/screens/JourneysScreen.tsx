/**
 * Journeys Screen for Mobile App
 * Displays available journeys and allows starting them with offline support
 */

import React, { useState, useEffect, useCallback } from 'react';
import {
  View,
  Text,
  StyleSheet,
  FlatList,
  TouchableOpacity,
  RefreshControl,
  Alert,
  ActivityIndicator,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { syncService } from '../services/syncService';

// Journey category configuration
const CATEGORY_CONFIG: Record<string, { icon: string; color: string; bgColor: string }> = {
  enrollment: { icon: 'person-add', color: '#2563eb', bgColor: '#dbeafe' },
  payments: { icon: 'card', color: '#16a34a', bgColor: '#dcfce7' },
  grievance: { icon: 'chatbubble', color: '#ca8a04', bgColor: '#fef9c3' },
  lifecycle: { icon: 'person', color: '#9333ea', bgColor: '#f3e8ff' },
  admin: { icon: 'shield', color: '#dc2626', bgColor: '#fee2e2' },
  reporting: { icon: 'bar-chart', color: '#4f46e5', bgColor: '#e0e7ff' },
  fraud: { icon: 'warning', color: '#ea580c', bgColor: '#ffedd5' },
};

interface JourneyContract {
  journeyKey: string;
  name: string;
  category: string;
  description?: string;
  temporalWorkflow: string;
  middlewareHooks: string[];
}

interface QueuedJourney {
  id: string;
  journeyKey: string;
  queuedAt: string;
  attempts: number;
}

export default function JourneysScreen({ navigation }: { navigation: any }) {
  const [contracts, setContracts] = useState<JourneyContract[]>([]);
  const [queuedJourneys, setQueuedJourneys] = useState<QueuedJourney[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isOnline, setIsOnline] = useState(true);
  const [selectedCategory, setSelectedCategory] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    try {
      // Load journey contracts
      const result = await syncService.getJourneyContracts();
      if (result.success && result.contracts) {
        setContracts(result.contracts as JourneyContract[]);
      }

      // Load queued journeys count
      const queueCount = await syncService.getQueuedJourneyCount();
      if (queueCount > 0) {
        // Get full queue details if there are queued items
        const status = await syncService.getStatus();
        setQueuedJourneys([]); // Would need to expose queue details
      }

      setIsOnline(syncService.getIsOnline());
    } catch (error) {
      console.error('Failed to load journey data:', error);
    } finally {
      setIsLoading(false);
      setIsRefreshing(false);
    }
  }, []);

  useEffect(() => {
    loadData();

    // Subscribe to sync events
    const unsubscribe = syncService.subscribe((event, data) => {
      if (event === 'online' || event === 'offline') {
        setIsOnline(event === 'online');
      }
      if (event === 'journeyQueued' || event === 'journeysSynced') {
        loadData();
      }
    });

    return () => unsubscribe();
  }, [loadData]);

  const handleRefresh = () => {
    setIsRefreshing(true);
    loadData();
  };

  const handleStartJourney = async (journey: JourneyContract) => {
    // Navigate to the appropriate form based on journey type
    switch (journey.journeyKey) {
      case 'beneficiary-enrollment':
        navigation.navigate('EnrollBeneficiary', { journeyKey: journey.journeyKey });
        break;
      case 'household-registration':
        navigation.navigate('Households', { journeyKey: journey.journeyKey, mode: 'register' });
        break;
      case 'kyc-verification':
        navigation.navigate('VerifyID', { journeyKey: journey.journeyKey });
        break;
      case 'profile-update':
        navigation.navigate('Beneficiaries', { journeyKey: journey.journeyKey, mode: 'update' });
        break;
      // Note: grievance-submission and other journeys without dedicated screens
      // will fall through to the default case which shows a confirmation dialog
      default:
        // Check if journey requires input - warn user if starting with empty input
        const requiresInput = journey.requiredInputs && journey.requiredInputs.length > 0;
        const warningMessage = requiresInput 
          ? `\n\nWarning: This journey may require additional input (${journey.requiredInputs?.join(', ')}). Starting without input may cause the journey to fail.`
          : '';
        
        // For journeys without dedicated screens, show a confirmation with validation warning
        Alert.alert(
          'Start Journey',
          `Start ${journey.name}?${!isOnline ? '\n\nYou are offline. This journey will be queued for sync.' : ''}${warningMessage}`,
          [
            { text: 'Cancel', style: 'cancel' },
            {
              text: requiresInput ? 'Start Anyway' : 'Start',
              style: requiresInput ? 'destructive' : 'default',
              onPress: async () => {
                const result = await syncService.startJourney(journey.journeyKey, {});
                if (result.error === 'Authentication required') {
                  Alert.alert('Sign In Required', 'Please sign in to start journeys.');
                } else if (result.queued) {
                  Alert.alert('Queued', 'Journey has been queued and will start when online.');
                } else if (result.success) {
                  Alert.alert('Started', `Journey started: ${result.journeyRunId}`);
                } else {
                  Alert.alert('Error', result.error || 'Failed to start journey');
                }
              },
            },
          ]
        );
    }
  };

  const handleSyncQueued = async () => {
    if (!isOnline) {
      Alert.alert('Offline', 'Cannot sync while offline');
      return;
    }

    const result = await syncService.syncQueuedJourneys();
    Alert.alert(
      'Sync Complete',
      `Synced: ${result.synced}\nFailed: ${result.failed}`
    );
  };

  const filteredContracts = selectedCategory
    ? contracts.filter(c => c.category === selectedCategory)
    : contracts;

  const categories = [...new Set(contracts.map(c => c.category))];

  const renderCategoryFilter = () => (
    <View style={styles.categoryFilter}>
      <TouchableOpacity
        style={[
          styles.categoryChip,
          !selectedCategory && styles.categoryChipActive,
        ]}
        onPress={() => setSelectedCategory(null)}
      >
        <Text style={[
          styles.categoryChipText,
          !selectedCategory && styles.categoryChipTextActive,
        ]}>
          All
        </Text>
      </TouchableOpacity>
      {categories.map(category => (
        <TouchableOpacity
          key={category}
          style={[
            styles.categoryChip,
            selectedCategory === category && styles.categoryChipActive,
            { backgroundColor: selectedCategory === category ? CATEGORY_CONFIG[category]?.color : '#f3f4f6' },
          ]}
          onPress={() => setSelectedCategory(category)}
        >
          <Text style={[
            styles.categoryChipText,
            selectedCategory === category && styles.categoryChipTextActive,
          ]}>
            {category.charAt(0).toUpperCase() + category.slice(1)}
          </Text>
        </TouchableOpacity>
      ))}
    </View>
  );

  const renderJourneyItem = ({ item }: { item: JourneyContract }) => {
    const config = CATEGORY_CONFIG[item.category] || { color: '#6b7280', bgColor: '#f3f4f6' };
    
    return (
      <TouchableOpacity
        style={styles.journeyCard}
        onPress={() => handleStartJourney(item)}
      >
        <View style={[styles.journeyIcon, { backgroundColor: config.bgColor }]}>
          <Text style={[styles.journeyIconText, { color: config.color }]}>
            {item.name.charAt(0)}
          </Text>
        </View>
        <View style={styles.journeyContent}>
          <Text style={styles.journeyName}>{item.name}</Text>
          <Text style={styles.journeyDescription} numberOfLines={2}>
            {item.description || `Workflow: ${item.temporalWorkflow}`}
          </Text>
          <View style={styles.journeyTags}>
            <View style={[styles.categoryBadge, { backgroundColor: config.bgColor }]}>
              <Text style={[styles.categoryBadgeText, { color: config.color }]}>
                {item.category}
              </Text>
            </View>
            {item.middlewareHooks.slice(0, 2).map(hook => (
              <View key={hook} style={styles.middlewareBadge}>
                <Text style={styles.middlewareBadgeText}>{hook}</Text>
              </View>
            ))}
          </View>
        </View>
        <View style={styles.journeyArrow}>
          <Text style={styles.arrowText}>{'>'}</Text>
        </View>
      </TouchableOpacity>
    );
  };

  if (isLoading) {
    return (
      <SafeAreaView style={styles.container}>
        <View style={styles.loadingContainer}>
          <ActivityIndicator size="large" color="#2563eb" />
          <Text style={styles.loadingText}>Loading journeys...</Text>
        </View>
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView style={styles.container}>
      {/* Header */}
      <View style={styles.header}>
        <Text style={styles.headerTitle}>Journeys</Text>
        <View style={styles.headerRight}>
          {!isOnline && (
            <View style={styles.offlineBadge}>
              <Text style={styles.offlineBadgeText}>Offline</Text>
            </View>
          )}
          {queuedJourneys.length > 0 && (
            <TouchableOpacity style={styles.queueBadge} onPress={handleSyncQueued}>
              <Text style={styles.queueBadgeText}>{queuedJourneys.length} queued</Text>
            </TouchableOpacity>
          )}
        </View>
      </View>

      {/* Stats */}
      <View style={styles.statsContainer}>
        <View style={styles.statCard}>
          <Text style={styles.statValue}>{contracts.length}</Text>
          <Text style={styles.statLabel}>Available</Text>
        </View>
        <View style={styles.statCard}>
          <Text style={[styles.statValue, { color: '#16a34a' }]}>0</Text>
          <Text style={styles.statLabel}>Running</Text>
        </View>
        <View style={styles.statCard}>
          <Text style={[styles.statValue, { color: '#ca8a04' }]}>{queuedJourneys.length}</Text>
          <Text style={styles.statLabel}>Queued</Text>
        </View>
      </View>

      {/* Category Filter */}
      {renderCategoryFilter()}

      {/* Journey List */}
      <FlatList
        data={filteredContracts}
        renderItem={renderJourneyItem}
        keyExtractor={item => item.journeyKey}
        contentContainerStyle={styles.listContent}
        refreshControl={
          <RefreshControl refreshing={isRefreshing} onRefresh={handleRefresh} />
        }
        ListEmptyComponent={
          <View style={styles.emptyContainer}>
            <Text style={styles.emptyText}>No journeys available</Text>
            <Text style={styles.emptySubtext}>
              {isOnline ? 'Pull to refresh' : 'Connect to load journeys'}
            </Text>
          </View>
        }
      />
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f9fafb',
  },
  loadingContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  loadingText: {
    marginTop: 12,
    fontSize: 16,
    color: '#6b7280',
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingHorizontal: 16,
    paddingVertical: 12,
    backgroundColor: '#fff',
    borderBottomWidth: 1,
    borderBottomColor: '#e5e7eb',
  },
  headerTitle: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#111827',
  },
  headerRight: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  offlineBadge: {
    backgroundColor: '#fee2e2',
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: 12,
  },
  offlineBadgeText: {
    fontSize: 12,
    color: '#dc2626',
    fontWeight: '500',
  },
  queueBadge: {
    backgroundColor: '#fef9c3',
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: 12,
  },
  queueBadgeText: {
    fontSize: 12,
    color: '#ca8a04',
    fontWeight: '500',
  },
  statsContainer: {
    flexDirection: 'row',
    paddingHorizontal: 16,
    paddingVertical: 12,
    gap: 12,
  },
  statCard: {
    flex: 1,
    backgroundColor: '#fff',
    padding: 12,
    borderRadius: 8,
    alignItems: 'center',
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.05,
    shadowRadius: 2,
    elevation: 1,
  },
  statValue: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#2563eb',
  },
  statLabel: {
    fontSize: 12,
    color: '#6b7280',
    marginTop: 2,
  },
  categoryFilter: {
    flexDirection: 'row',
    paddingHorizontal: 16,
    paddingVertical: 8,
    gap: 8,
    flexWrap: 'wrap',
  },
  categoryChip: {
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: 16,
    backgroundColor: '#f3f4f6',
  },
  categoryChipActive: {
    backgroundColor: '#2563eb',
  },
  categoryChipText: {
    fontSize: 13,
    color: '#374151',
    fontWeight: '500',
  },
  categoryChipTextActive: {
    color: '#fff',
  },
  listContent: {
    padding: 16,
    gap: 12,
  },
  journeyCard: {
    flexDirection: 'row',
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 12,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.05,
    shadowRadius: 2,
    elevation: 1,
    marginBottom: 12,
  },
  journeyIcon: {
    width: 48,
    height: 48,
    borderRadius: 24,
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 12,
  },
  journeyIconText: {
    fontSize: 20,
    fontWeight: 'bold',
  },
  journeyContent: {
    flex: 1,
  },
  journeyName: {
    fontSize: 16,
    fontWeight: '600',
    color: '#111827',
    marginBottom: 4,
  },
  journeyDescription: {
    fontSize: 13,
    color: '#6b7280',
    marginBottom: 8,
  },
  journeyTags: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 4,
  },
  categoryBadge: {
    paddingHorizontal: 8,
    paddingVertical: 2,
    borderRadius: 4,
  },
  categoryBadgeText: {
    fontSize: 11,
    fontWeight: '500',
  },
  middlewareBadge: {
    backgroundColor: '#f3f4f6',
    paddingHorizontal: 6,
    paddingVertical: 2,
    borderRadius: 4,
  },
  middlewareBadgeText: {
    fontSize: 10,
    color: '#6b7280',
  },
  journeyArrow: {
    justifyContent: 'center',
    paddingLeft: 8,
  },
  arrowText: {
    fontSize: 18,
    color: '#9ca3af',
  },
  emptyContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    paddingVertical: 48,
  },
  emptyText: {
    fontSize: 16,
    color: '#6b7280',
    marginBottom: 4,
  },
  emptySubtext: {
    fontSize: 14,
    color: '#9ca3af',
  },
});
