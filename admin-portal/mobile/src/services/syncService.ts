/**
 * Sync Service for Mobile App
 * Handles synchronization between local SQLite and remote server
 */

import NetInfo, { NetInfoState } from '@react-native-community/netinfo';
import {
  getSyncQueue,
  updateSyncQueueItem,
  clearSyncedItems,
  getSyncStats,
  getSetting,
  setSetting,
  saveBeneficiary,
  saveHousehold,
  saveConflict,
  getConflictCount,
} from './database';

// API_BASE_URL must be configured via app config or environment
// In production, this should be set via react-native-config or similar
const API_BASE_URL = process.env.API_BASE_URL || (() => {
  if (__DEV__) {
    return 'http://localhost:8090';
  }
  throw new Error('API_BASE_URL must be configured for production builds');
})();

export interface SyncResult {
  success: boolean;
  synced: number;
  failed: number;
  conflicts: number;
  errors: string[];
}

export interface SyncStatus {
  isOnline: boolean;
  isSyncing: boolean;
  lastSync: string | null;
  pendingChanges: number;
  conflicts: number;
}

type SyncEventCallback = (event: string, data: unknown) => void;

class SyncService {
  private isOnline: boolean = false;
  private isSyncing: boolean = false;
  private deviceId: string | null = null;
  private authToken: string | null = null;
  private listeners: Set<SyncEventCallback> = new Set();
  private syncInterval: NodeJS.Timeout | null = null;

