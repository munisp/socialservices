/**
 * Social Protection Platform - Field Worker Mobile App
 * 
 * React Native application for field workers to:
 * - Enroll beneficiaries offline
 * - Conduct PMT surveys
 * - Verify national IDs
 * - Sync data with the server
 */

import React, { useEffect } from 'react';
import { StatusBar, LogBox } from 'react-native';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { GestureHandlerRootView } from 'react-native-gesture-handler';
import { AppNavigator } from './src/navigation/AppNavigator';
import { initDatabase } from './src/services/database';
import { syncService } from './src/services/syncService';

LogBox.ignoreLogs([
  'Non-serializable values were found in the navigation state',
]);

export default function App() {
  useEffect(() => {
    const initializeApp = async () => {
      try {
        await initDatabase();
        await syncService.init();
        console.log('App initialized successfully');
      } catch (error) {
        console.error('Failed to initialize app:', error);
      }
    };

    initializeApp();

    return () => {
      syncService.stopPeriodicSync();
    };
  }, []);

  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      <SafeAreaProvider>
        <StatusBar barStyle="dark-content" backgroundColor="#fff" />
        <AppNavigator />
      </SafeAreaProvider>
    </GestureHandlerRootView>
  );
}
