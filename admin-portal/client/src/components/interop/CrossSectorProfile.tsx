import React, { useState, useEffect } from 'react';
import {
  Building2,
  Heart,
  GraduationCap,
  Receipt,
  Briefcase,
  Shield,
  CheckCircle,
  XCircle,
  AlertTriangle,
  Clock,
  RefreshCw,
  Lock,
  Unlock,
  Eye,
  EyeOff,
  FileText,
  Users,
  ChevronRight,
  ExternalLink,
  Database,
  Link2,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
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
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';

// Sector types
type SectorType = 'health' | 'education' | 'tax' | 'labor' | 'agriculture' | 'housing' | 'social';

type ConsentStatus = 'granted' | 'denied' | 'pending' | 'revoked' | 'expired';

interface SectorInfo {
  id: SectorType;
  name: string;
  icon: React.ComponentType<{ className?: string }>;
  description: string;
  dataTypes: string[];
  color: string;
}

const SECTORS: SectorInfo[] = [
  {
    id: 'health',
    name: 'Health',
    icon: Heart,
    description: 'Health insurance, medical records, disability status',
    dataTypes: ['Insurance Status', 'Chronic Conditions', 'Disability', 'Vaccination'],
    color: 'text-red-500',
  },
  {
    id: 'education',
    name: 'Education',
    icon: GraduationCap,
    description: 'School enrollment, attendance, scholarships',
    dataTypes: ['Enrollment', 'Attendance', 'Grade Level', 'Scholarships'],
    color: 'text-blue-500',
  },
  {
    id: 'tax',
    name: 'Tax',
    icon: Receipt,
    description: 'Tax filing status, income declaration, property',
    dataTypes: ['Filing Status', 'Income', 'Property', 'Business'],
    color: 'text-green-500',
  },
  {
    id: 'labor',
    name: 'Labor',
    icon: Briefcase,
    description: 'Employment status, social security, unemployment',
    dataTypes: ['Employment', 'Employer', 'Social Security', 'Unemployment'],
    color: 'text-purple-500',
  },
];

interface HealthRecord {
  patientId: string;
  nationalId: string;
  insuranceStatus: string;
  insuranceType?: string;
  lastVisitDate?: string;
  chronicConditions?: string[];
  disabilityStatus?: string;
  disabilityType?: string;
  vaccinationStatus?: Record<string, boolean>;
  pregnancyStatus?: string;
  nutritionalStatus?: string;
}

interface EducationRecord {
  studentId: string;
  nationalId: string;
  enrollmentStatus: string;
  schoolId?: string;
  schoolName?: string;
  gradeLevel?: string;
  attendanceRate?: number;
  lastAttendanceDate?: string;
  scholarshipStatus?: string;
  mealProgramStatus?: string;
  specialNeeds: boolean;
}

interface TaxRecord {
  taxpayerId: string;
  nationalId: string;
  filingStatus: string;
  lastFilingYear?: number;
  declaredIncome?: number;
  taxBracket?: string;
  propertyOwnership: boolean;
  businessOwnership: boolean;
  formalEmployment: boolean;
}

interface LaborRecord {
  workerId: string;
  nationalId: string;
  employmentStatus: string;
  employerId?: string;
  employerName?: string;
  occupationType?: string;
  sectorOfEmployment?: string;
  contractType?: string;
  socialSecurityStatus?: string;
  lastContributionDate?: string;
  unemploymentBenefits: boolean;
}

interface CrossSectorProfile {
  beneficiaryId: string;
  nationalId: string;
  health?: HealthRecord;
  education?: EducationRecord[];
  tax?: TaxRecord;
  labor?: LaborRecord;
  lastUpdated: string;
  dataSources: string[];
  consentStatus: Record<SectorType, ConsentStatus>;
}

interface ConsentRecord {
  sector: SectorType;
  dataTypes: string[];
  purpose: string;
  status: ConsentStatus;
  grantedAt?: string;
  expiresAt?: string;
}

// Main Cross-Sector Profile Component
export function CrossSectorProfileView({ 
  beneficiaryId,
  nationalId,
}: { 
  beneficiaryId: string;
  nationalId: string;
}) {
  const [profile, setProfile] = useState<CrossSectorProfile | null>(null);
  const [consents, setConsents] = useState<ConsentRecord[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showConsentDialog, setShowConsentDialog] = useState(false);
  const [selectedSector, setSelectedSector] = useState<SectorType | null>(null);

  useEffect(() => {
    loadProfile();
    loadConsents();
  }, [beneficiaryId]);

  const loadProfile = async () => {
    try {
      const response = await fetch(`/api/interop/profile/${beneficiaryId}`);
      if (response.ok) {
        const data = await response.json();
        setProfile(data);
      }
    } catch (err) {
      setError('Failed to load cross-sector profile');
    } finally {
      setIsLoading(false);
    }
  };

  const loadConsents = async () => {
    try {
      const response = await fetch(`/api/interop/consents/${beneficiaryId}`);
      if (response.ok) {
        const data = await response.json();
        setConsents(data.consents || []);
      }
    } catch (err) {
      console.error('Failed to load consents:', err);
    }
  };

  const refreshProfile = async () => {
    setIsRefreshing(true);
    try {
      const response = await fetch(`/api/interop/profile/${beneficiaryId}/refresh`, {
        method: 'POST',
      });
      if (response.ok) {
        const data = await response.json();
        setProfile(data);
      }
    } catch (err) {
      setError('Failed to refresh profile');
    } finally {
      setIsRefreshing(false);
    }
  };

  const handleGrantConsent = async (sector: SectorType, dataTypes: string[]) => {
    try {
      const response = await fetch('/api/interop/consent/grant', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          beneficiaryId,
          sector,
          dataTypes,
          purpose: 'Social protection eligibility verification',
          expiresIn: '365d',
        }),
      });
      
      if (response.ok) {
        await loadConsents();
        await refreshProfile();
        setShowConsentDialog(false);
      }
    } catch (err) {
      setError('Failed to grant consent');
    }
  };

  const handleRevokeConsent = async (sector: SectorType) => {
    try {
      const response = await fetch('/api/interop/consent/revoke', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ beneficiaryId, sector }),
      });
      
      if (response.ok) {
        await loadConsents();
        setProfile(prev => prev ? {
          ...prev,
          consentStatus: { ...prev.consentStatus, [sector]: 'revoked' as ConsentStatus },
        } : null);
      }
    } catch (err) {
      setError('Failed to revoke consent');
    }
  };

  const getConsentForSector = (sector: SectorType) => {
    return consents.find(c => c.sector === sector);
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold flex items-center gap-2">
            <Link2 className="h-6 w-6 text-primary" />
            Cross-Sector Profile
          </h2>
          <p className="text-muted-foreground">
            Unified view of data from connected government systems
          </p>
        </div>
        <Button onClick={refreshProfile} disabled={isRefreshing}>
          <RefreshCw className={`h-4 w-4 mr-2 ${isRefreshing ? 'animate-spin' : ''}`} />
          Refresh Data
        </Button>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertTriangle className="h-4 w-4" />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* Data Sources Summary */}
      {profile && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Connected Data Sources</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-2">
              {profile.dataSources.map((source) => (
                <Badge key={source} variant="secondary" className="capitalize">
                  <Database className="h-3 w-3 mr-1" />
                  {source}
                </Badge>
              ))}
              {profile.dataSources.length === 0 && (
                <span className="text-sm text-muted-foreground">
                  No data sources connected. Grant consent to access sector data.
                </span>
              )}
            </div>
            {profile.lastUpdated && (
              <p className="text-xs text-muted-foreground mt-2">
                Last updated: {new Date(profile.lastUpdated).toLocaleString()}
              </p>
            )}
          </CardContent>
        </Card>
      )}

      {/* Sector Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {SECTORS.map((sector) => {
          const Icon = sector.icon;
          const consent = getConsentForSector(sector.id);
          const hasConsent = consent?.status === 'granted';
          const sectorData = profile?.[sector.id as keyof CrossSectorProfile];

          return (
            <Card key={sector.id} className={!hasConsent ? 'opacity-75' : ''}>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <CardTitle className="flex items-center gap-2 text-lg">
                    <Icon className={`h-5 w-5 ${sector.color}`} />
                    {sector.name}
                  </CardTitle>
                  <ConsentStatusBadge status={profile?.consentStatus[sector.id] || 'pending'} />
                </div>
                <CardDescription>{sector.description}</CardDescription>
              </CardHeader>

              <CardContent>
                {hasConsent && sectorData ? (
                  <SectorDataView sector={sector.id} data={sectorData} />
                ) : (
                  <div className="text-center py-6">
                    <Lock className="h-8 w-8 mx-auto text-muted-foreground mb-2" />
                    <p className="text-sm text-muted-foreground mb-4">
                      Consent required to access {sector.name.toLowerCase()} data
                    </p>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setSelectedSector(sector.id);
                        setShowConsentDialog(true);
                      }}
                    >
                      <Unlock className="h-4 w-4 mr-1" />
                      Grant Consent
                    </Button>
                  </div>
                )}
              </CardContent>

              {hasConsent && (
                <CardFooter className="pt-0">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-destructive"
                    onClick={() => handleRevokeConsent(sector.id)}
                  >
                    Revoke Consent
                  </Button>
                </CardFooter>
              )}
            </Card>
          );
        })}
      </div>

      {/* Consent Dialog */}
      <ConsentDialog
        open={showConsentDialog}
        onOpenChange={setShowConsentDialog}
        sector={selectedSector ? SECTORS.find(s => s.id === selectedSector) : null}
        onGrant={(dataTypes) => selectedSector && handleGrantConsent(selectedSector, dataTypes)}
      />
    </div>
  );
}