  async init(): Promise<void> {
    // Get or create device ID
    this.deviceId = await getSetting('deviceId');
    if (!this.deviceId) {
      this.deviceId = `mobile-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
      await setSetting('deviceId', this.deviceId);
    }

    // Get auth token
    this.authToken = await getSetting('authToken');

    // Subscribe to network state changes
    NetInfo.addEventListener((state: NetInfoState) => {
      const wasOnline = this.isOnline;
      this.isOnline = state.isConnected ?? false;

      if (!wasOnline && this.isOnline) {
        this.emit('online', { timestamp: Date.now() });
        this.sync();
      } else if (wasOnline && !this.isOnline) {
        this.emit('offline', { timestamp: Date.now() });
      }
    });

    // Check initial network state
    const state = await NetInfo.fetch();
    this.isOnline = state.isConnected ?? false;

    // Start periodic sync
    this.startPeriodicSync();
  }

  setAuthToken(token: string): void {
    this.authToken = token;
    setSetting('authToken', token);
  }

  clearAuthToken(): void {
    this.authToken = null;
    setSetting('authToken', '');
  }

  subscribe(callback: SyncEventCallback): () => void {
    this.listeners.add(callback);
    return () => this.listeners.delete(callback);
  }

  private emit(event: string, data: unknown): void {
    this.listeners.forEach((callback) => callback(event, data));
  }

  private startPeriodicSync(): void {
    // Sync every 5 minutes when online
    this.syncInterval = setInterval(() => {
      if (this.isOnline && !this.isSyncing) {
        this.sync();
      }
    }, 5 * 60 * 1000);
  }

  stopPeriodicSync(): void {
    if (this.syncInterval) {
      clearInterval(this.syncInterval);
      this.syncInterval = null;
    }
  }

  async getStatus(): Promise<SyncStatus> {
    const stats = await getSyncStats();
    const lastSync = await getSetting('lastSync');
    const conflictCount = await getConflictCount();

    return {
      isOnline: this.isOnline,
      isSyncing: this.isSyncing,
      lastSync,
      pendingChanges: stats.queueSize,
      conflicts: conflictCount,
    };
  }

  async sync(): Promise<SyncResult> {
    if (this.isSyncing) {
      return {
        success: false,
        synced: 0,
        failed: 0,
        conflicts: 0,
        errors: ['Sync already in progress'],
      };
    }

    if (!this.isOnline) {
      return {
        success: false,
        synced: 0,
        failed: 0,
        conflicts: 0,
        errors: ['Device is offline'],
      };
    }

    this.isSyncing = true;
    this.emit('syncStarted', { timestamp: Date.now() });

    const result: SyncResult = {
      success: true,
      synced: 0,
      failed: 0,
      conflicts: 0,
      errors: [],
    };

    try {
      // Push local changes
      const pushResult = await this.pushChanges();
      result.synced += pushResult.synced;
      result.failed += pushResult.failed;
      result.conflicts += pushResult.conflicts;
      result.errors.push(...pushResult.errors);

      // Pull server changes
      const pullResult = await this.pullChanges();
      result.synced += pullResult.synced;
      result.errors.push(...pullResult.errors);

      // Clean up synced items
      await clearSyncedItems();

      // Update last sync time
      await setSetting('lastSync', new Date().toISOString());

      this.emit('syncCompleted', result);
    } catch (error) {
      result.success = false;
      result.errors.push(error instanceof Error ? error.message : 'Unknown error');
      this.emit('syncFailed', result);
    } finally {
      this.isSyncing = false;
    }

    return result;
  }

  private async pushChanges(): Promise<{ synced: number; failed: number; conflicts: number; errors: string[] }> {
    const queue = await getSyncQueue();
    let synced = 0;
    let failed = 0;
    let conflicts = 0;
    const errors: string[] = [];

    for (const item of queue) {
      try {
        const response = await fetch(`${API_BASE_URL}/api/offline/sync`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${this.authToken}`,
            'X-Device-ID': this.deviceId || '',
          },
          body: JSON.stringify({
            deviceId: this.deviceId,
            records: [{
              id: item.id,
              entityType: item.entityType,
              entityId: item.entityId,
              operation: item.operation,
              data: JSON.parse(item.data),
              checksum: item.checksum,
              clientVersion: item.clientVersion,
              serverVersion: item.serverVersion,
            }],
          }),
        });

        if (!response.ok) {
          throw new Error(`Server returned ${response.status}`);
        }

        const result = await response.json();

        if (result.conflicts && result.conflicts.length > 0) {
          conflicts++;
          await updateSyncQueueItem(item.id, 'conflict');
          // Store conflict for resolution
          for (const conflict of result.conflicts) {
            await saveConflict(
              item.entityType,
              item.entityId,
              JSON.parse(item.data),
              conflict.serverData
            );
          }
        } else {
          synced++;
          await updateSyncQueueItem(item.id, 'synced');
        }
      } catch (error) {
        failed++;
        const errorMessage = error instanceof Error ? error.message : 'Unknown error';
        errors.push(`${item.entityType}/${item.entityId}: ${errorMessage}`);
        await updateSyncQueueItem(item.id, 'failed', errorMessage);
      }
    }

    return { synced, failed, conflicts, errors };
  }

  private async pullChanges(): Promise<{ synced: number; errors: string[] }> {
    let synced = 0;
    const errors: string[] = [];

    try {
      const lastSync = await getSetting('lastSync');
      const since = lastSync || new Date(0).toISOString();

      const response = await fetch(
        `${API_BASE_URL}/api/offline/changes?since=${encodeURIComponent(since)}&deviceId=${this.deviceId}`,
        {
          headers: {
            'Authorization': `Bearer ${this.authToken}`,
            'X-Device-ID': this.deviceId || '',
          },
        }
      );

      if (!response.ok) {
        throw new Error(`Server returned ${response.status}`);
      }

      const changes = await response.json();

      // Process beneficiary changes
      for (const beneficiary of changes.beneficiaries || []) {
        await saveBeneficiary({
          ...beneficiary,
          syncStatus: 'synced',
          serverUpdatedAt: Date.now(),
        });
        synced++;
      }

      // Process household changes
      for (const household of changes.households || []) {
        await saveHousehold({
          ...household,
          syncStatus: 'synced',
          serverUpdatedAt: Date.now(),
        });
        synced++;
      }
    } catch (error) {
      errors.push(error instanceof Error ? error.message : 'Failed to pull changes');
    }

    return { synced, errors };
  }

  async registerDevice(): Promise<boolean> {
    if (!this.isOnline) return false;

    try {
      const response = await fetch(`${API_BASE_URL}/api/offline/devices/register`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.authToken}`,
        },
        body: JSON.stringify({
          deviceId: this.deviceId,
          deviceType: 'mobile',
          platform: 'react-native',
          appVersion: '1.0.0',
        }),
      });

      return response.ok;
    } catch (error) {
      console.error('Failed to register device:', error);
      return false;
    }
  }

  async uploadBiometric(beneficiaryId: string, imageData: string): Promise<boolean> {
    if (!this.isOnline) {
      // Queue for later upload
      await setSetting(`biometric_${beneficiaryId}`, imageData);
      return true;
    }

    try {
      const response = await fetch(`${API_BASE_URL}/api/biometric/upload`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.authToken}`,
        },
        body: JSON.stringify({
          beneficiaryId,
          imageData,
          capturedAt: new Date().toISOString(),
          deviceId: this.deviceId,
        }),
      });

      return response.ok;
    } catch (error) {
      console.error('Failed to upload biometric:', error);
      // Queue for later upload
      await setSetting(`biometric_${beneficiaryId}`, imageData);
      return false;
    }
  }

  async verifyNationalId(
    beneficiaryId: string,
    provider: string,
    nationalId: string,
    biometricData?: string
  ): Promise<{ success: boolean; result?: unknown; error?: string }> {
    if (!this.isOnline) {
      return { success: false, error: 'Device is offline' };
    }

    try {
      const response = await fetch(`${API_BASE_URL}/api/federation/verify`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.authToken}`,
        },
        body: JSON.stringify({
          beneficiaryId,
          provider,
          nationalId,
          biometricData,
          deviceId: this.deviceId,
        }),
      });

      if (!response.ok) {
        throw new Error(`Verification failed: ${response.status}`);
      }

      const result = await response.json();
      return { success: true, result };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : 'Verification failed',
      };
    }
  }

  async calculatePMTScore(householdId: string, surveyData: unknown): Promise<{
    success: boolean;
    score?: number;
    category?: string;
    error?: string;
  }> {
    if (!this.isOnline) {
      return { success: false, error: 'Device is offline. PMT calculation requires network.' };
    }

    try {
      const response = await fetch(`${API_BASE_URL}/api/pmt/calculate`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.authToken}`,
        },
        body: JSON.stringify({
          householdId,
          characteristics: surveyData,
        }),
      });

      if (!response.ok) {
        throw new Error(`PMT calculation failed: ${response.status}`);
      }

      const result = await response.json();
      return {
        success: true,
        score: result.pmtScore,
        category: result.eligibilityCategory,
      };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : 'PMT calculation failed',
      };
    }
  }

  getIsOnline(): boolean {
    return this.isOnline;
  }

  getIsSyncing(): boolean {
    return this.isSyncing;
  }

  // Journey-related methods
  async getJourneyContracts(): Promise<{ success: boolean; contracts?: unknown[]; error?: string }> {
    if (!this.isOnline) {
      // Return cached contracts if offline
      const cached = await getSetting('journeyContracts');
      if (cached) {
        return { success: true, contracts: JSON.parse(cached) };
      }
      return { success: false, error: 'Device is offline and no cached contracts available' };
    }

    try {
      const response = await fetch(`${API_BASE_URL}/api/journey-contracts`, {
        headers: {
          'Authorization': `Bearer ${this.authToken}`,
        },
      });

      if (!response.ok) {
        throw new Error(`Failed to fetch journey contracts: ${response.status}`);
      }

      const contracts = await response.json();
      // Cache for offline use
      await setSetting('journeyContracts', JSON.stringify(contracts));
      return { success: true, contracts };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : 'Failed to fetch journey contracts',
      };
    }
  }

  // Generate UUID for idempotency keys
  private generateUUID(): string {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
      const r = Math.random() * 16 | 0;
      const v = c === 'x' ? r : (r & 0x3 | 0x8);
      return v.toString(16);
    });
  }

  async startJourney(
    journeyKey: string,
    input: Record<string, unknown>
  ): Promise<{ success: boolean; journeyRunId?: string; queued?: boolean; error?: string }> {
    // Check for auth token before proceeding
    if (!this.authToken) {
      this.emit('authRequired', { message: 'Sign in to start journeys' });
      return { success: false, error: 'Authentication required' };
    }

    // Generate idempotency key to prevent duplicate journey starts on retry
    const idempotencyKey = this.generateUUID();
    
    const journeyRequest = {
      journeyKey,
      input,
      deviceId: this.deviceId,
      timestamp: new Date().toISOString(),
      idempotencyKey,
    };

    if (!this.isOnline) {
      // Queue journey for later execution
      const queue = await this.getJourneyQueue();
      const queuedJourney = {
        id: `journey-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
        ...journeyRequest,
        status: 'queued',
        attempts: 0,
      };
      queue.push(queuedJourney);
      await this.saveJourneyQueue(queue);
      this.emit('journeyQueued', queuedJourney);
      return { success: true, queued: true };
    }

    try {
      const response = await fetch(`${API_BASE_URL}/api/journeys/${journeyKey}/start`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.authToken}`,
          'X-Device-ID': this.deviceId || '',
          'X-Idempotency-Key': idempotencyKey,
        },
        body: JSON.stringify(input),
      });

      // Handle auth failures
      if (response.status === 401 || response.status === 403) {
        this.emit('authRequired', { message: 'Sign in to start journeys' });
        return { success: false, error: 'Authentication required' };
      }

      if (!response.ok) {
        throw new Error(`Failed to start journey: ${response.status}`);
      }

      const result = await response.json();
      this.emit('journeyStarted', { journeyKey, journeyRunId: result.journeyRunId });
      return { success: true, journeyRunId: result.journeyRunId };
    } catch (error) {
      // Queue for retry if network error
      const queue = await this.getJourneyQueue();
      const queuedJourney = {
        id: `journey-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
        ...journeyRequest,
        status: 'queued',
        attempts: 1,
        lastError: error instanceof Error ? error.message : 'Unknown error',
      };
      queue.push(queuedJourney);
      await this.saveJourneyQueue(queue);
      return {
        success: false,
        queued: true,
        error: error instanceof Error ? error.message : 'Failed to start journey',
      };
    }
  }

  async getJourneyStatus(
    journeyKey: string,
    journeyRunId: string
  ): Promise<{ success: boolean; status?: unknown; error?: string }> {
    if (!this.isOnline) {
      return { success: false, error: 'Device is offline' };
    }

    try {
      const response = await fetch(
        `${API_BASE_URL}/api/journeys/${journeyKey}/status?runId=${journeyRunId}`,
        {
          headers: {
            'Authorization': `Bearer ${this.authToken}`,
          },
        }
      );

      if (!response.ok) {
        throw new Error(`Failed to get journey status: ${response.status}`);
      }

      const status = await response.json();
      return { success: true, status };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : 'Failed to get journey status',
      };
    }
  }

  private async getJourneyQueue(): Promise<Array<{
    id: string;
    journeyKey: string;
    input: Record<string, unknown>;
    status: string;
    attempts: number;
    lastError?: string;
  }>> {
    const queueJson = await getSetting('journeyQueue');
    return queueJson ? JSON.parse(queueJson) : [];
  }

  private async saveJourneyQueue(queue: unknown[]): Promise<void> {
    await setSetting('journeyQueue', JSON.stringify(queue));
  }

  async syncQueuedJourneys(): Promise<{
    synced: number;
    failed: number;
    authBlocked: number;
    results: Array<{ id: string; success: boolean; error?: string }>;
  }> {
    if (!this.isOnline) {
      return { synced: 0, failed: 0, authBlocked: 0, results: [] };
    }

    // Check for auth token before syncing
    if (!this.authToken) {
      this.emit('authRequired', { message: 'Sign in to sync queued journeys' });
      return { synced: 0, failed: 0, authBlocked: 0, results: [] };
    }

    const queue = await this.getJourneyQueue();
    if (queue.length === 0) {
      return { synced: 0, failed: 0, authBlocked: 0, results: [] };
    }

    const results: Array<{ id: string; success: boolean; journeyRunId?: string; error?: string; authBlocked?: boolean }> = [];
    const remainingQueue: typeof queue = [];
    const authBlockedQueue: typeof queue = [];

    for (const item of queue) {
      try {
        // Use stored idempotency key to prevent duplicates on retry
        const idempotencyKey = (item as { idempotencyKey?: string }).idempotencyKey || this.generateUUID();
        
        const response = await fetch(`${API_BASE_URL}/api/journeys/${item.journeyKey}/start`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${this.authToken}`,
            'X-Device-ID': this.deviceId || '',
            'X-Idempotency-Key': idempotencyKey,
          },
          body: JSON.stringify(item.input),
        });

        // Handle auth failures - keep in queue but mark as blocked
        if (response.status === 401 || response.status === 403) {
          authBlockedQueue.push({ ...item, authBlocked: true });
          results.push({ id: item.id, success: false, error: 'Authentication required', authBlocked: true });
          continue;
        }

        if (response.ok) {
          const result = await response.json();
          results.push({ id: item.id, success: true, journeyRunId: result.journeyRunId });
        } else {
          item.attempts++;
          item.lastError = `Server returned ${response.status}`;
          if (item.attempts < 3) {
            remainingQueue.push(item);
          }
          results.push({ id: item.id, success: false, error: item.lastError });
        }
      } catch (error) {
        item.attempts++;
        item.lastError = error instanceof Error ? error.message : 'Unknown error';
        if (item.attempts < 3) {
          remainingQueue.push(item);
        }
        results.push({ id: item.id, success: false, error: item.lastError });
      }
    }

    // Save remaining and auth-blocked items back to queue
    await this.saveJourneyQueue([...remainingQueue, ...authBlockedQueue]);
    
    const syncResult = {
      synced: results.filter(r => r.success).length,
      failed: results.filter(r => !r.success && !r.authBlocked).length,
      authBlocked: authBlockedQueue.length,
      results,
    };
    
    // Emit auth required if any items are blocked
    if (authBlockedQueue.length > 0) {
      this.emit('authRequired', { 
        message: 'Sign in to sync queued journeys',
        blockedCount: authBlockedQueue.length,
      });
    }
    
    this.emit('journeysSynced', syncResult);
    return syncResult;
  }

  async getQueuedJourneyCount(): Promise<number> {
    const queue = await this.getJourneyQueue();
    return queue.length;
  }

  // Enrollment journey shortcuts
  async startBeneficiaryEnrollment(data: {
    nationalId: string;
    firstName: string;
    lastName: string;
    dateOfBirth: string;
    gender: string;
    phone?: string;
    address: {
      region: string;
      district: string;
      ward: string;
      village?: string;
    };
    programId?: string;
  }): Promise<{ success: boolean; journeyRunId?: string; queued?: boolean; error?: string }> {
    return this.startJourney('beneficiary-enrollment', data);
  }

  async startHouseholdRegistration(data: {
    headBeneficiaryId: string;
    members: Array<{
      nationalId?: string;
      firstName: string;
      lastName: string;
      relationship: string;
      dateOfBirth: string;
      gender: string;
    }>;
    address: {
      region: string;
      district: string;
      ward: string;
      village?: string;
    };
  }): Promise<{ success: boolean; journeyRunId?: string; queued?: boolean; error?: string }> {
    return this.startJourney('household-registration', data);
  }

  async startKYCVerification(data: {
    beneficiaryId: string;
    providerId: string;
    verificationMethod: 'otp' | 'biometric' | 'document';
  }): Promise<{ success: boolean; journeyRunId?: string; queued?: boolean; error?: string }> {
    return this.startJourney('kyc-verification', data);
  }

  // Grievance journey shortcuts
  async startGrievanceSubmission(data: {
    beneficiaryId: string;
    category: 'payment' | 'enrollment' | 'service' | 'other';
    description: string;
    priority?: 'low' | 'medium' | 'high' | 'critical';
    attachments?: string[];
  }): Promise<{ success: boolean; journeyRunId?: string; queued?: boolean; error?: string }> {
    return this.startJourney('grievance-submission', data);
  }
}

export const syncService = new SyncService();
