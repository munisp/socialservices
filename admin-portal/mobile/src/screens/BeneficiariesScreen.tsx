/**
 * Beneficiaries Screen - List and search beneficiaries
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
import { getAllBeneficiaries, searchBeneficiaries, BeneficiaryRecord } from '../services/database';

function BeneficiaryCard({ beneficiary, onPress }: { beneficiary: BeneficiaryRecord; onPress: () => void }) {
  const getSyncStatusColor = () => {
    switch (beneficiary.syncStatus) {
      case 'synced': return '#4CAF50';
      case 'pending': return '#FF9800';
      case 'conflict': return '#f44336';
      default: return '#999';
    }
  };

  return (
    <TouchableOpacity style={styles.card} onPress={onPress}>
      <View style={styles.cardHeader}>
        <View style={styles.avatar}>
          <Text style={styles.avatarText}>
            {beneficiary.firstName[0]}{beneficiary.lastName[0]}
          </Text>
        </View>
        <View style={styles.cardInfo}>
          <Text style={styles.cardName}>
            {beneficiary.firstName} {beneficiary.lastName}
          </Text>
          <Text style={styles.cardId}>ID: {beneficiary.nationalId || 'Not provided'}</Text>
        </View>
        <View style={[styles.syncIndicator, { backgroundColor: getSyncStatusColor() }]} />
      </View>
      <View style={styles.cardDetails}>
        <View style={styles.detailItem}>
          <Icon name="calendar" size={14} color="#666" />
          <Text style={styles.detailText}>{beneficiary.dateOfBirth || 'N/A'}</Text>
        </View>
        <View style={styles.detailItem}>
          <Icon name="phone" size={14} color="#666" />
          <Text style={styles.detailText}>{beneficiary.phoneNumber || 'N/A'}</Text>
        </View>
        <View style={[styles.statusBadge, { backgroundColor: beneficiary.status === 'active' ? '#E8F5E9' : '#FFF3E0' }]}>
          <Text style={[styles.statusText, { color: beneficiary.status === 'active' ? '#4CAF50' : '#FF9800' }]}>
            {beneficiary.status}
          </Text>
        </View>
      </View>
    </TouchableOpacity>
  );
}

export function BeneficiariesScreen() {
  const navigation = useNavigation<any>();
  const { beneficiaries, setBeneficiaries } = useAppStore();
  const [searchQuery, setSearchQuery] = useState('');
  const [filteredBeneficiaries, setFilteredBeneficiaries] = useState<BeneficiaryRecord[]>([]);
  const [refreshing, setRefreshing] = useState(false);

  const loadBeneficiaries = useCallback(async () => {
    const data = await getAllBeneficiaries();
    setBeneficiaries(data);
    setFilteredBeneficiaries(data);
  }, [setBeneficiaries]);

  useEffect(() => {
    loadBeneficiaries();
  }, [loadBeneficiaries]);

  useEffect(() => {
    if (searchQuery.trim()) {
      searchBeneficiaries(searchQuery).then(setFilteredBeneficiaries);
    } else {
      setFilteredBeneficiaries(beneficiaries);
    }
  }, [searchQuery, beneficiaries]);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await loadBeneficiaries();
    setRefreshing(false);
  }, [loadBeneficiaries]);

  const renderItem = ({ item }: { item: BeneficiaryRecord }) => (
    <BeneficiaryCard
      beneficiary={item}
      onPress={() => navigation.navigate('BeneficiaryDetail', { id: item.id })}
    />
  );

  return (
    <View style={styles.container}>
      {/* Search Bar */}
      <View style={styles.searchContainer}>
        <Icon name="magnify" size={20} color="#666" style={styles.searchIcon} />
        <TextInput
          style={styles.searchInput}
          placeholder="Search by name or ID..."
          value={searchQuery}
          onChangeText={setSearchQuery}
        />
        {searchQuery.length > 0 && (
          <TouchableOpacity onPress={() => setSearchQuery('')}>
            <Icon name="close-circle" size={20} color="#666" />
          </TouchableOpacity>
        )}
      </View>

      {/* Stats */}
      <View style={styles.statsBar}>
        <Text style={styles.statsText}>
          {filteredBeneficiaries.length} beneficiaries
        </Text>
        <View style={styles.statsRight}>
          <View style={styles.statItem}>
            <View style={[styles.statDot, { backgroundColor: '#4CAF50' }]} />
            <Text style={styles.statLabel}>
              {beneficiaries.filter(b => b.syncStatus === 'synced').length} synced
            </Text>
          </View>
          <View style={styles.statItem}>
            <View style={[styles.statDot, { backgroundColor: '#FF9800' }]} />
            <Text style={styles.statLabel}>
              {beneficiaries.filter(b => b.syncStatus === 'pending').length} pending
            </Text>
          </View>
        </View>
      </View>

      {/* List */}
      <FlatList
        data={filteredBeneficiaries}
        renderItem={renderItem}
        keyExtractor={(item) => item.id}
        contentContainerStyle={styles.listContent}
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
        }
        ListEmptyComponent={
          <View style={styles.emptyContainer}>
            <Icon name="account-search" size={64} color="#ccc" />
            <Text style={styles.emptyText}>
              {searchQuery ? 'No beneficiaries found' : 'No beneficiaries enrolled yet'}
            </Text>
            {!searchQuery && (
              <TouchableOpacity
                style={styles.emptyButton}
                onPress={() => navigation.navigate('EnrollBeneficiary')}
              >
                <Text style={styles.emptyButtonText}>Enroll First Beneficiary</Text>
              </TouchableOpacity>
            )}
          </View>
        }
      />

      {/* FAB */}
      <TouchableOpacity
        style={styles.fab}
        onPress={() => navigation.navigate('EnrollBeneficiary')}
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
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingHorizontal: 16,
    paddingBottom: 8,
  },
  statsText: {
    fontSize: 14,
    color: '#666',
  },
  statsRight: {
    flexDirection: 'row',
    gap: 12,
  },
  statItem: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
  },
  statDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
  },
  statLabel: {
    fontSize: 12,
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
  avatar: {
    width: 48,
    height: 48,
    borderRadius: 24,
    backgroundColor: '#2196F3',
    justifyContent: 'center',
    alignItems: 'center',
  },
  avatarText: {
    color: '#fff',
    fontSize: 18,
    fontWeight: '600',
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
  statusBadge: {
    marginLeft: 'auto',
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: 4,
  },
  statusText: {
    fontSize: 12,
    fontWeight: '500',
    textTransform: 'capitalize',
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
  emptyButton: {
    marginTop: 16,
    backgroundColor: '#2196F3',
    paddingHorizontal: 24,
    paddingVertical: 12,
    borderRadius: 8,
  },
  emptyButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '500',
  },
  fab: {
    position: 'absolute',
    right: 16,
    bottom: 16,
    width: 56,
    height: 56,
    borderRadius: 28,
    backgroundColor: '#2196F3',
    justifyContent: 'center',
    alignItems: 'center',
    elevation: 4,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.25,
    shadowRadius: 4,
  },
});

export default BeneficiariesScreen;