// Consent Status Badge
function ConsentStatusBadge({ status }: { status: ConsentStatus }) {
  const variants: Record<ConsentStatus, { variant: 'default' | 'secondary' | 'destructive' | 'outline'; icon: React.ReactNode }> = {
    granted: { variant: 'default', icon: <CheckCircle className="h-3 w-3 mr-1" /> },
    denied: { variant: 'destructive', icon: <XCircle className="h-3 w-3 mr-1" /> },
    pending: { variant: 'secondary', icon: <Clock className="h-3 w-3 mr-1" /> },
    revoked: { variant: 'outline', icon: <XCircle className="h-3 w-3 mr-1" /> },
    expired: { variant: 'outline', icon: <AlertTriangle className="h-3 w-3 mr-1" /> },
  };

  const { variant, icon } = variants[status];

  return (
    <Badge variant={variant} className="capitalize">
      {icon}
      {status}
    </Badge>
  );
}

// Sector Data View
function SectorDataView({ sector, data }: { sector: SectorType; data: unknown }) {
  switch (sector) {
    case 'health':
      return <HealthDataView data={data as HealthRecord} />;
    case 'education':
      return <EducationDataView data={data as EducationRecord[]} />;
    case 'tax':
      return <TaxDataView data={data as TaxRecord} />;
    case 'labor':
      return <LaborDataView data={data as LaborRecord} />;
    default:
      return <div>No data available</div>;
  }
}

