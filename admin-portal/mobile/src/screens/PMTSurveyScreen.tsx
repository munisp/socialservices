/**
 * PMT Survey Screen - Proxy Means Test survey for households
 */

import React, { useState } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TextInput,
  TouchableOpacity,
  Alert,
  KeyboardAvoidingView,
  Platform,
} from 'react-native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import { useNavigation, useRoute } from '@react-navigation/native';
import { useAppStore } from '../store/appStore';
import { savePMTSurvey } from '../services/database';
import { syncService } from '../services/syncService';

interface SurveyData {
  householdSize: string;
  numChildrenUnder5: string;
  numElderlyOver65: string;
  headEducationYears: string;
  housingType: string;
  wallMaterial: string;
  roofMaterial: string;
  floorMaterial: string;
  numRooms: string;
  hasElectricity: boolean;
  hasPipedWater: boolean;
  hasFlushToilet: boolean;
  ownsLand: boolean;
  ownsLivestock: boolean;
  ownsVehicle: boolean;
  ownsRefrigerator: boolean;
  ownsTelevision: boolean;
  ownsMobilePhone: boolean;
  urbanRural: string;
}

const HOUSING_TYPES = ['permanent', 'semi_permanent', 'temporary', 'traditional'];
const WALL_MATERIALS = ['brick_cement', 'stone', 'wood', 'mud', 'bamboo', 'metal'];
const ROOF_MATERIALS = ['concrete', 'tiles', 'metal', 'asbestos', 'thatch'];
const FLOOR_MATERIALS = ['tiles', 'cement', 'wood', 'earth'];

