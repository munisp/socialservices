/**
 * Offline Sync Library for Social Protection Platform
 * Provides IndexedDB storage, sync queue management, and conflict resolution
 */

import { openDB, DBSchema, IDBPDatabase } from 'idb';

// Database schema
interface OfflineDB extends DBSchema {
  beneficiaries: {
    key: string;
    value: BeneficiaryRecord;
    indexes: { 'by-status': string; 'by-updated': number };
  };
  households: {
    key: string;
    value: HouseholdRecord;
    indexes: { 'by-status': string; 'by-updated': number };
  };
  syncQueue: {
    key: string;
    value: SyncQueueItem;
    indexes: { 'by-status': string; 'by-timestamp': number };
  };
  syncLog: {
    key: string;
    value: SyncLogEntry;
    indexes: { 'by-timestamp': number };
  };
  conflicts: {
    key: string;
    value: ConflictRecord;
    indexes: { 'by-status': string };
  };
}

export interface BeneficiaryRecord {
  id: string;
  firstName: string;
  lastName: string;
  dateOfBirth: string;
  gender: string;
  nationalId: string;
  phoneNumber?: string;
  email?: string;
  address?: Record<string, unknown>;
  location?: { lat: number; lng: number };
  householdId?: string;
  status: string;
  enrollmentDate?: string;
  biometricCaptured: boolean;
  documentsVerified: boolean;
  eligibilityScore?: number;
  proxyMeansScore?: number;
  version: number;
  syncStatus: 'synced' | 'pending' | 'conflict';
  localUpdatedAt: number;
  serverUpdatedAt?: number;
}

export interface HouseholdRecord {
  id: string;
  headId: string;
  name: string;
  address?: Record<string, unknown>;
  location?: { lat: number; lng: number };
  memberCount: number;
  incomeLevel?: string;
  housingType?: string;
  assets?: Record<string, unknown>;
  proxyMeansScore?: number;
  status: string;
  version: number;
  syncStatus: 'synced' | 'pending' | 'conflict';
  localUpdatedAt: number;
  serverUpdatedAt?: number;
}

export interface SyncQueueItem {
  id: string;
  entityType: 'beneficiary' | 'household' | 'transaction';
  entityId: string;
  operation: 'create' | 'update' | 'delete';
  data: Record<string, unknown>;
  checksum: string;
  clientVersion: number;
  serverVersion: number;
  status: 'pending' | 'syncing' | 'synced' | 'failed' | 'conflict';
  retryCount: number;
  errorMessage?: string;
  createdAt: number;
  syncedAt?: number;
}

export interface SyncLogEntry {
  id: string;
  action: 'sync_started' | 'sync_completed' | 'sync_failed' | 'conflict_detected' | 'conflict_resolved';
  details: Record<string, unknown>;
  timestamp: number;
}

export interface ConflictRecord {
  id: string;
  entityType: string;
  entityId: string;
  clientData: Record<string, unknown>;
  serverData: Record<string, unknown>;
  resolution?: 'client_wins' | 'server_wins' | 'merge' | 'manual';
  resolvedData?: Record<string, unknown>;
  status: 'pending' | 'resolved';
  createdAt: number;
  resolvedAt?: number;
}

export type ConflictResolution = 'client_wins' | 'server_wins' | 'merge' | 'manual';

export interface SyncResult {
  success: boolean;
  synced: number;
  failed: number;
  conflicts: number;
  errors: string[];
}

function asRecord(value: object): Record<string, unknown> {
  return { ...value } as Record<string, unknown>;
}

class OfflineSyncManager {
  private db: IDBPDatabase<OfflineDB> | null = null;
  private syncInProgress = false;
  private listeners: Map<string, Set<(data: unknown) => void>> = new Map();
  private deviceId: string;

  constructor() {
    this.deviceId = this.getOrCreateDeviceId();
  }