function HealthDataView({ data }: { data: HealthRecord }) {
  return (
    <div className="space-y-3">
      <DataRow label="Insurance Status" value={data.insuranceStatus} />
      {data.insuranceType && <DataRow label="Insurance Type" value={data.insuranceType} />}
      {data.disabilityStatus && (
        <DataRow 
          label="Disability Status" 
          value={data.disabilityStatus}
          badge={data.disabilityType}
        />
      )}
      {data.chronicConditions && data.chronicConditions.length > 0 && (
        <div>
          <span className="text-sm text-muted-foreground">Chronic Conditions</span>
          <div className="flex flex-wrap gap-1 mt-1">
            {data.chronicConditions.map((condition, i) => (
              <Badge key={i} variant="outline" className="text-xs">
                {condition}
              </Badge>
            ))}
          </div>
        </div>
      )}
      {data.lastVisitDate && (
        <DataRow label="Last Visit" value={new Date(data.lastVisitDate).toLocaleDateString()} />
      )}
    </div>
  );
}

function EducationDataView({ data }: { data: EducationRecord[] }) {
  if (!data || data.length === 0) {
    return <p className="text-sm text-muted-foreground">No education records found</p>;
  }

  return (
    <div className="space-y-4">
      {data.map((record, i) => (
        <div key={i} className="p-3 bg-muted/50 rounded-lg space-y-2">
          <div className="flex items-center justify-between">
            <span className="font-medium">{record.schoolName || 'School'}</span>
            <Badge variant={record.enrollmentStatus === 'enrolled' ? 'default' : 'secondary'}>
              {record.enrollmentStatus}
            </Badge>
          </div>
          {record.gradeLevel && <DataRow label="Grade" value={record.gradeLevel} />}
          {record.attendanceRate !== undefined && (
            <div>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-muted-foreground">Attendance</span>
                <span>{(record.attendanceRate * 100).toFixed(0)}%</span>
              </div>
              <Progress value={record.attendanceRate * 100} className="h-2" />
            </div>
          )}
          {record.scholarshipStatus && (
            <DataRow label="Scholarship" value={record.scholarshipStatus} />
          )}
        </div>
      ))}
    </div>
  );
}

