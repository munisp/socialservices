/**
 * Zustand Store for Mobile App State Management
 */

import { create } from 'zustand';
import { BeneficiaryRecord, HouseholdRecord } from '../services/database';

interface User {
  id: string;
  email: string;
  name: string;
  role: string;
}

interface SyncState {
  isOnline: boolean;
  isSyncing: boolean;
  lastSync: string | null;
  pendingChanges: number;
  conflicts: number;
}

interface AppState {
  // Auth
  user: User | null;
  isAuthenticated: boolean;
  authToken: string | null;

  // Sync
  syncState: SyncState;

  // Data
  beneficiaries: BeneficiaryRecord[];
  households: HouseholdRecord[];
  selectedBeneficiary: BeneficiaryRecord | null;
  selectedHousehold: HouseholdRecord | null;

  // UI
  isLoading: boolean;
  error: string | null;

  // Actions
  setUser: (user: User | null) => void;
  setAuthToken: (token: string | null) => void;
  logout: () => void;

  setSyncState: (state: Partial<SyncState>) => void;

  setBeneficiaries: (beneficiaries: BeneficiaryRecord[]) => void;
  addBeneficiary: (beneficiary: BeneficiaryRecord) => void;
  updateBeneficiary: (id: string, updates: Partial<BeneficiaryRecord>) => void;
  setSelectedBeneficiary: (beneficiary: BeneficiaryRecord | null) => void;

  setHouseholds: (households: HouseholdRecord[]) => void;
  addHousehold: (household: HouseholdRecord) => void;
  updateHousehold: (id: string, updates: Partial<HouseholdRecord>) => void;
  setSelectedHousehold: (household: HouseholdRecord | null) => void;

  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;
  clearError: () => void;
}

export const useAppStore = create<AppState>((set) => ({
  // Initial state
  user: null,
  isAuthenticated: false,
  authToken: null,

  syncState: {
    isOnline: false,
    isSyncing: false,
    lastSync: null,
    pendingChanges: 0,
    conflicts: 0,
  },

  beneficiaries: [],
  households: [],
  selectedBeneficiary: null,
  selectedHousehold: null,

  isLoading: false,
  error: null,

  // Auth actions
  setUser: (user) => set({ user, isAuthenticated: !!user }),
  setAuthToken: (authToken) => set({ authToken }),
  logout: () => set({
    user: null,
    isAuthenticated: false,
    authToken: null,
    beneficiaries: [],
    households: [],
    selectedBeneficiary: null,
    selectedHousehold: null,
  }),

  // Sync actions
  setSyncState: (state) => set((prev) => ({
    syncState: { ...prev.syncState, ...state },
  })),

  // Beneficiary actions
  setBeneficiaries: (beneficiaries) => set({ beneficiaries }),
  addBeneficiary: (beneficiary) => set((state) => ({
    beneficiaries: [beneficiary, ...state.beneficiaries],
  })),
  updateBeneficiary: (id, updates) => set((state) => ({
    beneficiaries: state.beneficiaries.map((b) =>
      b.id === id ? { ...b, ...updates } : b
    ),
    selectedBeneficiary:
      state.selectedBeneficiary?.id === id
        ? { ...state.selectedBeneficiary, ...updates }
        : state.selectedBeneficiary,
  })),
  setSelectedBeneficiary: (selectedBeneficiary) => set({ selectedBeneficiary }),

  // Household actions
  setHouseholds: (households) => set({ households }),
  addHousehold: (household) => set((state) => ({
    households: [household, ...state.households],
  })),
  updateHousehold: (id, updates) => set((state) => ({
    households: state.households.map((h) =>
      h.id === id ? { ...h, ...updates } : h
    ),
    selectedHousehold:
      state.selectedHousehold?.id === id
        ? { ...state.selectedHousehold, ...updates }
        : state.selectedHousehold,
  })),
  setSelectedHousehold: (selectedHousehold) => set({ selectedHousehold }),

  // UI actions
  setLoading: (isLoading) => set({ isLoading }),
  setError: (error) => set({ error }),
  clearError: () => set({ error: null }),
}));
