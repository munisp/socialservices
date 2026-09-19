/**
 * Verify ID Screen - National ID verification with biometric capture
 */

import React, { useState, useRef } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TextInput,
  TouchableOpacity,
  Alert,
  Image,
} from 'react-native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import { useNavigation, useRoute } from '@react-navigation/native';
import { useAppStore } from '../store/appStore';
import { syncService } from '../services/syncService';

const ID_PROVIDERS = [
  { id: 'aadhaar', name: 'Aadhaar (India)', icon: 'card-account-details', country: 'IN' },
  { id: 'nin', name: 'NIN (Nigeria)', icon: 'card-account-details', country: 'NG' },
  { id: 'nida', name: 'NIDA (Rwanda)', icon: 'card-account-details', country: 'RW' },
  { id: 'nadra', name: 'NADRA (Pakistan)', icon: 'card-account-details', country: 'PK' },
  { id: 'cpf', name: 'CPF (Brazil)', icon: 'card-account-details', country: 'BR' },
  { id: 'curp', name: 'CURP (Mexico)', icon: 'card-account-details', country: 'MX' },
];

interface VerificationResult {
  verified: boolean;
  matchScore: number;
  name?: string;
  dateOfBirth?: string;
  gender?: string;
  address?: string;
}

export function VerifyIDScreen() {
  const navigation = useNavigation<any>();
  const route = useRoute<any>();
  const { syncState } = useAppStore();
  const beneficiaryId = route.params?.beneficiaryId;

  const [currentStep, setCurrentStep] = useState(0);
  const [selectedProvider, setSelectedProvider] = useState<string | null>(null);
  const [nationalId, setNationalId] = useState('');
  const [capturedImage, setCapturedImage] = useState<string | null>(null);
  const [isVerifying, setIsVerifying] = useState(false);
  const [verificationResult, setVerificationResult] = useState<VerificationResult | null>(null);
  const [consentGiven, setConsentGiven] = useState(false);

  const handleProviderSelect = (providerId: string) => {
    setSelectedProvider(providerId);
    setCurrentStep(1);
  };

  const handleCapturePhoto = () => {
    Alert.alert(
      'Camera',
      'Camera integration would capture biometric photo here.',
      [
        {
          text: 'Simulate Capture',
          onPress: () => {
            setCapturedImage('captured');
            setCurrentStep(3);
          },
        },
        { text: 'Cancel', style: 'cancel' },
      ]
    );
  };

  const handleVerify = async () => {
    if (!syncState.isOnline) {
      Alert.alert('Offline', 'ID verification requires an internet connection.');
      return;
    }

    if (!consentGiven) {
      Alert.alert('Consent Required', 'Please provide consent to proceed with verification.');
      return;
    }

    setIsVerifying(true);

    try {
      const result = await syncService.verifyNationalId(
        beneficiaryId || `BEN-${Date.now()}`,
        selectedProvider!,
        nationalId,
        capturedImage || undefined
      );

      if (result.success && result.result) {
        setVerificationResult(result.result as VerificationResult);
        setCurrentStep(4);
      } else {
        Alert.alert('Verification Failed', result.error || 'Unable to verify identity.');
      }
    } catch (error) {
      Alert.alert('Error', 'An error occurred during verification.');
    } finally {
      setIsVerifying(false);
    }
  };

  const steps = [
    { title: 'Provider', icon: 'domain' },
    { title: 'ID Number', icon: 'card-text' },
    { title: 'Biometric', icon: 'camera' },
    { title: 'Consent', icon: 'shield-check' },
    { title: 'Result', icon: 'check-decagram' },
  ];

  return (
    <View style={styles.container}>
      {/* Progress */}
      <View style={styles.progressContainer}>
        {steps.map((step, index) => (
          <View key={index} style={styles.progressStep}>
            <View
              style={[
                styles.progressCircle,
                index <= currentStep && styles.progressCircleActive,
                index < currentStep && styles.progressCircleCompleted,
              ]}
            >
              {index < currentStep ? (
                <Icon name="check" size={14} color="#fff" />
              ) : (
                <Icon
                  name={step.icon}
                  size={14}
                  color={index <= currentStep ? '#fff' : '#999'}
                />
              )}
            </View>
          </View>
        ))}
      </View>

      <ScrollView style={styles.content}>
        {/* Step 0: Select Provider */}
        {currentStep === 0 && (
          <View style={styles.stepContent}>
            <Text style={styles.stepTitle}>Select ID Provider</Text>
            <Text style={styles.stepDescription}>
              Choose the national identity system to verify against
            </Text>

            <View style={styles.providerList}>
              {ID_PROVIDERS.map((provider) => (
                <TouchableOpacity
                  key={provider.id}
                  style={styles.providerCard}
                  onPress={() => handleProviderSelect(provider.id)}
                >
                  <Icon name={provider.icon} size={32} color="#2196F3" />
                  <Text style={styles.providerName}>{provider.name}</Text>
                  <Icon name="chevron-right" size={24} color="#999" />
                </TouchableOpacity>
              ))}
            </View>
          </View>
        )}

        {/* Step 1: Enter ID Number */}
        {currentStep === 1 && (
          <View style={styles.stepContent}>
            <Text style={styles.stepTitle}>Enter National ID</Text>
            <Text style={styles.stepDescription}>
              Enter the beneficiary's national identification number
            </Text>

            <View style={styles.inputGroup}>
              <Text style={styles.label}>National ID Number</Text>
              <TextInput
                style={styles.input}
                value={nationalId}
                onChangeText={setNationalId}
                placeholder="Enter ID number"
                autoCapitalize="characters"
              />
            </View>

            <View style={styles.buttonRow}>
              <TouchableOpacity
                style={styles.backButton}
                onPress={() => setCurrentStep(0)}
              >
                <Text style={styles.backButtonText}>Back</Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={[styles.nextButton, !nationalId && styles.buttonDisabled]}
                onPress={() => setCurrentStep(2)}
                disabled={!nationalId}
              >
                <Text style={styles.nextButtonText}>Next</Text>
              </TouchableOpacity>
            </View>
          </View>
        )}

        {/* Step 2: Biometric Capture */}
        {currentStep === 2 && (
          <View style={styles.stepContent}>
            <Text style={styles.stepTitle}>Biometric Capture</Text>
            <Text style={styles.stepDescription}>
              Capture a photo for biometric verification (optional)
            </Text>

            <View style={styles.cameraContainer}>
              {capturedImage ? (
                <View style={styles.capturedImageContainer}>
                  <Icon name="account-check" size={64} color="#4CAF50" />
                  <Text style={styles.capturedText}>Photo Captured</Text>
                </View>
              ) : (
                <TouchableOpacity
                  style={styles.cameraButton}
                  onPress={handleCapturePhoto}
                >
                  <Icon name="camera" size={48} color="#2196F3" />
                  <Text style={styles.cameraButtonText}>Capture Photo</Text>
                </TouchableOpacity>
              )}
            </View>

            <View style={styles.buttonRow}>
              <TouchableOpacity
                style={styles.backButton}
                onPress={() => setCurrentStep(1)}
              >
                <Text style={styles.backButtonText}>Back</Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={styles.nextButton}
                onPress={() => setCurrentStep(3)}
              >
                <Text style={styles.nextButtonText}>
                  {capturedImage ? 'Next' : 'Skip'}
                </Text>
              </TouchableOpacity>
            </View>
          </View>
        )}

        {/* Step 3: Consent */}
        {currentStep === 3 && (
          <View style={styles.stepContent}>
            <Text style={styles.stepTitle}>Consent</Text>
            <Text style={styles.stepDescription}>
              Please review and provide consent for identity verification
            </Text>

            <View style={styles.consentBox}>
              <Icon name="shield-check" size={32} color="#2196F3" />
              <Text style={styles.consentTitle}>Data Protection Notice</Text>
              <Text style={styles.consentText}>
                By proceeding, you consent to:
                {'\n\n'}• Verification of identity against the national ID system
                {'\n'}• Collection and processing of biometric data (if provided)
                {'\n'}• Storage of verification results for eligibility determination
                {'\n\n'}Your data will be protected according to applicable data protection laws.
              </Text>
            </View>

            <TouchableOpacity
              style={styles.consentCheckbox}
              onPress={() => setConsentGiven(!consentGiven)}
            >
              <Icon
                name={consentGiven ? 'checkbox-marked' : 'checkbox-blank-outline'}
                size={24}
                color={consentGiven ? '#4CAF50' : '#666'}
              />
              <Text style={styles.consentCheckboxText}>
                I understand and consent to the above
              </Text>
            </TouchableOpacity>

            <View style={styles.buttonRow}>
              <TouchableOpacity
                style={styles.backButton}
                onPress={() => setCurrentStep(2)}
              >
                <Text style={styles.backButtonText}>Back</Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={[styles.verifyButton, (!consentGiven || isVerifying) && styles.buttonDisabled]}
                onPress={handleVerify}
                disabled={!consentGiven || isVerifying}
              >
                <Text style={styles.verifyButtonText}>
                  {isVerifying ? 'Verifying...' : 'Verify Identity'}
                </Text>
              </TouchableOpacity>
            </View>
          </View>
        )}

        {/* Step 4: Result */}
        {currentStep === 4 && verificationResult && (
          <View style={styles.stepContent}>
            <View style={styles.resultCard}>
              <Icon
                name={verificationResult.verified ? 'check-circle' : 'close-circle'}
                size={64}
                color={verificationResult.verified ? '#4CAF50' : '#f44336'}
              />
              <Text style={styles.resultTitle}>
                {verificationResult.verified ? 'Verified' : 'Not Verified'}
              </Text>
              <Text style={styles.resultScore}>
                Match Score: {(verificationResult.matchScore * 100).toFixed(0)}%
              </Text>

              {verificationResult.verified && (
                <View style={styles.resultDetails}>
                  {verificationResult.name && (
                    <View style={styles.resultRow}>
                      <Text style={styles.resultLabel}>Name</Text>
                      <Text style={styles.resultValue}>{verificationResult.name}</Text>
                    </View>
                  )}
                  {verificationResult.dateOfBirth && (
                    <View style={styles.resultRow}>
                      <Text style={styles.resultLabel}>Date of Birth</Text>
                      <Text style={styles.resultValue}>{verificationResult.dateOfBirth}</Text>
                    </View>
                  )}
                  {verificationResult.gender && (
                    <View style={styles.resultRow}>
                      <Text style={styles.resultLabel}>Gender</Text>
                      <Text style={styles.resultValue}>{verificationResult.gender}</Text>
                    </View>
                  )}
                </View>
              )}
            </View>

            <TouchableOpacity
              style={styles.doneButton}
              onPress={() => navigation.goBack()}
            >
              <Text style={styles.doneButtonText}>Done</Text>
            </TouchableOpacity>
          </View>
        )}
      </ScrollView>
    </View>
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
    marginHorizontal: 8,
  },
  progressCircle: {
    width: 28,
    height: 28,
    borderRadius: 14,
    backgroundColor: '#e0e0e0',
    justifyContent: 'center',
    alignItems: 'center',
  },
  progressCircleActive: {
    backgroundColor: '#2196F3',
  },
  progressCircleCompleted: {
    backgroundColor: '#4CAF50',
  },
  content: {
    flex: 1,
  },
  stepContent: {
    padding: 20,
  },
  stepTitle: {
    fontSize: 24,
    fontWeight: '600',
    color: '#333',
    marginBottom: 8,
  },
  stepDescription: {
    fontSize: 14,
    color: '#666',
    marginBottom: 24,
  },
  providerList: {
    gap: 12,
  },
  providerCard: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 12,
    gap: 16,
  },
  providerName: {
    flex: 1,
    fontSize: 16,
    fontWeight: '500',
    color: '#333',
  },
  inputGroup: {
    marginBottom: 24,
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
  cameraContainer: {
    alignItems: 'center',
    marginVertical: 32,
  },
  cameraButton: {
    width: 200,
    height: 200,
    borderRadius: 100,
    backgroundColor: '#E3F2FD',
    justifyContent: 'center',
    alignItems: 'center',
  },
  cameraButtonText: {
    fontSize: 16,
    color: '#2196F3',
    marginTop: 8,
  },
  capturedImageContainer: {
    width: 200,
    height: 200,
    borderRadius: 100,
    backgroundColor: '#E8F5E9',
    justifyContent: 'center',
    alignItems: 'center',
  },
  capturedText: {
    fontSize: 16,
    color: '#4CAF50',
    marginTop: 8,
  },
  consentBox: {
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 20,
    alignItems: 'center',
    marginBottom: 20,
  },
  consentTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: '#333',
    marginTop: 12,
    marginBottom: 12,
  },
  consentText: {
    fontSize: 14,
    color: '#666',
    lineHeight: 22,
  },
  consentCheckbox: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 24,
  },
  consentCheckboxText: {
    fontSize: 16,
    color: '#333',
    marginLeft: 12,
  },
  buttonRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    marginTop: 16,
  },
  backButton: {
    paddingVertical: 12,
    paddingHorizontal: 24,
  },
  backButtonText: {
    fontSize: 16,
    color: '#666',
  },
  nextButton: {
    backgroundColor: '#2196F3',
    paddingVertical: 12,
    paddingHorizontal: 32,
    borderRadius: 8,
  },
  nextButtonText: {
    fontSize: 16,
    color: '#fff',
    fontWeight: '500',
  },
  verifyButton: {
    backgroundColor: '#4CAF50',
    paddingVertical: 12,
    paddingHorizontal: 32,
    borderRadius: 8,
  },
  verifyButtonText: {
    fontSize: 16,
    color: '#fff',
    fontWeight: '500',
  },
  buttonDisabled: {
    opacity: 0.5,
  },
  resultCard: {
    backgroundColor: '#fff',
    borderRadius: 16,
    padding: 32,
    alignItems: 'center',
  },
  resultTitle: {
    fontSize: 24,
    fontWeight: '600',
    color: '#333',
    marginTop: 16,
  },
  resultScore: {
    fontSize: 16,
    color: '#666',
    marginTop: 8,
  },
  resultDetails: {
    width: '100%',
    marginTop: 24,
    paddingTop: 24,
    borderTopWidth: 1,
    borderTopColor: '#e0e0e0',
  },
  resultRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: 8,
  },
  resultLabel: {
    fontSize: 14,
    color: '#666',
  },
  resultValue: {
    fontSize: 14,
    fontWeight: '500',
    color: '#333',
  },
  doneButton: {
    backgroundColor: '#2196F3',
    paddingVertical: 16,
    borderRadius: 8,
    alignItems: 'center',
    marginTop: 24,
  },
  doneButtonText: {
    fontSize: 18,
    color: '#fff',
    fontWeight: '600',
  },
});

export default VerifyIDScreen;