function TaxDataView({ data }: { data: TaxRecord }) {
  return (
    <div className="space-y-3">
      <DataRow label="Filing Status" value={data.filingStatus} />
      {data.lastFilingYear && <DataRow label="Last Filing Year" value={data.lastFilingYear.toString()} />}
      {data.taxBracket && <DataRow label="Tax Bracket" value={data.taxBracket} />}
      <div className="flex flex-wrap gap-2 mt-2">
        {data.propertyOwnership && (
          <Badge variant="outline">
            <Building2 className="h-3 w-3 mr-1" />
            Property Owner
          </Badge>
        )}
        {data.businessOwnership && (
          <Badge variant="outline">
            <Briefcase className="h-3 w-3 mr-1" />
            Business Owner
          </Badge>
        )}
        {data.formalEmployment && (
          <Badge variant="outline">
            <Users className="h-3 w-3 mr-1" />
            Formal Employment
          </Badge>
        )}
      </div>
    </div>
  );
}

function LaborDataView({ data }: { data: LaborRecord }) {
  return (
    <div className="space-y-3">
      <DataRow label="Employment Status" value={data.employmentStatus} />
      {data.employerName && <DataRow label="Employer" value={data.employerName} />}
      {data.occupationType && <DataRow label="Occupation" value={data.occupationType} />}
      {data.contractType && <DataRow label="Contract Type" value={data.contractType} />}
      {data.socialSecurityStatus && (
        <DataRow label="Social Security" value={data.socialSecurityStatus} />
      )}
      {data.unemploymentBenefits && (
        <Badge variant="secondary">
          Receiving Unemployment Benefits
        </Badge>
      )}
    </div>
  );
}

function DataRow({ label, value, badge }: { label: string; value: string; badge?: string }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-sm text-muted-foreground">{label}</span>
      <div className="flex items-center gap-2">
        <span className="text-sm font-medium capitalize">{value}</span>
        {badge && <Badge variant="outline" className="text-xs">{badge}</Badge>}
      </div>
    </div>
  );
}

