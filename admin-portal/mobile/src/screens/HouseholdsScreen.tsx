/**
 * Households Screen - List and manage households
 */

import React, { useState, useCallback, useEffect } from 'react';
import {
  View,
  Text,
  StyleSheet,
  FlatList,
  TextInput,
  TouchableOpacity,
  RefreshControl,
} from 'react-native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import { useNavigation } from '@react-navigation/native';
import { useAppStore } from '../store/appStore';
import { getAllHouseholds, HouseholdRecord } from '../services/database';

function HouseholdCard({ household, onPress }: { household: HouseholdRecord; onPress: () => void }) {
  const getSyncStatusColor = () => {
    switch (household.syncStatus) {
      case 'synced': return '#4CAF50';
      case 'pending': return '#FF9800';
      case 'conflict': return '#f44336';
      default: return '#999';
    }
  };

  return (
    <TouchableOpacity style={styles.card} onPress={onPress}>
      <View style={styles.cardHeader}>
        <View style={styles.iconContainer}>
          <Icon name="home-group" size={24} color="#9C27B0" />
        </View>
        <View style={styles.cardInfo}>
          <Text style={styles.cardName}>{household.name}</Text>
          <Text style={styles.cardId}>ID: {household.id.slice(0, 12)}...</Text>
        </View>
        <View style={[styles.syncIndicator, { backgroundColor: getSyncStatusColor() }]} />
      </View>
      <View style={styles.cardDetails}>
        <View style={styles.detailItem}>
          <Icon name="account-multiple" size={14} color="#666" />
          <Text style={styles.detailText}>{household.memberCount} members</Text>
        </View>
        {household.housingType && (
          <View style={styles.detailItem}>
            <Icon name="home" size={14} color="#666" />
            <Text style={styles.detailText}>{household.housingType}</Text>
          </View>
        )}
        {household.proxyMeansScore !== undefined && (
          <View style={styles.pmtBadge}>
            <Text style={styles.pmtText}>PMT: {household.proxyMeansScore.toFixed(1)}</Text>
          </View>
        )}
      </View>
    </TouchableOpacity>
  );
}

export function HouseholdsScreen() {
  const navigation = useNavigation<any>();
  const { households, setHouseholds } = useAppStore();
  const [searchQuery, setSearchQuery] = useState('');
  const [filteredHouseholds, setFilteredHouseholds] = useState<HouseholdRecord[]>([]);
  const [refreshing, setRefreshing] = useState(false);

  const loadHouseholds = useCallback(async () => {
    const data = await getAllHouseholds();
    setHouseholds(data);
    setFilteredHouseholds(data);
  }, [setHouseholds]);

  useEffect(() => {
    loadHouseholds();
  }, [loadHouseholds]);

  useEffect(() => {
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      setFilteredHouseholds(
        households.filter(h => 
          h.name.toLowerCase().includes(query) ||
          h.id.toLowerCase().includes(query)
        )
      );
    } else {
      setFilteredHouseholds(households);
    }
  }, [searchQuery, households]);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await loadHouseholds();
    setRefreshing(false);
  }, [loadHouseholds]);

  const renderItem = ({ item }: { item: HouseholdRecord }) => (
    <HouseholdCard
      household={item}
      onPress={() => navigation.navigate('HouseholdDetail', { id: item.id })}
    />
  );

  return (
    <View style={styles.container}>
      <View style={styles.searchContainer}>
        <Icon name="magnify" size={20} color="#666" style={styles.searchIcon} />
        <TextInput
          style={styles.searchInput}
          placeholder="Search households..."
          value={searchQuery}
          onChangeText={setSearchQuery}
        />
        {searchQuery.length > 0 && (
          <TouchableOpacity onPress={() => setSearchQuery('')}>
            <Icon name="close-circle" size={20} color="#666" />
          </TouchableOpacity>
        )}
      </View>

      <View style={styles.statsBar}>
        <Text style={styles.statsText}>
          {filteredHouseholds.length} households
        </Text>
      </View>

      <FlatList
        data={filteredHouseholds}
        renderItem={renderItem}
        keyExtractor={(item) => item.id}
        contentContainerStyle={styles.listContent}
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
        }
        ListEmptyComponent={
          <View style={styles.emptyContainer}>
            <Icon name="home-search" size={64} color="#ccc" />
            <Text style={styles.emptyText}>
              {searchQuery ? 'No households found' : 'No households registered yet'}
            </Text>
          </View>
        }
      />

      <TouchableOpacity
        style={styles.fab}
        onPress={() => navigation.navigate('EnrollHousehold')}
      >
        <Icon name="plus" size={24} color="#fff" />
      </TouchableOpacity>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  searchContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#fff',
    margin: 16,
    paddingHorizontal: 12,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: '#e0e0e0',
  },
  searchIcon: {
    marginRight: 8,
  },
  searchInput: {
    flex: 1,
    paddingVertical: 12,
    fontSize: 16,
  },
  statsBar: {
    paddingHorizontal: 16,
    paddingBottom: 8,
  },
  statsText: {
    fontSize: 14,
    color: '#666',
  },
  listContent: {
    paddingHorizontal: 16,
    paddingBottom: 80,
  },
  card: {
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 16,
    marginBottom: 12,
    elevation: 2,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.1,
    shadowRadius: 2,
  },
  cardHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 12,
  },
  iconContainer: {
    width: 48,
    height: 48,
    borderRadius: 24,
    backgroundColor: '#F3E5F5',
    justifyContent: 'center',
    alignItems: 'center',
  },
  cardInfo: {
    flex: 1,
    marginLeft: 12,
  },
  cardName: {
    fontSize: 16,
    fontWeight: '600',
    color: '#333',
  },
  cardId: {
    fontSize: 14,
    color: '#666',
    marginTop: 2,
  },
  syncIndicator: {
    width: 12,
    height: 12,
    borderRadius: 6,
  },
  cardDetails: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 16,
  },
  detailItem: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
  },
  detailText: {
    fontSize: 12,
    color: '#666',
  },
  pmtBadge: {
    marginLeft: 'auto',
    backgroundColor: '#E8F5E9',
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: 4,
  },
  pmtText: {
    fontSize: 12,
    fontWeight: '500',
    color: '#4CAF50',
  },
  emptyContainer: {
    alignItems: 'center',
    paddingTop: 60,
  },
  emptyText: {
    fontSize: 16,
    color: '#999',
    marginTop: 16,
  },
  fab: {
    position: 'absolute',
    right: 16,
    bottom: 16,
    width: 56,
    height: 56,
    borderRadius: 28,
    backgroundColor: '#9C27B0',
    justifyContent: 'center',
    alignItems: 'center',
    elevation: 4,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.25,
    shadowRadius: 4,
  },
});

export default HouseholdsScreen;