export function PMTSurveyScreen() {
  const navigation = useNavigation<any>();
  const route = useRoute<any>();
  const { syncState } = useAppStore();
  const householdId = route.params?.householdId;

  const [currentStep, setCurrentStep] = useState(0);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [pmtResult, setPmtResult] = useState<{ score: number; category: string } | null>(null);

  const [surveyData, setSurveyData] = useState<SurveyData>({
    householdSize: '1',
    numChildrenUnder5: '0',
    numElderlyOver65: '0',
    headEducationYears: '0',
    housingType: 'temporary',
    wallMaterial: 'mud',
    roofMaterial: 'thatch',
    floorMaterial: 'earth',
    numRooms: '1',
    hasElectricity: false,
    hasPipedWater: false,
    hasFlushToilet: false,
    ownsLand: false,
    ownsLivestock: false,
    ownsVehicle: false,
    ownsRefrigerator: false,
    ownsTelevision: false,
    ownsMobilePhone: false,
    urbanRural: 'rural',
  });

  const updateField = (field: keyof SurveyData, value: string | boolean) => {
    setSurveyData((prev) => ({ ...prev, [field]: value }));
  };

  const steps = [
    { title: 'Demographics', icon: 'account-group' },
    { title: 'Housing', icon: 'home' },
    { title: 'Assets', icon: 'car' },
  ];

  const handleSubmit = async () => {
    setIsSubmitting(true);

    try {
      const id = `PMT-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
      const hId = householdId || `HH-${Date.now()}`;

      // Save survey locally
      await savePMTSurvey({
        id,
        householdId: hId,
        surveyData: JSON.stringify(surveyData),
        syncStatus: 'pending',
      });

      // Try to calculate PMT score if online
      if (syncState.isOnline) {
        const result = await syncService.calculatePMTScore(hId, {
          householdSize: parseInt(surveyData.householdSize),
          numChildrenUnder5: parseInt(surveyData.numChildrenUnder5),
          numElderlyOver65: parseInt(surveyData.numElderlyOver65),
          headEducationYears: parseInt(surveyData.headEducationYears),
          housingType: surveyData.housingType,
          wallMaterial: surveyData.wallMaterial,
          roofMaterial: surveyData.roofMaterial,
          floorMaterial: surveyData.floorMaterial,
          numRooms: parseInt(surveyData.numRooms),
          hasElectricity: surveyData.hasElectricity,
          hasPipedWater: surveyData.hasPipedWater,
          hasFlushToilet: surveyData.hasFlushToilet,
          ownsLand: surveyData.ownsLand,
          ownsLivestock: surveyData.ownsLivestock,
          ownsVehicle: surveyData.ownsVehicle,
          ownsRefrigerator: surveyData.ownsRefrigerator,
          ownsTelevision: surveyData.ownsTelevision,
          ownsMobilePhone: surveyData.ownsMobilePhone,
          urbanRural: surveyData.urbanRural,
        });

        if (result.success && result.score !== undefined) {
          setPmtResult({ score: result.score, category: result.category || 'unknown' });
        }
      }

      if (!pmtResult) {
        Alert.alert(
          'Survey Saved',
          'Survey data has been saved locally. PMT score will be calculated when online.',
          [{ text: 'OK', onPress: () => navigation.goBack() }]
        );
      }
    } catch (error) {
      Alert.alert('Error', 'Failed to save survey. Please try again.');
    } finally {
      setIsSubmitting(false);
    }
  };

  const renderOptionButton = (
    field: keyof SurveyData,
    value: string,
    label: string
  ) => (
    <TouchableOpacity
      key={value}
      style={[
        styles.optionButton,
        surveyData[field] === value && styles.optionButtonActive,
      ]}
      onPress={() => updateField(field, value)}
    >
      <Text
        style={[
          styles.optionButtonText,
          surveyData[field] === value && styles.optionButtonTextActive,
        ]}
      >
        {label}
      </Text>
    </TouchableOpacity>
  );

  const renderCheckbox = (field: keyof SurveyData, label: string) => (
    <TouchableOpacity
      style={styles.checkboxRow}
      onPress={() => updateField(field, !surveyData[field])}
    >
      <Icon
        name={surveyData[field] ? 'checkbox-marked' : 'checkbox-blank-outline'}
        size={24}
        color={surveyData[field] ? '#2196F3' : '#666'}
      />
      <Text style={styles.checkboxLabel}>{label}</Text>
    </TouchableOpacity>
  );

  if (pmtResult) {
    return (
      <View style={styles.resultContainer}>
        <View style={styles.resultCard}>
          <Icon
            name="chart-arc"
            size={64}
            color={pmtResult.category === 'extremely_poor' ? '#f44336' : '#4CAF50'}
          />
          <Text style={styles.resultScore}>{pmtResult.score.toFixed(1)}</Text>
          <Text style={styles.resultLabel}>PMT Score</Text>
          <View style={[
            styles.categoryBadge,
            { backgroundColor: pmtResult.category === 'extremely_poor' ? '#FFEBEE' : '#E8F5E9' }
          ]}>
            <Text style={[
              styles.categoryText,
              { color: pmtResult.category === 'extremely_poor' ? '#f44336' : '#4CAF50' }
            ]}>
              {pmtResult.category.replace(/_/g, ' ').toUpperCase()}
            </Text>
          </View>
        </View>
        <TouchableOpacity
          style={styles.doneButton}
          onPress={() => navigation.goBack()}
        >
          <Text style={styles.doneButtonText}>Done</Text>
        </TouchableOpacity>
      </View>
    );
  }

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}
    >
      {/* Progress */}
      <View style={styles.progressContainer}>
        {steps.map((step, index) => (
          <View key={index} style={styles.progressStep}>
            <View
              style={[
                styles.progressCircle,
                index <= currentStep && styles.progressCircleActive,
              ]}
            >
              <Icon
                name={step.icon}
                size={16}
                color={index <= currentStep ? '#fff' : '#999'}
              />
            </View>
            <Text style={[
              styles.progressLabel,
              index <= currentStep && styles.progressLabelActive,
            ]}>
              {step.title}
            </Text>
          </View>
        ))}
      </View>

      <ScrollView style={styles.formContainer}>
        {/* Step 0: Demographics */}
        {currentStep === 0 && (
          <View style={styles.stepContent}>
            <Text style={styles.stepTitle}>Household Demographics</Text>

            <View style={styles.inputGroup}>
              <Text style={styles.label}>Household Size</Text>
              <TextInput
                style={styles.input}
                value={surveyData.householdSize}
                onChangeText={(v) => updateField('householdSize', v)}
                keyboardType="numeric"
              />
            </View>

            <View style={styles.inputGroup}>
              <Text style={styles.label}>Children Under 5</Text>
              <TextInput
                style={styles.input}
                value={surveyData.numChildrenUnder5}
                onChangeText={(v) => updateField('numChildrenUnder5', v)}
                keyboardType="numeric"
              />
            </View>

            <View style={styles.inputGroup}>
              <Text style={styles.label}>Elderly Over 65</Text>
              <TextInput
                style={styles.input}
                value={surveyData.numElderlyOver65}
                onChangeText={(v) => updateField('numElderlyOver65', v)}
                keyboardType="numeric"
              />
            </View>

            <View style={styles.inputGroup}>
              <Text style={styles.label}>Head's Education (years)</Text>
              <TextInput
                style={styles.input}
                value={surveyData.headEducationYears}
                onChangeText={(v) => updateField('headEducationYears', v)}
                keyboardType="numeric"
              />
            </View>

            <View style={styles.inputGroup}>
              <Text style={styles.label}>Area Type</Text>
              <View style={styles.optionsRow}>
                {renderOptionButton('urbanRural', 'urban', 'Urban')}
                {renderOptionButton('urbanRural', 'rural', 'Rural')}
              </View>
            </View>
          </View>
        )}

        {/* Step 1: Housing */}
        {currentStep === 1 && (
          <View style={styles.stepContent}>
            <Text style={styles.stepTitle}>Housing Characteristics</Text>

            <View style={styles.inputGroup}>
              <Text style={styles.label}>Housing Type</Text>
              <View style={styles.optionsGrid}>
                {HOUSING_TYPES.map((type) =>
                  renderOptionButton('housingType', type, type.replace(/_/g, ' '))
                )}
              </View>
            </View>

            <View style={styles.inputGroup}>
              <Text style={styles.label}>Wall Material</Text>
              <View style={styles.optionsGrid}>
                {WALL_MATERIALS.map((mat) =>
                  renderOptionButton('wallMaterial', mat, mat.replace(/_/g, ' '))
                )}
              </View>
            </View>

            <View style={styles.inputGroup}>
              <Text style={styles.label}>Number of Rooms</Text>
              <TextInput
                style={styles.input}
                value={surveyData.numRooms}
                onChangeText={(v) => updateField('numRooms', v)}
                keyboardType="numeric"
              />
            </View>

            <View style={styles.inputGroup}>
              <Text style={styles.label}>Utilities</Text>
              {renderCheckbox('hasElectricity', 'Has Electricity')}
              {renderCheckbox('hasPipedWater', 'Has Piped Water')}
              {renderCheckbox('hasFlushToilet', 'Has Flush Toilet')}
            </View>
          </View>
        )}

        {/* Step 2: Assets */}
        {currentStep === 2 && (
          <View style={styles.stepContent}>
            <Text style={styles.stepTitle}>Household Assets</Text>

            <View style={styles.inputGroup}>
              <Text style={styles.label}>Property</Text>
              {renderCheckbox('ownsLand', 'Owns Agricultural Land')}
              {renderCheckbox('ownsLivestock', 'Owns Livestock')}
              {renderCheckbox('ownsVehicle', 'Owns Vehicle')}
            </View>

            <View style={styles.inputGroup}>
              <Text style={styles.label}>Appliances</Text>
              {renderCheckbox('ownsRefrigerator', 'Owns Refrigerator')}
              {renderCheckbox('ownsTelevision', 'Owns Television')}
              {renderCheckbox('ownsMobilePhone', 'Owns Mobile Phone')}
            </View>
          </View>
        )}
      </ScrollView>

      {/* Navigation */}
      <View style={styles.buttonContainer}>
        <TouchableOpacity
          style={styles.backButton}
          onPress={() => currentStep > 0 ? setCurrentStep(currentStep - 1) : navigation.goBack()}
        >
          <Icon name="arrow-left" size={20} color="#666" />
          <Text style={styles.backButtonText}>
            {currentStep > 0 ? 'Back' : 'Cancel'}
          </Text>
        </TouchableOpacity>

        {currentStep < steps.length - 1 ? (
          <TouchableOpacity
            style={styles.nextButton}
            onPress={() => setCurrentStep(currentStep + 1)}
          >
            <Text style={styles.nextButtonText}>Next</Text>
            <Icon name="arrow-right" size={20} color="#fff" />
          </TouchableOpacity>
        ) : (
          <TouchableOpacity
            style={[styles.submitButton, isSubmitting && styles.buttonDisabled]}
            onPress={handleSubmit}
            disabled={isSubmitting}
          >
            <Text style={styles.submitButtonText}>
              {isSubmitting ? 'Calculating...' : 'Calculate PMT'}
            </Text>
          </TouchableOpacity>
        )}
      </View>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  progressContainer: {
    flexDirection: 'row',
    justifyContent: 'center',
    paddingVertical: 16,
    backgroundColor: '#fff',
  },
  progressStep: {
    alignItems: 'center',
    marginHorizontal: 24,
  },
  progressCircle: {
    width: 32,
    height: 32,
    borderRadius: 16,
    backgroundColor: '#e0e0e0',
    justifyContent: 'center',
    alignItems: 'center',
  },
  progressCircleActive: {
    backgroundColor: '#4CAF50',
  },
  progressLabel: {
    fontSize: 12,
    color: '#999',
    marginTop: 4,
  },
  progressLabelActive: {
    color: '#333',
    fontWeight: '500',
  },
  formContainer: {
    flex: 1,
  },
  stepContent: {
    padding: 20,
  },
  stepTitle: {
    fontSize: 20,
    fontWeight: '600',
    color: '#333',
    marginBottom: 20,
  },
  inputGroup: {
    marginBottom: 20,
  },
  label: {
    fontSize: 14,
    fontWeight: '500',
    color: '#333',
    marginBottom: 8,
  },
  input: {
    backgroundColor: '#fff',
    borderWidth: 1,
    borderColor: '#ddd',
    borderRadius: 8,
    paddingHorizontal: 16,
    paddingVertical: 12,
    fontSize: 16,
  },
  optionsRow: {
    flexDirection: 'row',
    gap: 8,
  },
  optionsGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
  },
  optionButton: {
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 8,
    backgroundColor: '#fff',
    borderWidth: 1,
    borderColor: '#ddd',
  },
  optionButtonActive: {
    backgroundColor: '#2196F3',
    borderColor: '#2196F3',
  },
  optionButtonText: {
    fontSize: 14,
    color: '#666',
    textTransform: 'capitalize',
  },
  optionButtonTextActive: {
    color: '#fff',
  },
  checkboxRow: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 8,
  },
  checkboxLabel: {
    fontSize: 16,
    color: '#333',
    marginLeft: 12,
  },
  buttonContainer: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    padding: 16,
    backgroundColor: '#fff',
    borderTopWidth: 1,
    borderTopColor: '#e0e0e0',
  },
  backButton: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 12,
    paddingHorizontal: 16,
    gap: 8,
  },
  backButtonText: {
    fontSize: 16,
    color: '#666',
  },
  nextButton: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#2196F3',
    paddingVertical: 12,
    paddingHorizontal: 24,
    borderRadius: 8,
    gap: 8,
  },
  nextButtonText: {
    fontSize: 16,
    color: '#fff',
    fontWeight: '500',
  },
  submitButton: {
    backgroundColor: '#4CAF50',
    paddingVertical: 12,
    paddingHorizontal: 24,
    borderRadius: 8,
  },
  submitButtonText: {
    fontSize: 16,
    color: '#fff',
    fontWeight: '500',
  },
  buttonDisabled: {
    opacity: 0.6,
  },
  resultContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 20,
    backgroundColor: '#f5f5f5',
  },
  resultCard: {
    backgroundColor: '#fff',
    borderRadius: 16,
    padding: 32,
    alignItems: 'center',
    width: '100%',
  },
  resultScore: {
    fontSize: 48,
    fontWeight: 'bold',
    color: '#333',
    marginTop: 16,
  },
  resultLabel: {
    fontSize: 16,
    color: '#666',
    marginTop: 4,
  },
  categoryBadge: {
    marginTop: 16,
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 8,
  },
  categoryText: {
    fontSize: 14,
    fontWeight: '600',
  },
  doneButton: {
    marginTop: 24,
    backgroundColor: '#2196F3',
    paddingVertical: 16,
    paddingHorizontal: 48,
    borderRadius: 8,
  },
  doneButtonText: {
    fontSize: 18,
    color: '#fff',
    fontWeight: '600',
  },
});

export default PMTSurveyScreen;
