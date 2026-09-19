/**
 * Login Screen - Authentication for field workers
 */

import React, { useState } from 'react';
import {
  View,
  Text,
  StyleSheet,
  TextInput,
  TouchableOpacity,
  KeyboardAvoidingView,
  Platform,
  Alert,
  ActivityIndicator,
} from 'react-native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import { useAppStore } from '../store/appStore';
import { syncService } from '../services/syncService';
import { initDatabase } from '../services/database';

export function LoginScreen() {
  const { setUser, setAuthToken, setSyncState } = useAppStore();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleLogin = async () => {
    if (!email.trim() || !password.trim()) {
      setError('Please enter email and password');
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      // Initialize database
      await initDatabase();

      // Initialize sync service
      await syncService.init();

      // For demo purposes, accept any credentials
      // In production, this would call the authentication API
      const mockUser = {
        id: `user-${Date.now()}`,
        email: email.trim(),
        name: email.split('@')[0].replace(/[._]/g, ' ').replace(/\b\w/g, c => c.toUpperCase()),
        role: 'field_worker',
      };

      const mockToken = `token-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

      // Set auth token in sync service
      syncService.setAuthToken(mockToken);

      // Register device
      await syncService.registerDevice();

      // Update app state
      setAuthToken(mockToken);
      setUser(mockUser);

      // Get initial sync status
      const status = await syncService.getStatus();
      setSyncState({
        isOnline: status.isOnline,
        lastSync: status.lastSync,
        pendingChanges: status.pendingChanges,
      });

    } catch (err) {
      setError('Login failed. Please try again.');
      console.error('Login error:', err);
    } finally {
      setIsLoading(false);
    }
  };

  const handleOfflineMode = async () => {
    Alert.alert(
      'Offline Mode',
      'You can use the app offline with limited functionality. Data will sync when you connect.',
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'Continue Offline',
          onPress: async () => {
            setIsLoading(true);
            try {
              await initDatabase();
              await syncService.init();

              setUser({
                id: 'offline-user',
                email: 'offline@local',
                name: 'Offline User',
                role: 'field_worker',
              });

              setSyncState({
                isOnline: false,
                lastSync: null,
                pendingChanges: 0,
              });
            } catch (err) {
              setError('Failed to initialize offline mode');
            } finally {
              setIsLoading(false);
            }
          },
        },
      ]
    );
  };

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}
    >
      <View style={styles.header}>
        <Icon name="shield-account" size={80} color="#2196F3" />
        <Text style={styles.title}>Social Protection</Text>
        <Text style={styles.subtitle}>Field Worker App</Text>
      </View>

      <View style={styles.form}>
        {error && (
          <View style={styles.errorContainer}>
            <Icon name="alert-circle" size={20} color="#f44336" />
            <Text style={styles.errorText}>{error}</Text>
          </View>
        )}

        <View style={styles.inputContainer}>
          <Icon name="email" size={20} color="#666" style={styles.inputIcon} />
          <TextInput
            style={styles.input}
            placeholder="Email"
            value={email}
            onChangeText={setEmail}
            keyboardType="email-address"
            autoCapitalize="none"
            autoCorrect={false}
            editable={!isLoading}
          />
        </View>

        <View style={styles.inputContainer}>
          <Icon name="lock" size={20} color="#666" style={styles.inputIcon} />
          <TextInput
            style={styles.input}
            placeholder="Password"
            value={password}
            onChangeText={setPassword}
            secureTextEntry={!showPassword}
            editable={!isLoading}
          />
          <TouchableOpacity
            onPress={() => setShowPassword(!showPassword)}
            style={styles.eyeButton}
          >
            <Icon
              name={showPassword ? 'eye-off' : 'eye'}
              size={20}
              color="#666"
            />
          </TouchableOpacity>
        </View>

        <TouchableOpacity
          style={[styles.loginButton, isLoading && styles.loginButtonDisabled]}
          onPress={handleLogin}
          disabled={isLoading}
        >
          {isLoading ? (
            <ActivityIndicator color="#fff" />
          ) : (
            <Text style={styles.loginButtonText}>Login</Text>
          )}
        </TouchableOpacity>

        <TouchableOpacity
          style={styles.offlineButton}
          onPress={handleOfflineMode}
          disabled={isLoading}
        >
          <Icon name="wifi-off" size={20} color="#666" />
          <Text style={styles.offlineButtonText}>Continue Offline</Text>
        </TouchableOpacity>
      </View>

      <View style={styles.footer}>
        <Text style={styles.footerText}>
          By logging in, you agree to the Terms of Service and Privacy Policy
        </Text>
      </View>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#fff',
  },
  header: {
    alignItems: 'center',
    paddingTop: 80,
    paddingBottom: 40,
  },
  title: {
    fontSize: 28,
    fontWeight: 'bold',
    color: '#333',
    marginTop: 16,
  },
  subtitle: {
    fontSize: 16,
    color: '#666',
    marginTop: 4,
  },
  form: {
    paddingHorizontal: 24,
  },
  errorContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#FFEBEE',
    padding: 12,
    borderRadius: 8,
    marginBottom: 16,
    gap: 8,
  },
  errorText: {
    color: '#f44336',
    fontSize: 14,
    flex: 1,
  },
  inputContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#f5f5f5',
    borderRadius: 8,
    marginBottom: 16,
    paddingHorizontal: 12,
  },
  inputIcon: {
    marginRight: 8,
  },
  input: {
    flex: 1,
    paddingVertical: 14,
    fontSize: 16,
    color: '#333',
  },
  eyeButton: {
    padding: 8,
  },
  loginButton: {
    backgroundColor: '#2196F3',
    paddingVertical: 16,
    borderRadius: 8,
    alignItems: 'center',
    marginTop: 8,
  },
  loginButtonDisabled: {
    opacity: 0.7,
  },
  loginButtonText: {
    color: '#fff',
    fontSize: 18,
    fontWeight: '600',
  },
  offlineButton: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    paddingVertical: 16,
    marginTop: 16,
    gap: 8,
  },
  offlineButtonText: {
    color: '#666',
    fontSize: 16,
  },
  footer: {
    position: 'absolute',
    bottom: 32,
    left: 24,
    right: 24,
  },
  footerText: {
    textAlign: 'center',
    fontSize: 12,
    color: '#999',
  },
});

export default LoginScreen;
