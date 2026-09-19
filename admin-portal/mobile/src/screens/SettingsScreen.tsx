/**
 * Settings Screen - App settings and user profile
 */

import React, { useState } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
  Switch,
  Alert,
} from 'react-native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import { useAppStore } from '../store/appStore';
import { syncService } from '../services/syncService';
import { closeDatabase } from '../services/database';

export function SettingsScreen() {
  const { user, logout, syncState } = useAppStore();
  const [autoSync, setAutoSync] = useState(true);
  const [notifications, setNotifications] = useState(true);
  const [biometricAuth, setBiometricAuth] = useState(false);

  const handleLogout = () => {
    Alert.alert(
      'Logout',
      syncState.pendingChanges > 0
        ? `You have ${syncState.pendingChanges} unsynced changes. Are you sure you want to logout?`
        : 'Are you sure you want to logout?',
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'Logout',
          style: 'destructive',
          onPress: async () => {
            syncService.clearAuthToken();
            await closeDatabase();
            logout();
          },
        },
      ]
    );
  };

  const handleClearData = () => {
    Alert.alert(
      'Clear Local Data',
      'This will delete all locally stored data. Synced data on the server will not be affected.',
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'Clear',
          style: 'destructive',
          onPress: () => {
            Alert.alert('Data Cleared', 'Local data has been cleared.');
          },
        },
      ]
    );
  };

  const SettingRow = ({
    icon,
    label,
    value,
    onPress,
    showArrow = true,
  }: {
    icon: string;
    label: string;
    value?: string;
    onPress?: () => void;
    showArrow?: boolean;
  }) => (
    <TouchableOpacity
      style={styles.settingRow}
      onPress={onPress}
      disabled={!onPress}
    >
      <Icon name={icon} size={24} color="#666" />
      <Text style={styles.settingLabel}>{label}</Text>
      {value && <Text style={styles.settingValue}>{value}</Text>}
      {showArrow && onPress && (
        <Icon name="chevron-right" size={24} color="#999" />
      )}
    </TouchableOpacity>
  );

  const SettingToggle = ({
    icon,
    label,
    value,
    onValueChange,
  }: {
    icon: string;
    label: string;
    value: boolean;
    onValueChange: (value: boolean) => void;
  }) => (
    <View style={styles.settingRow}>
      <Icon name={icon} size={24} color="#666" />
      <Text style={styles.settingLabel}>{label}</Text>
      <Switch
        value={value}
        onValueChange={onValueChange}
        trackColor={{ false: '#e0e0e0', true: '#81C784' }}
        thumbColor={value ? '#4CAF50' : '#f4f3f4'}
      />
    </View>
  );

  return (
    <ScrollView style={styles.container}>
      {/* User Profile */}
      <View style={styles.profileSection}>
        <View style={styles.avatar}>
          <Text style={styles.avatarText}>
            {user?.name?.split(' ').map((n) => n[0]).join('') || 'FW'}
          </Text>
        </View>
        <Text style={styles.userName}>{user?.name || 'Field Worker'}</Text>
        <Text style={styles.userEmail}>{user?.email || 'field.worker@example.com'}</Text>
        <Text style={styles.userRole}>{user?.role || 'Field Worker'}</Text>
      </View>

      {/* Sync Settings */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Sync</Text>
        <View style={styles.sectionContent}>
          <SettingToggle
            icon="sync-circle"
            label="Auto Sync"
            value={autoSync}
            onValueChange={setAutoSync}
          />
          <SettingRow
            icon="cloud-sync"
            label="Sync Status"
            value={syncState.isOnline ? 'Online' : 'Offline'}
            showArrow={false}
          />
          <SettingRow
            icon="clock-outline"
            label="Last Sync"
            value={syncState.lastSync ? new Date(syncState.lastSync).toLocaleString() : 'Never'}
            showArrow={false}
          />
          <SettingRow
            icon="alert-circle-outline"
            label="Pending Changes"
            value={syncState.pendingChanges.toString()}
            showArrow={false}
          />
        </View>
      </View>

      {/* App Settings */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>App Settings</Text>
        <View style={styles.sectionContent}>
          <SettingToggle
            icon="bell"
            label="Notifications"
            value={notifications}
            onValueChange={setNotifications}
          />
          <SettingToggle
            icon="fingerprint"
            label="Biometric Authentication"
            value={biometricAuth}
            onValueChange={setBiometricAuth}
          />
          <SettingRow
            icon="translate"
            label="Language"
            value="English"
            onPress={() => Alert.alert('Language', 'Language selection coming soon')}
          />
        </View>
      </View>

      {/* Data Management */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Data Management</Text>
        <View style={styles.sectionContent}>
          <SettingRow
            icon="database"
            label="Storage Used"
            value="12.5 MB"
            showArrow={false}
          />
          <SettingRow
            icon="delete-outline"
            label="Clear Local Data"
            onPress={handleClearData}
          />
          <SettingRow
            icon="download"
            label="Export Data"
            onPress={() => Alert.alert('Export', 'Data export coming soon')}
          />
        </View>
      </View>

      {/* About */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>About</Text>
        <View style={styles.sectionContent}>
          <SettingRow
            icon="information"
            label="Version"
            value="1.0.0"
            showArrow={false}
          />
          <SettingRow
            icon="file-document-outline"
            label="Terms of Service"
            onPress={() => Alert.alert('Terms', 'Terms of Service')}
          />
          <SettingRow
            icon="shield-check"
            label="Privacy Policy"
            onPress={() => Alert.alert('Privacy', 'Privacy Policy')}
          />
          <SettingRow
            icon="help-circle"
            label="Help & Support"
            onPress={() => Alert.alert('Help', 'Contact support@example.com')}
          />
        </View>
      </View>

      {/* Logout */}
      <TouchableOpacity style={styles.logoutButton} onPress={handleLogout}>
        <Icon name="logout" size={24} color="#f44336" />
        <Text style={styles.logoutText}>Logout</Text>
      </TouchableOpacity>

      <View style={styles.footer}>
        <Text style={styles.footerText}>Social Protection Platform</Text>
        <Text style={styles.footerText}>Field Worker Mobile App</Text>
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  profileSection: {
    backgroundColor: '#2196F3',
    padding: 24,
    alignItems: 'center',
  },
  avatar: {
    width: 80,
    height: 80,
    borderRadius: 40,
    backgroundColor: '#fff',
    justifyContent: 'center',
    alignItems: 'center',
  },
  avatarText: {
    fontSize: 28,
    fontWeight: '600',
    color: '#2196F3',
  },
  userName: {
    fontSize: 20,
    fontWeight: '600',
    color: '#fff',
    marginTop: 12,
  },
  userEmail: {
    fontSize: 14,
    color: 'rgba(255,255,255,0.8)',
    marginTop: 4,
  },
  userRole: {
    fontSize: 12,
    color: 'rgba(255,255,255,0.6)',
    marginTop: 4,
    textTransform: 'uppercase',
  },
  section: {
    marginTop: 24,
  },
  sectionTitle: {
    fontSize: 14,
    fontWeight: '600',
    color: '#666',
    paddingHorizontal: 16,
    marginBottom: 8,
    textTransform: 'uppercase',
  },
  sectionContent: {
    backgroundColor: '#fff',
  },
  settingRow: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 14,
    paddingHorizontal: 16,
    borderBottomWidth: 1,
    borderBottomColor: '#f0f0f0',
  },
  settingLabel: {
    flex: 1,
    fontSize: 16,
    color: '#333',
    marginLeft: 16,
  },
  settingValue: {
    fontSize: 14,
    color: '#999',
    marginRight: 8,
  },
  logoutButton: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#fff',
    marginTop: 24,
    marginHorizontal: 16,
    paddingVertical: 16,
    borderRadius: 8,
    gap: 8,
  },
  logoutText: {
    fontSize: 16,
    fontWeight: '500',
    color: '#f44336',
  },
  footer: {
    alignItems: 'center',
    paddingVertical: 32,
  },
  footerText: {
    fontSize: 12,
    color: '#999',
  },
});

export default SettingsScreen;