  private getOrCreateDeviceId(): string {
    let deviceId = localStorage.getItem('sp-device-id');
    if (!deviceId) {
      deviceId = `device-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
      localStorage.setItem('sp-device-id', deviceId);
    }
    return deviceId;
  }

  async init(): Promise<void> {
    this.db = await openDB<OfflineDB>('sp-offline-db', 1, {
      upgrade(db) {
        // Beneficiaries store
        const beneficiariesStore = db.createObjectStore('beneficiaries', { keyPath: 'id' });
        beneficiariesStore.createIndex('by-status', 'syncStatus');
        beneficiariesStore.createIndex('by-updated', 'localUpdatedAt');

        // Households store
        const householdsStore = db.createObjectStore('households', { keyPath: 'id' });
        householdsStore.createIndex('by-status', 'syncStatus');
        householdsStore.createIndex('by-updated', 'localUpdatedAt');

        // Sync queue store
        const syncQueueStore = db.createObjectStore('syncQueue', { keyPath: 'id' });
        syncQueueStore.createIndex('by-status', 'status');
        syncQueueStore.createIndex('by-timestamp', 'createdAt');

        // Sync log store
        const syncLogStore = db.createObjectStore('syncLog', { keyPath: 'id' });
        syncLogStore.createIndex('by-timestamp', 'timestamp');

        // Conflicts store
        const conflictsStore = db.createObjectStore('conflicts', { keyPath: 'id' });
        conflictsStore.createIndex('by-status', 'status');
      },
    });

    // Listen for online/offline events
    window.addEventListener('online', () => this.onOnline());
    window.addEventListener('offline', () => this.onOffline());

    // Listen for service worker messages
    if ('serviceWorker' in navigator) {
      navigator.serviceWorker.addEventListener('message', (event) => {
        this.handleServiceWorkerMessage(event.data);
      });
    }
  }

  private onOnline(): void {
    console.log('[OfflineSync] Online - triggering sync');
    this.emit('online', { timestamp: Date.now() });
    this.sync();
  }

  private onOffline(): void {
    console.log('[OfflineSync] Offline');
    this.emit('offline', { timestamp: Date.now() });
  }

  private handleServiceWorkerMessage(data: { type: string; data?: unknown }): void {
    switch (data.type) {
      case 'OFFLINE_REQUEST_QUEUED':
        this.emit('requestQueued', data.data);
        // Register background sync when items are queued
        this.registerBackgroundSync('offline-sync');
        break;
      case 'OFFLINE_SYNC_COMPLETE':
        this.emit('syncComplete', data.data);
        break;
      case 'AUTH_REQUIRED_FOR_SYNC':
        this.emit('authRequired', data.data);
        break;
    }
  }

  // Register background sync with the service worker
  private async registerBackgroundSync(tag: string): Promise<boolean> {
    if (!('serviceWorker' in navigator) || !('SyncManager' in window)) {
      console.log('[OfflineSync] Background Sync not supported');
      return false;
    }

    try {
      const registration = await navigator.serviceWorker.ready;
      // @ts-expect-error - SyncManager types may not be available
      await registration.sync.register(tag);
      console.log(`[OfflineSync] Background sync registered: ${tag}`);
      return true;
    } catch (error) {
      console.error('[OfflineSync] Failed to register background sync:', error);
      return false;
    }
  }

  // Event emitter methods
  on(event: string, callback: (data: unknown) => void): void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event)!.add(callback);
  }

  off(event: string, callback: (data: unknown) => void): void {
    this.listeners.get(event)?.delete(callback);
  }

  private emit(event: string, data: unknown): void {
    this.listeners.get(event)?.forEach((callback) => callback(data));
  }

  // Beneficiary operations
  async saveBeneficiary(beneficiary: Omit<BeneficiaryRecord, 'syncStatus' | 'localUpdatedAt'>): Promise<void> {
    if (!this.db) throw new Error('Database not initialized');

    const record: BeneficiaryRecord = {
      ...beneficiary,
      syncStatus: 'pending',
      localUpdatedAt: Date.now(),
    };

    await this.db.put('beneficiaries', record);
    await this.addToSyncQueue('beneficiary', beneficiary.id, 'update', asRecord(record));
    this.emit('beneficiaryUpdated', record);
  }

  async getBeneficiary(id: string): Promise<BeneficiaryRecord | undefined> {
    if (!this.db) throw new Error('Database not initialized');
    return this.db.get('beneficiaries', id);
  }

  async getAllBeneficiaries(): Promise<BeneficiaryRecord[]> {
    if (!this.db) throw new Error('Database not initialized');
    return this.db.getAll('beneficiaries');
  }

  async searchBeneficiaries(query: string): Promise<BeneficiaryRecord[]> {
    const all = await this.getAllBeneficiaries();
    const lowerQuery = query.toLowerCase();
    return all.filter(
      (b) =>
        b.firstName.toLowerCase().includes(lowerQuery) ||
        b.lastName.toLowerCase().includes(lowerQuery) ||
        b.nationalId.toLowerCase().includes(lowerQuery)
    );
  }

  async deleteBeneficiary(id: string): Promise<void> {
    if (!this.db) throw new Error('Database not initialized');
    
    const beneficiary = await this.getBeneficiary(id);
    if (beneficiary) {
      await this.addToSyncQueue('beneficiary', id, 'delete', asRecord(beneficiary));
      await this.db.delete('beneficiaries', id);
      this.emit('beneficiaryDeleted', { id });
    }
  }

  // Household operations
  async saveHousehold(household: Omit<HouseholdRecord, 'syncStatus' | 'localUpdatedAt'>): Promise<void> {
    if (!this.db) throw new Error('Database not initialized');

    const record: HouseholdRecord = {
      ...household,
      syncStatus: 'pending',
      localUpdatedAt: Date.now(),
    };

    await this.db.put('households', record);
    await this.addToSyncQueue('household', household.id, 'update', asRecord(record));
    this.emit('householdUpdated', record);
  }

  async getHousehold(id: string): Promise<HouseholdRecord | undefined> {
    if (!this.db) throw new Error('Database not initialized');
    return this.db.get('households', id);
  }

  async getAllHouseholds(): Promise<HouseholdRecord[]> {
    if (!this.db) throw new Error('Database not initialized');
    return this.db.getAll('households');
  }

  // Sync queue operations
  private async addToSyncQueue(
    entityType: 'beneficiary' | 'household' | 'transaction',
    entityId: string,
    operation: 'create' | 'update' | 'delete',
    data: Record<string, unknown>
  ): Promise<void> {
    if (!this.db) throw new Error('Database not initialized');

    const item: SyncQueueItem = {
      id: `${entityType}-${entityId}-${Date.now()}`,
      entityType,
      entityId,
      operation,
      data,
      checksum: this.generateChecksum(data),
      clientVersion: (data.version as number) || 1,
      serverVersion: (data.serverVersion as number) || 0,
      status: 'pending',
      retryCount: 0,
      createdAt: Date.now(),
    };

    await this.db.put('syncQueue', item);
  }

  async getSyncQueue(): Promise<SyncQueueItem[]> {
    if (!this.db) throw new Error('Database not initialized');
    return this.db.getAllFromIndex('syncQueue', 'by-status', 'pending');
  }

  async getSyncQueueCount(): Promise<number> {
    const queue = await this.getSyncQueue();
    return queue.length;
  }

  // Conflict operations
  async getConflicts(): Promise<ConflictRecord[]> {
    if (!this.db) throw new Error('Database not initialized');
    return this.db.getAllFromIndex('conflicts', 'by-status', 'pending');
  }

  async resolveConflict(
    conflictId: string,
    resolution: ConflictResolution,
    resolvedData?: Record<string, unknown>
  ): Promise<void> {
    if (!this.db) throw new Error('Database not initialized');

    const conflict = await this.db.get('conflicts', conflictId);
    if (!conflict) throw new Error('Conflict not found');

    let finalData: Record<string, unknown>;
    switch (resolution) {
      case 'client_wins':
        finalData = conflict.clientData;
        break;
      case 'server_wins':
        finalData = conflict.serverData;
        break;
      case 'merge':
      case 'manual':
        if (!resolvedData) throw new Error('Resolved data required for merge/manual resolution');
        finalData = resolvedData;
        break;
    }

    // Update the conflict record
    conflict.resolution = resolution;
    conflict.resolvedData = finalData;
    conflict.status = 'resolved';
    conflict.resolvedAt = Date.now();
    await this.db.put('conflicts', conflict);

    // Update the entity with resolved data
    if (conflict.entityType === 'beneficiary') {
      await this.db.put('beneficiaries', finalData as unknown as BeneficiaryRecord);
    } else if (conflict.entityType === 'household') {
      await this.db.put('households', finalData as unknown as HouseholdRecord);
    }

    // Add to sync queue to push resolution to server
    await this.addToSyncQueue(
      conflict.entityType as 'beneficiary' | 'household',
      conflict.entityId,
      'update',
      finalData
    );

    await this.logSync('conflict_resolved', { conflictId, resolution });
    this.emit('conflictResolved', { conflictId, resolution, data: finalData });
  }

  // Sync operations
  async sync(): Promise<SyncResult> {
    if (this.syncInProgress) {
      return { success: false, synced: 0, failed: 0, conflicts: 0, errors: ['Sync already in progress'] };
    }

    if (!navigator.onLine) {
      return { success: false, synced: 0, failed: 0, conflicts: 0, errors: ['Device is offline'] };
    }

    this.syncInProgress = true;
    this.emit('syncStarted', { timestamp: Date.now() });
    await this.logSync('sync_started', {});

    const result: SyncResult = {
      success: true,
      synced: 0,
      failed: 0,
      conflicts: 0,
      errors: [],
    };

    try {
      const queue = await this.getSyncQueue();

      for (const item of queue) {
        try {
          await this.syncItem(item);
          result.synced++;
        } catch (error) {
          const errorMessage = error instanceof Error ? error.message : 'Unknown error';
          
          if (errorMessage.includes('conflict')) {
            result.conflicts++;
          } else {
            result.failed++;
            result.errors.push(`${item.entityType}/${item.entityId}: ${errorMessage}`);
          }
        }
      }

      // Pull server changes
      await this.pullServerChanges();

      await this.logSync('sync_completed', asRecord(result));
      this.emit('syncCompleted', result);
    } catch (error) {
      result.success = false;
      result.errors.push(error instanceof Error ? error.message : 'Unknown error');
      await this.logSync('sync_failed', { error: result.errors });
      this.emit('syncFailed', result);
    } finally {
      this.syncInProgress = false;
    }

    return result;
  }

  private async syncItem(item: SyncQueueItem): Promise<void> {
    if (!this.db) throw new Error('Database not initialized');

    item.status = 'syncing';
    await this.db.put('syncQueue', item);

    try {
      const response = await fetch('/api/offline/sync', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          deviceId: this.deviceId,
          records: [{
            id: item.id,
            entityType: item.entityType,
            entityId: item.entityId,
            operation: item.operation,
            data: item.data,
            checksum: item.checksum,
            clientVersion: item.clientVersion,
            serverVersion: item.serverVersion,
          }],
        }),
      });

      if (!response.ok) {
        throw new Error(`Sync failed: ${response.status}`);
      }

      const result = await response.json();

      if (result.conflicts && result.conflicts.length > 0) {
        // Handle conflict
        const conflict = result.conflicts[0];
        await this.createConflict(item, conflict.serverData);
        throw new Error('conflict');
      }

      // Success - remove from queue and update entity
      item.status = 'synced';
      item.syncedAt = Date.now();
      await this.db.put('syncQueue', item);

      // Update entity sync status
      if (item.entityType === 'beneficiary') {
        const beneficiary = await this.getBeneficiary(item.entityId);
        if (beneficiary) {
          beneficiary.syncStatus = 'synced';
          beneficiary.serverUpdatedAt = Date.now();
          await this.db.put('beneficiaries', beneficiary);
        }
      } else if (item.entityType === 'household') {
        const household = await this.getHousehold(item.entityId);
        if (household) {
          household.syncStatus = 'synced';
          household.serverUpdatedAt = Date.now();
          await this.db.put('households', household);
        }
      }
    } catch (error) {
      item.status = 'failed';
      item.retryCount++;
      item.errorMessage = error instanceof Error ? error.message : 'Unknown error';
      await this.db.put('syncQueue', item);
      throw error;
    }
  }

  private async createConflict(item: SyncQueueItem, serverData: Record<string, unknown>): Promise<void> {
    if (!this.db) throw new Error('Database not initialized');

    const conflict: ConflictRecord = {
      id: `conflict-${item.entityId}-${Date.now()}`,
      entityType: item.entityType,
      entityId: item.entityId,
      clientData: item.data,
      serverData,
      status: 'pending',
      createdAt: Date.now(),
    };

    await this.db.put('conflicts', conflict);
    await this.logSync('conflict_detected', { entityType: item.entityType, entityId: item.entityId });
    this.emit('conflictDetected', conflict);
  }

  private async pullServerChanges(): Promise<void> {
    if (!this.db) throw new Error('Database not initialized');

    const lastSync = localStorage.getItem('sp-last-sync');
    const since = lastSync ? new Date(lastSync).toISOString() : new Date(0).toISOString();

    try {
      const response = await fetch(`/api/offline/changes?since=${since}&deviceId=${this.deviceId}`);
      
      if (!response.ok) {
        throw new Error(`Failed to pull changes: ${response.status}`);
      }

      const changes = await response.json();

      for (const change of changes.beneficiaries || []) {
        const existing = await this.getBeneficiary(change.id);
        if (!existing || existing.syncStatus === 'synced') {
          await this.db.put('beneficiaries', {
            ...change,
            syncStatus: 'synced',
            localUpdatedAt: Date.now(),
            serverUpdatedAt: Date.now(),
          });
        }
      }

      for (const change of changes.households || []) {
        const existing = await this.getHousehold(change.id);
        if (!existing || existing.syncStatus === 'synced') {
          await this.db.put('households', {
            ...change,
            syncStatus: 'synced',
            localUpdatedAt: Date.now(),
            serverUpdatedAt: Date.now(),
          });
        }
      }

      localStorage.setItem('sp-last-sync', new Date().toISOString());
    } catch (error) {
      console.error('[OfflineSync] Failed to pull server changes:', error);
    }
  }

  // Utility methods
  private generateChecksum(data: Record<string, unknown>): string {
    const str = JSON.stringify(data);
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
      const char = str.charCodeAt(i);
      hash = ((hash << 5) - hash) + char;
      hash = hash & hash;
    }
    return hash.toString(16);
  }

  private async logSync(action: SyncLogEntry['action'], details: Record<string, unknown>): Promise<void> {
    if (!this.db) return;

    const entry: SyncLogEntry = {
      id: `log-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
      action,
      details,
      timestamp: Date.now(),
    };

    await this.db.put('syncLog', entry);
  }