// Consent Dialog
function ConsentDialog({
  open,
  onOpenChange,
  sector,
  onGrant,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sector: SectorInfo | null;
  onGrant: (dataTypes: string[]) => void;
}) {
  const [selectedDataTypes, setSelectedDataTypes] = useState<string[]>([]);
  const [acceptedTerms, setAcceptedTerms] = useState(false);

  useEffect(() => {
    if (sector) {
      setSelectedDataTypes(sector.dataTypes);
    }
  }, [sector]);

  const handleGrant = () => {
    if (acceptedTerms && selectedDataTypes.length > 0) {
      onGrant(selectedDataTypes);
      setAcceptedTerms(false);
    }
  };

  if (!sector) return null;

  const Icon = sector.icon;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Icon className={`h-5 w-5 ${sector.color}`} />
            Grant {sector.name} Data Access
          </DialogTitle>
          <DialogDescription>
            Allow the Social Protection Platform to access your {sector.name.toLowerCase()} data
            for eligibility verification.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-4">
          <div className="space-y-3">
            <Label>Data Types to Share</Label>
            {sector.dataTypes.map((dataType) => (
              <div key={dataType} className="flex items-center space-x-2">
                <Checkbox
                  id={dataType}
                  checked={selectedDataTypes.includes(dataType)}
                  onCheckedChange={(checked) => {
                    if (checked) {
                      setSelectedDataTypes([...selectedDataTypes, dataType]);
                    } else {
                      setSelectedDataTypes(selectedDataTypes.filter(t => t !== dataType));
                    }
                  }}
                />
                <Label htmlFor={dataType} className="text-sm font-normal">
                  {dataType}
                </Label>
              </div>
            ))}
          </div>

          <Alert>
            <Shield className="h-4 w-4" />
            <AlertTitle>Data Protection</AlertTitle>
            <AlertDescription className="text-xs">
              Your data will be used solely for eligibility verification and will be protected
              according to data protection regulations. You can revoke consent at any time.
            </AlertDescription>
          </Alert>

          <div className="flex items-start space-x-2">
            <Checkbox
              id="terms"
              checked={acceptedTerms}
              onCheckedChange={(checked) => setAcceptedTerms(checked as boolean)}
            />
            <Label htmlFor="terms" className="text-sm font-normal leading-tight">
              I understand and consent to sharing my {sector.name.toLowerCase()} data with the
              Social Protection Platform for the purpose of eligibility verification.
            </Label>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button 
            onClick={handleGrant}
            disabled={!acceptedTerms || selectedDataTypes.length === 0}
          >
            Grant Consent
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// Consent Management Page
export function ConsentManagement({ beneficiaryId }: { beneficiaryId: string }) {
  const [consents, setConsents] = useState<ConsentRecord[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    loadConsents();
  }, [beneficiaryId]);

  const loadConsents = async () => {
    try {
      const response = await fetch(`/api/interop/consents/${beneficiaryId}`);
      if (response.ok) {
        const data = await response.json();
        setConsents(data.consents || []);
      }
    } catch (err) {
      console.error('Failed to load consents:', err);
    } finally {
      setIsLoading(false);
    }
  };

  if (isLoading) {
    return <div className="text-center py-8">Loading consents...</div>;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Shield className="h-5 w-5" />
          Data Sharing Consents
        </CardTitle>
        <CardDescription>
          Manage your data sharing permissions across government sectors
        </CardDescription>
      </CardHeader>
      <CardContent>
        {consents.length === 0 ? (
          <p className="text-center text-muted-foreground py-4">
            No active consents
          </p>
        ) : (
          <div className="space-y-4">
            {consents.map((consent, i) => {
              const sector = SECTORS.find(s => s.id === consent.sector);
              const Icon = sector?.icon || FileText;

              return (
                <div 
                  key={i}
                  className="flex items-center justify-between p-4 border rounded-lg"
                >
                  <div className="flex items-center gap-3">
                    <Icon className={`h-5 w-5 ${sector?.color || 'text-muted-foreground'}`} />
                    <div>
                      <div className="font-medium">{sector?.name || consent.sector}</div>
                      <div className="text-sm text-muted-foreground">
                        {consent.dataTypes.join(', ')}
                      </div>
                      {consent.expiresAt && (
                        <div className="text-xs text-muted-foreground">
                          Expires: {new Date(consent.expiresAt).toLocaleDateString()}
                        </div>
                      )}
                    </div>
                  </div>
                  <ConsentStatusBadge status={consent.status} />
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export default CrossSectorProfileView;
