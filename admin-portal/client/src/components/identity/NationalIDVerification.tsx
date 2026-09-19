import React, { useState, useCallback, useRef } from 'react';
import {
  Shield,
  CheckCircle,
  XCircle,
  AlertTriangle,
  Camera,
  Fingerprint,
  User,
  FileText,
  Clock,
  RefreshCw,
  ChevronRight,
  Lock,
  Unlock,
  Globe,
  Building,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from '@/components/ui/alert';
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs';

// Identity Provider types
type IdentityProvider = 
  | 'aadhaar' 
  | 'nin' 
  | 'nida' 
  | 'npr' 
  | 'nadra' 
  | 'dukcapil' 
  | 'cpf' 
  | 'rut' 
  | 'curp' 
  | 'ssn' 
  | 'nhif';

interface ProviderInfo {
  id: IdentityProvider;
  name: string;
  country: string;
  flag: string;
  requiresBiometric: boolean;
  requiresOTP: boolean;
  idFormat: string;
  idPlaceholder: string;
}

const IDENTITY_PROVIDERS: ProviderInfo[] = [
  { id: 'aadhaar', name: 'Aadhaar', country: 'India', flag: '🇮🇳', requiresBiometric: true, requiresOTP: true, idFormat: '#### #### ####', idPlaceholder: '1234 5678 9012' },
  { id: 'nin', name: 'NIN', country: 'Nigeria', flag: '🇳🇬', requiresBiometric: true, requiresOTP: false, idFormat: '###########', idPlaceholder: '12345678901' },
  { id: 'nida', name: 'NIDA', country: 'Rwanda', flag: '🇷🇼', requiresBiometric: true, requiresOTP: false, idFormat: '################', idPlaceholder: '1199012345678901' },
  { id: 'npr', name: 'NPR', country: 'Nepal', flag: '🇳🇵', requiresBiometric: false, requiresOTP: true, idFormat: '##-##-##-#####', idPlaceholder: '12-34-56-78901' },
  { id: 'nadra', name: 'NADRA', country: 'Pakistan', flag: '🇵🇰', requiresBiometric: true, requiresOTP: true, idFormat: '#####-#######-#', idPlaceholder: '12345-1234567-1' },
  { id: 'dukcapil', name: 'Dukcapil', country: 'Indonesia', flag: '🇮🇩', requiresBiometric: false, requiresOTP: false, idFormat: '################', idPlaceholder: '1234567890123456' },
  { id: 'cpf', name: 'CPF', country: 'Brazil', flag: '🇧🇷', requiresBiometric: false, requiresOTP: false, idFormat: '###.###.###-##', idPlaceholder: '123.456.789-00' },
  { id: 'rut', name: 'RUT', country: 'Chile', flag: '🇨🇱', requiresBiometric: false, requiresOTP: false, idFormat: '##.###.###-#', idPlaceholder: '12.345.678-9' },
  { id: 'curp', name: 'CURP', country: 'Mexico', flag: '🇲🇽', requiresBiometric: false, requiresOTP: false, idFormat: '##################', idPlaceholder: 'ABCD123456HDFXXX00' },
  { id: 'ssn', name: 'SSN', country: 'USA', flag: '🇺🇸', requiresBiometric: false, requiresOTP: false, idFormat: '###-##-####', idPlaceholder: '123-45-6789' },
  { id: 'nhif', name: 'NHIF', country: 'Kenya', flag: '🇰🇪', requiresBiometric: false, requiresOTP: true, idFormat: '########', idPlaceholder: '12345678' },
];

interface VerificationResult {
  status: 'verified' | 'failed' | 'pending' | 'partial';
  provider: IdentityProvider;
  nationalId: string;
  verifiedAt?: string;
  biometricMatch?: number;
  demographicMatch?: number;
  details?: {
    name?: string;
    dateOfBirth?: string;
    gender?: string;
    address?: string;
    photo?: string;
  };
  errors?: string[];
}

interface ConsentData {
  dataSharing: boolean;
  biometricCapture: boolean;
  termsAccepted: boolean;
  timestamp?: string;
}

// Main verification component
export function NationalIDVerification({ 
  beneficiaryId,
  onVerificationComplete 
}: { 
  beneficiaryId: string;
  onVerificationComplete?: (result: VerificationResult) => void;
}) {
  const [step, setStep] = useState<'provider' | 'consent' | 'input' | 'biometric' | 'otp' | 'verifying' | 'result'>('provider');
  const [selectedProvider, setSelectedProvider] = useState<ProviderInfo | null>(null);
  const [nationalId, setNationalId] = useState('');
  const [consent, setConsent] = useState<ConsentData>({ dataSharing: false, biometricCapture: false, termsAccepted: false });
  const [otp, setOtp] = useState('');
  const [biometricData, setBiometricData] = useState<string | null>(null);
  const [verificationResult, setVerificationResult] = useState<VerificationResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);

  const handleProviderSelect = (providerId: IdentityProvider) => {
    const provider = IDENTITY_PROVIDERS.find(p => p.id === providerId);
    if (provider) {
      setSelectedProvider(provider);
      setStep('consent');
    }
  };

  const handleConsentSubmit = () => {
    if (!consent.dataSharing || !consent.termsAccepted) {
      setError('Please accept all required consents');
      return;
    }
    if (selectedProvider?.requiresBiometric && !consent.biometricCapture) {
      setError('Biometric consent is required for this provider');
      return;
    }
    setConsent({ ...consent, timestamp: new Date().toISOString() });
    setStep('input');
    setError(null);
  };

  const handleIdSubmit = () => {
    if (!nationalId.trim()) {
      setError('Please enter your National ID');
      return;
    }
    setError(null);
    
    if (selectedProvider?.requiresBiometric) {
      setStep('biometric');
    } else if (selectedProvider?.requiresOTP) {
      sendOTP();
      setStep('otp');
    } else {
      startVerification();
    }
  };

  const startCamera = useCallback(async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ 
        video: { facingMode: 'user', width: 640, height: 480 } 
      });
      if (videoRef.current) {
        videoRef.current.srcObject = stream;
      }
    } catch (err) {
      setError('Failed to access camera. Please ensure camera permissions are granted.');
    }
  }, []);

  const capturePhoto = () => {
    if (videoRef.current && canvasRef.current) {
      const context = canvasRef.current.getContext('2d');
      if (context) {
        canvasRef.current.width = videoRef.current.videoWidth;
        canvasRef.current.height = videoRef.current.videoHeight;
        context.drawImage(videoRef.current, 0, 0);
        const imageData = canvasRef.current.toDataURL('image/jpeg', 0.8);
        setBiometricData(imageData);
        
        // Stop camera
        const stream = videoRef.current.srcObject as MediaStream;
        stream?.getTracks().forEach(track => track.stop());
        
        if (selectedProvider?.requiresOTP) {
          sendOTP();
          setStep('otp');
        } else {
          startVerification();
        }
      }
    }
  };

  const sendOTP = async () => {
    setIsLoading(true);
    try {
      await fetch('/api/federation/send-otp', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          provider: selectedProvider?.id,
          nationalId,
        }),
      });
      setIsLoading(false);
    } catch (err) {
      setIsLoading(false);
      setError('Failed to send OTP. Please try again.');
    }
  };

  const handleOtpSubmit = () => {
    if (otp.length < 4) {
      setError('Please enter a valid OTP');
      return;
    }
    startVerification();
  };

  const startVerification = async () => {
    setStep('verifying');
    setIsLoading(true);
    setError(null);

    try {
      const response = await fetch('/api/federation/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          beneficiaryId,
          provider: selectedProvider?.id,
          nationalId,
          biometricData,
          otp: otp || undefined,
          consent,
        }),
      });

      if (!response.ok) {
        throw new Error('Verification failed');
      }

      const result: VerificationResult = await response.json();
      setVerificationResult(result);
      setStep('result');
      onVerificationComplete?.(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Verification failed');
      setStep('result');
      setVerificationResult({
        status: 'failed',
        provider: selectedProvider!.id,
        nationalId,
        errors: [err instanceof Error ? err.message : 'Unknown error'],
      });
    } finally {
      setIsLoading(false);
    }
  };

  const resetVerification = () => {
    setStep('provider');
    setSelectedProvider(null);
    setNationalId('');
    setConsent({ dataSharing: false, biometricCapture: false, termsAccepted: false });
    setOtp('');
    setBiometricData(null);
    setVerificationResult(null);
    setError(null);
  };

  return (
    <Card className="w-full max-w-2xl mx-auto">
      <CardHeader>
        <div className="flex items-center gap-2">
          <Shield className="h-6 w-6 text-primary" />
          <CardTitle>National ID Verification</CardTitle>
        </div>
        <CardDescription>
          Verify identity using national ID systems
        </CardDescription>
      </CardHeader>

      <CardContent>
        {/* Progress indicator */}
        <div className="mb-6">
          <div className="flex justify-between text-xs text-muted-foreground mb-2">
            <span className={step === 'provider' ? 'text-primary font-medium' : ''}>Provider</span>
            <span className={step === 'consent' ? 'text-primary font-medium' : ''}>Consent</span>
            <span className={step === 'input' ? 'text-primary font-medium' : ''}>ID Input</span>
            <span className={['biometric', 'otp'].includes(step) ? 'text-primary font-medium' : ''}>Verify</span>
            <span className={step === 'result' ? 'text-primary font-medium' : ''}>Result</span>
          </div>
          <Progress 
            value={
              step === 'provider' ? 0 :
              step === 'consent' ? 25 :
              step === 'input' ? 50 :
              ['biometric', 'otp', 'verifying'].includes(step) ? 75 :
              100
            } 
          />
        </div>

        {error && (
          <Alert variant="destructive" className="mb-4">
            <AlertTriangle className="h-4 w-4" />
            <AlertTitle>Error</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        {/* Step: Provider Selection */}
        {step === 'provider' && (
          <div className="space-y-4">
            <Label>Select Identity Provider</Label>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              {IDENTITY_PROVIDERS.map((provider) => (
                <Button
                  key={provider.id}
                  variant="outline"
                  className="h-auto p-4 flex flex-col items-center gap-2"
                  onClick={() => handleProviderSelect(provider.id)}
                >
                  <span className="text-2xl">{provider.flag}</span>
                  <span className="font-medium">{provider.name}</span>
                  <span className="text-xs text-muted-foreground">{provider.country}</span>
                  <div className="flex gap-1 mt-1">
                    {provider.requiresBiometric && (
                      <Badge variant="secondary" className="text-xs">
                        <Fingerprint className="h-3 w-3 mr-1" />
                        Bio
                      </Badge>
                    )}
                    {provider.requiresOTP && (
                      <Badge variant="secondary" className="text-xs">
                        <Lock className="h-3 w-3 mr-1" />
                        OTP
                      </Badge>
                    )}
                  </div>
                </Button>
              ))}
            </div>
          </div>
        )}

        {/* Step: Consent */}
        {step === 'consent' && selectedProvider && (
          <div className="space-y-6">
            <div className="flex items-center gap-3 p-4 bg-muted rounded-lg">
              <span className="text-3xl">{selectedProvider.flag}</span>
              <div>
                <div className="font-medium">{selectedProvider.name}</div>
                <div className="text-sm text-muted-foreground">{selectedProvider.country}</div>
              </div>
            </div>

            <div className="space-y-4">
              <div className="flex items-start space-x-3">
                <Checkbox
                  id="dataSharing"
                  checked={consent.dataSharing}
                  onCheckedChange={(checked) => 
                    setConsent({ ...consent, dataSharing: checked as boolean })
                  }
                />
                <div className="grid gap-1.5 leading-none">
                  <Label htmlFor="dataSharing" className="font-medium">
                    Data Sharing Consent *
                  </Label>
                  <p className="text-sm text-muted-foreground">
                    I consent to share my identity data with the Social Protection Platform 
                    for verification purposes.
                  </p>
                </div>
              </div>

              {selectedProvider.requiresBiometric && (
                <div className="flex items-start space-x-3">
                  <Checkbox
                    id="biometric"
                    checked={consent.biometricCapture}
                    onCheckedChange={(checked) => 
                      setConsent({ ...consent, biometricCapture: checked as boolean })
                    }
                  />
                  <div className="grid gap-1.5 leading-none">
                    <Label htmlFor="biometric" className="font-medium">
                      Biometric Capture Consent *
                    </Label>
                    <p className="text-sm text-muted-foreground">
                      I consent to capture and process my biometric data (facial image) 
                      for identity verification.
                    </p>
                  </div>
                </div>
              )}

              <div className="flex items-start space-x-3">
                <Checkbox
                  id="terms"
                  checked={consent.termsAccepted}
                  onCheckedChange={(checked) => 
                    setConsent({ ...consent, termsAccepted: checked as boolean })
                  }
                />
                <div className="grid gap-1.5 leading-none">
                  <Label htmlFor="terms" className="font-medium">
                    Terms & Conditions *
                  </Label>
                  <p className="text-sm text-muted-foreground">
                    I have read and accept the terms and conditions for identity verification.
                  </p>
                </div>
              </div>
            </div>

            <div className="flex gap-2">
              <Button variant="outline" onClick={() => setStep('provider')}>
                Back
              </Button>
              <Button onClick={handleConsentSubmit}>
                Continue
                <ChevronRight className="h-4 w-4 ml-1" />
              </Button>
            </div>
          </div>
        )}

        {/* Step: ID Input */}
        {step === 'input' && selectedProvider && (
          <div className="space-y-6">
            <div className="space-y-2">
              <Label htmlFor="nationalId">
                {selectedProvider.name} Number
              </Label>
              <Input
                id="nationalId"
                placeholder={selectedProvider.idPlaceholder}
                value={nationalId}
                onChange={(e) => setNationalId(e.target.value)}
                className="text-lg tracking-wider"
              />
              <p className="text-xs text-muted-foreground">
                Format: {selectedProvider.idFormat}
              </p>
            </div>

            <div className="flex gap-2">
              <Button variant="outline" onClick={() => setStep('consent')}>
                Back
              </Button>
              <Button onClick={handleIdSubmit}>
                {selectedProvider.requiresBiometric ? 'Capture Biometric' : 
                 selectedProvider.requiresOTP ? 'Send OTP' : 'Verify'}
                <ChevronRight className="h-4 w-4 ml-1" />
              </Button>
            </div>
          </div>
        )}

        {/* Step: Biometric Capture */}
        {step === 'biometric' && (
          <div className="space-y-6">
            <div className="text-center">
              <Camera className="h-12 w-12 mx-auto text-muted-foreground mb-2" />
              <h3 className="font-medium">Capture Facial Image</h3>
              <p className="text-sm text-muted-foreground">
                Position your face within the frame and click capture
              </p>
            </div>

            <div className="relative aspect-video bg-black rounded-lg overflow-hidden">
              <video
                ref={videoRef}
                autoPlay
                playsInline
                muted
                className="w-full h-full object-cover"
                onLoadedMetadata={() => startCamera()}
              />
              <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
                <div className="w-48 h-64 border-2 border-white/50 rounded-full" />
              </div>
            </div>
            <canvas ref={canvasRef} className="hidden" />

            <div className="flex gap-2 justify-center">
              <Button variant="outline" onClick={() => setStep('input')}>
                Back
              </Button>
              <Button onClick={capturePhoto}>
                <Camera className="h-4 w-4 mr-2" />
                Capture Photo
              </Button>
            </div>
          </div>
        )}

        {/* Step: OTP */}
        {step === 'otp' && (
          <div className="space-y-6">
            <div className="text-center">
              <Lock className="h-12 w-12 mx-auto text-muted-foreground mb-2" />
              <h3 className="font-medium">Enter OTP</h3>
              <p className="text-sm text-muted-foreground">
                A one-time password has been sent to your registered mobile number
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="otp">One-Time Password</Label>
              <Input
                id="otp"
                placeholder="Enter OTP"
                value={otp}
                onChange={(e) => setOtp(e.target.value)}
                className="text-center text-2xl tracking-widest"
                maxLength={6}
              />
            </div>

            <div className="flex gap-2 justify-center">
              <Button variant="outline" onClick={() => setStep('input')}>
                Back
              </Button>
              <Button variant="ghost" onClick={sendOTP} disabled={isLoading}>
                <RefreshCw className={`h-4 w-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
                Resend OTP
              </Button>
              <Button onClick={handleOtpSubmit}>
                Verify
                <ChevronRight className="h-4 w-4 ml-1" />
              </Button>
            </div>
          </div>
        )}

        {/* Step: Verifying */}
        {step === 'verifying' && (
          <div className="text-center py-12">
            <RefreshCw className="h-16 w-16 mx-auto text-primary animate-spin mb-4" />
            <h3 className="font-medium text-lg">Verifying Identity</h3>
            <p className="text-sm text-muted-foreground">
              Please wait while we verify your identity with {selectedProvider?.name}...
            </p>
          </div>
        )}

        {/* Step: Result */}
        {step === 'result' && verificationResult && (
          <div className="space-y-6">
            <div className={`text-center p-6 rounded-lg ${
              verificationResult.status === 'verified' ? 'bg-green-50' :
              verificationResult.status === 'partial' ? 'bg-yellow-50' :
              'bg-red-50'
            }`}>
              {verificationResult.status === 'verified' ? (
                <CheckCircle className="h-16 w-16 mx-auto text-green-500 mb-4" />
              ) : verificationResult.status === 'partial' ? (
                <AlertTriangle className="h-16 w-16 mx-auto text-yellow-500 mb-4" />
              ) : (
                <XCircle className="h-16 w-16 mx-auto text-red-500 mb-4" />
              )}
              <h3 className="font-medium text-lg capitalize">
                {verificationResult.status === 'verified' ? 'Identity Verified' :
                 verificationResult.status === 'partial' ? 'Partial Verification' :
                 'Verification Failed'}
              </h3>
            </div>

            {verificationResult.status === 'verified' && verificationResult.details && (
              <div className="space-y-3">
                <div className="flex items-center justify-between p-3 bg-muted rounded">
                  <span className="text-sm text-muted-foreground">Name</span>
                  <span className="font-medium">{verificationResult.details.name}</span>
                </div>
                <div className="flex items-center justify-between p-3 bg-muted rounded">
                  <span className="text-sm text-muted-foreground">Date of Birth</span>
                  <span className="font-medium">{verificationResult.details.dateOfBirth}</span>
                </div>
                <div className="flex items-center justify-between p-3 bg-muted rounded">
                  <span className="text-sm text-muted-foreground">Gender</span>
                  <span className="font-medium">{verificationResult.details.gender}</span>
                </div>
                {verificationResult.biometricMatch !== undefined && (
                  <div className="flex items-center justify-between p-3 bg-muted rounded">
                    <span className="text-sm text-muted-foreground">Biometric Match</span>
                    <Badge variant={verificationResult.biometricMatch > 0.9 ? 'default' : 'secondary'}>
                      {(verificationResult.biometricMatch * 100).toFixed(1)}%
                    </Badge>
                  </div>
                )}
                {verificationResult.demographicMatch !== undefined && (
                  <div className="flex items-center justify-between p-3 bg-muted rounded">
                    <span className="text-sm text-muted-foreground">Demographic Match</span>
                    <Badge variant={verificationResult.demographicMatch > 0.9 ? 'default' : 'secondary'}>
                      {(verificationResult.demographicMatch * 100).toFixed(1)}%
                    </Badge>
                  </div>
                )}
              </div>
            )}

            {verificationResult.errors && verificationResult.errors.length > 0 && (
              <Alert variant="destructive">
                <AlertTriangle className="h-4 w-4" />
                <AlertTitle>Verification Errors</AlertTitle>
                <AlertDescription>
                  <ul className="list-disc list-inside">
                    {verificationResult.errors.map((err, i) => (
                      <li key={i}>{err}</li>
                    ))}
                  </ul>
                </AlertDescription>
              </Alert>
            )}

            <Button onClick={resetVerification} className="w-full">
              <RefreshCw className="h-4 w-4 mr-2" />
              Start New Verification
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// Verification history component
export function VerificationHistory({ beneficiaryId }: { beneficiaryId: string }) {
  const [verifications, setVerifications] = useState<VerificationResult[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  React.useEffect(() => {
    fetch(`/api/federation/history/${beneficiaryId}`)
      .then(res => res.json())
      .then(data => {
        setVerifications(data.verifications || []);
        setIsLoading(false);
      })
      .catch(() => setIsLoading(false));
  }, [beneficiaryId]);

  if (isLoading) {
    return <div className="text-center py-8">Loading verification history...</div>;
  }

  if (verifications.length === 0) {
    return (
      <div className="text-center py-8 text-muted-foreground">
        No verification history found
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {verifications.map((v, i) => {
        const provider = IDENTITY_PROVIDERS.find(p => p.id === v.provider);
        return (
          <Card key={i}>
            <CardContent className="p-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <span className="text-2xl">{provider?.flag}</span>
                  <div>
                    <div className="font-medium">{provider?.name}</div>
                    <div className="text-sm text-muted-foreground">
                      {v.nationalId.replace(/./g, (c, i) => i < 4 ? c : '*')}
                    </div>
                  </div>
                </div>
                <div className="text-right">
                  <Badge variant={
                    v.status === 'verified' ? 'default' :
                    v.status === 'partial' ? 'secondary' :
                    'destructive'
                  }>
                    {v.status}
                  </Badge>
                  {v.verifiedAt && (
                    <div className="text-xs text-muted-foreground mt-1">
                      {new Date(v.verifiedAt).toLocaleDateString()}
                    </div>
                  )}
                </div>
              </div>
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}

export default NationalIDVerification;
