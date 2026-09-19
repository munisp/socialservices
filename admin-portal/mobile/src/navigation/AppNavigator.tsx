/**
 * App Navigator - Main navigation structure for the mobile app
 */

import React from 'react';
import { NavigationContainer } from '@react-navigation/native';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';

import { useAppStore } from '../store/appStore';
import { HomeScreen } from '../screens/HomeScreen';
import { EnrollBeneficiaryScreen } from '../screens/EnrollBeneficiaryScreen';
import { BeneficiariesScreen } from '../screens/BeneficiariesScreen';
import { HouseholdsScreen } from '../screens/HouseholdsScreen';
import { PMTSurveyScreen } from '../screens/PMTSurveyScreen';
import { VerifyIDScreen } from '../screens/VerifyIDScreen';
import { SyncScreen } from '../screens/SyncScreen';
import { SettingsScreen } from '../screens/SettingsScreen';
import { LoginScreen } from '../screens/LoginScreen';
import JourneysScreen from '../screens/JourneysScreen';

export type RootStackParamList = {
  Login: undefined;
  Main: undefined;
  EnrollBeneficiary: undefined;
  EnrollHousehold: undefined;
  BeneficiaryDetail: { id: string };
  HouseholdDetail: { id: string };
  PMTSurvey: { householdId?: string };
  VerifyID: { beneficiaryId?: string };
};

export type MainTabParamList = {
  Home: undefined;
  Journeys: undefined;
  Beneficiaries: undefined;
  Households: undefined;
  Sync: undefined;
  Settings: undefined;
};

const Stack = createNativeStackNavigator<RootStackParamList>();
const Tab = createBottomTabNavigator<MainTabParamList>();

function MainTabs() {
  const { syncState } = useAppStore();

  return (
    <Tab.Navigator
      screenOptions={({ route }) => ({
        tabBarIcon: ({ focused, color, size }) => {
          let iconName: string;

                    switch (route.name) {
                      case 'Home':
                        iconName = focused ? 'home' : 'home-outline';
                        break;
                      case 'Journeys':
                        iconName = focused ? 'routes' : 'routes';
                        break;
                      case 'Beneficiaries':
                        iconName = focused ? 'account-group' : 'account-group-outline';
                        break;
                      case 'Households':
                        iconName = focused ? 'home-group' : 'home-group';
                        break;
                      case 'Sync':
                        iconName = syncState.isOnline ? 'cloud-sync' : 'cloud-off-outline';
                        break;
                      case 'Settings':
                        iconName = focused ? 'cog' : 'cog-outline';
                        break;
                      default:
                        iconName = 'circle';
                    }

          return <Icon name={iconName} size={size} color={color} />;
        },
        tabBarActiveTintColor: '#2196F3',
        tabBarInactiveTintColor: '#999',
        tabBarBadge: route.name === 'Sync' && syncState.pendingChanges > 0
          ? syncState.pendingChanges
          : undefined,
        headerShown: false,
      })}
    >
            <Tab.Screen name="Home" component={HomeScreen} />
            <Tab.Screen name="Journeys" component={JourneysScreen} />
            <Tab.Screen name="Beneficiaries" component={BeneficiariesScreen} />
            <Tab.Screen name="Households" component={HouseholdsScreen} />
            <Tab.Screen name="Sync" component={SyncScreen} />
            <Tab.Screen name="Settings" component={SettingsScreen} />
    </Tab.Navigator>
  );
}

export function AppNavigator() {
  const { isAuthenticated } = useAppStore();

  return (
    <NavigationContainer>
      <Stack.Navigator
        screenOptions={{
          headerStyle: {
            backgroundColor: '#2196F3',
          },
          headerTintColor: '#fff',
          headerTitleStyle: {
            fontWeight: '600',
          },
        }}
      >
        {!isAuthenticated ? (
          <Stack.Screen
            name="Login"
            component={LoginScreen}
            options={{ headerShown: false }}
          />
        ) : (
          <>
            <Stack.Screen
              name="Main"
              component={MainTabs}
              options={{ headerShown: false }}
            />
            <Stack.Screen
              name="EnrollBeneficiary"
              component={EnrollBeneficiaryScreen}
              options={{ title: 'Enroll Beneficiary' }}
            />
            <Stack.Screen
              name="PMTSurvey"
              component={PMTSurveyScreen}
              options={{ title: 'PMT Survey' }}
            />
            <Stack.Screen
              name="VerifyID"
              component={VerifyIDScreen}
              options={{ title: 'Verify Identity' }}
            />
          </>
        )}
      </Stack.Navigator>
    </NavigationContainer>
  );
}

export default AppNavigator;