  async getSyncLog(limit = 100): Promise<SyncLogEntry[]> {
    if (!this.db) throw new Error('Database not initialized');
    const all = await this.db.getAllFromIndex('syncLog', 'by-timestamp');
    return all.slice(-limit).reverse();
  }

  async clearSyncQueue(): Promise<void> {
    if (!this.db) throw new Error('Database not initialized');
    const tx = this.db.transaction('syncQueue', 'readwrite');
    await tx.store.clear();
  }

  async getStats(): Promise<{
    beneficiaries: { total: number; pending: number; synced: number; conflict: number };
    households: { total: number; pending: number; synced: number; conflict: number };
    queueSize: number;
    conflicts: number;
    lastSync: string | null;
  }> {
    if (!this.db) throw new Error('Database not initialized');

    const beneficiaries = await this.getAllBeneficiaries();
    const households = await this.getAllHouseholds();
    const queue = await this.getSyncQueue();
    const conflicts = await this.getConflicts();

    return {
      beneficiaries: {
        total: beneficiaries.length,
        pending: beneficiaries.filter((b) => b.syncStatus === 'pending').length,
        synced: beneficiaries.filter((b) => b.syncStatus === 'synced').length,
        conflict: beneficiaries.filter((b) => b.syncStatus === 'conflict').length,
      },
      households: {
        total: households.length,
        pending: households.filter((h) => h.syncStatus === 'pending').length,
        synced: households.filter((h) => h.syncStatus === 'synced').length,
        conflict: households.filter((h) => h.syncStatus === 'conflict').length,
      },
      queueSize: queue.length,
      conflicts: conflicts.length,
      lastSync: localStorage.getItem('sp-last-sync'),
    };
  }

  isOnline(): boolean {
    return navigator.onLine;
  }

  isSyncing(): boolean {
    return this.syncInProgress;
  }
}

// Singleton instance
export const offlineSync = new OfflineSyncManager();

// React hook for offline sync
export function useOfflineSync() {
  return offlineSync;
}
