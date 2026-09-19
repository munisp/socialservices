import React, { useState } from 'react';
import {
  Home,
  Users,
  Briefcase,
  Car,
  Tv,
  Phone,
  Droplets,
  Zap,
  MapPin,
  Calculator,
  ChevronRight,
  ChevronLeft,
  CheckCircle,
  AlertTriangle,
  TrendingUp,
  TrendingDown,
  DollarSign,
  PieChart,
  BarChart3,
  Info,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Checkbox } from '@/components/ui/checkbox';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { Slider } from '@/components/ui/slider';
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
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
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

// PMT Survey Data Types
export interface HouseholdCharacteristics {
  householdId: string;
  beneficiaryId?: string;
  
  // Demographics
  householdSize: number;
  dependencyRatio: number;
  headAge: number;
  headGender: 'male' | 'female';
  headEducationYears: number;
  headEmploymentStatus: string;
  numChildrenUnder5: number;
  numChildren5to17: number;
  numElderlyOver65: number;
  numDisabledMembers: number;
  
  // Housing
  housingType: string;
  wallMaterial: string;
  roofMaterial: string;
  floorMaterial: string;
  numRooms: number;
  hasElectricity: boolean;
  hasPipedWater: boolean;
  hasFlushToilet: boolean;
  cookingFuel: string;
  
  // Assets
  ownsLand: boolean;
  landAreaHectares: number;
  ownsLivestock: boolean;
  livestockValueUsd: number;
  ownsVehicle: boolean;
  vehicleType?: string;
  ownsRefrigerator: boolean;
  ownsTelevision: boolean;
  ownsMobilePhone: boolean;
  ownsComputer: boolean;
  
  // Location
  region: string;
  district: string;
  urbanRural: 'urban' | 'rural' | 'peri-urban';
  
  // Additional
  hasBankAccount: boolean;
  receivesRemittances: boolean;
  hasHealthInsurance: boolean;
  childrenInSchool: boolean;
  foodSecurityScore?: number;
}

export interface PMTScore {
  householdId: string;
  pmtScore: number;
  estimatedConsumption: number;
  eligibilityCategory: 'extremely_poor' | 'poor' | 'vulnerable' | 'non_poor';
  confidence: number;
  componentScores: {
    housing: number;
    assets: number;
    demographics: number;
    humanCapital: number;
    location: number;
  };
  explanation: string[];
  calculatedAt: string;
}

export function calculatePMTScore(data: HouseholdCharacteristics): PMTScore {
  const housing = (Number(data.hasElectricity) + Number(data.hasPipedWater) + Number(data.hasFlushToilet) + (data.housingType === 'permanent' ? 2 : 0)) / 5;
  const assets = (Number(data.ownsLand) + Number(data.ownsLivestock) + Number(data.ownsVehicle) + Number(data.ownsRefrigerator) + Number(data.ownsComputer)) / 5;
  const demographics = Math.max(0, 1 - data.dependencyRatio / 3);
  const humanCapital = Math.min(1, data.headEducationYears / 16);
  const location = data.urbanRural === 'urban' ? 0.7 : data.urbanRural === 'peri-urban' ? 0.5 : 0.3;
  const pmtScore = Math.round((housing * .25 + assets * .2 + demographics * .25 + humanCapital * .15 + location * .15) * 1000) / 10;
  const eligibilityCategory = pmtScore < 20 ? 'extremely_poor' : pmtScore < 40 ? 'poor' : pmtScore < 60 ? 'vulnerable' : 'non_poor';
  return { householdId: data.householdId, pmtScore, estimatedConsumption: Math.round(pmtScore * 100) / 100, eligibilityCategory, confidence: .6, componentScores: { housing: housing * 100, assets: assets * 100, demographics: demographics * 100, humanCapital: humanCapital * 100, location: location * 100 }, explanation: ['Local screening score; verify with programme policy and human review.'], calculatedAt: new Date().toISOString() };
}

const HOUSING_TYPES = [
  { value: 'permanent', label: 'Permanent (Brick/Concrete)' },
  { value: 'semi_permanent', label: 'Semi-Permanent' },
  { value: 'temporary', label: 'Temporary/Makeshift' },
  { value: 'traditional', label: 'Traditional' },
];

const WALL_MATERIALS = [
  { value: 'brick_cement', label: 'Brick/Cement' },
  { value: 'stone', label: 'Stone' },
  { value: 'wood', label: 'Wood' },
  { value: 'mud', label: 'Mud/Earth' },
  { value: 'bamboo', label: 'Bamboo' },
  { value: 'metal', label: 'Metal Sheets' },
  { value: 'other', label: 'Other' },
];

const ROOF_MATERIALS = [
  { value: 'concrete', label: 'Concrete/RCC' },
  { value: 'tiles', label: 'Tiles' },
  { value: 'metal', label: 'Metal/Iron Sheets' },
  { value: 'asbestos', label: 'Asbestos' },
  { value: 'thatch', label: 'Thatch/Straw' },
  { value: 'plastic', label: 'Plastic' },
  { value: 'other', label: 'Other' },
];

const FLOOR_MATERIALS = [
  { value: 'tiles', label: 'Tiles/Marble' },
  { value: 'cement', label: 'Cement' },
  { value: 'wood', label: 'Wood' },
  { value: 'earth', label: 'Earth/Mud' },
  { value: 'other', label: 'Other' },
];

const COOKING_FUELS = [
  { value: 'lpg', label: 'LPG/Natural Gas' },
  { value: 'electricity', label: 'Electricity' },
  { value: 'kerosene', label: 'Kerosene' },
  { value: 'charcoal', label: 'Charcoal' },
  { value: 'wood', label: 'Wood/Firewood' },
  { value: 'dung', label: 'Animal Dung' },
  { value: 'crop_residue', label: 'Crop Residue' },
];

const EMPLOYMENT_STATUSES = [
  { value: 'formal_employed', label: 'Formally Employed' },
  { value: 'informal_employed', label: 'Informally Employed' },
  { value: 'self_employed', label: 'Self-Employed' },
  { value: 'unemployed', label: 'Unemployed' },
  { value: 'retired', label: 'Retired' },
  { value: 'student', label: 'Student' },
  { value: 'homemaker', label: 'Homemaker' },
  { value: 'disabled', label: 'Unable to Work' },
];

const VEHICLE_TYPES = [
  { value: 'car', label: 'Car' },
  { value: 'motorcycle', label: 'Motorcycle' },
  { value: 'bicycle', label: 'Bicycle' },
  { value: 'tractor', label: 'Tractor' },
  { value: 'boat', label: 'Boat' },
];

interface PMTSurveyFormProps {
  householdId: string;
  beneficiaryId?: string;
  initialData?: Partial<HouseholdCharacteristics>;
  onSubmit: (data: HouseholdCharacteristics) => Promise<PMTScore>;
  onCancel?: () => void;
}

export function PMTSurveyForm({ 
  householdId, 
  beneficiaryId,
  initialData,
  onSubmit,
  onCancel 
}: PMTSurveyFormProps) {
  const [currentStep, setCurrentStep] = useState(0);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [result, setResult] = useState<PMTScore | null>(null);
  const [error, setError] = useState<string | null>(null);

  const [formData, setFormData] = useState<HouseholdCharacteristics>({
    householdId,
    beneficiaryId,
    householdSize: initialData?.householdSize || 1,
    dependencyRatio: initialData?.dependencyRatio || 0,
    headAge: initialData?.headAge || 30,
    headGender: initialData?.headGender || 'male',
    headEducationYears: initialData?.headEducationYears || 0,
    headEmploymentStatus: initialData?.headEmploymentStatus || 'unemployed',
    numChildrenUnder5: initialData?.numChildrenUnder5 || 0,
    numChildren5to17: initialData?.numChildren5to17 || 0,
    numElderlyOver65: initialData?.numElderlyOver65 || 0,
    numDisabledMembers: initialData?.numDisabledMembers || 0,
    housingType: initialData?.housingType || 'temporary',
    wallMaterial: initialData?.wallMaterial || 'mud',
    roofMaterial: initialData?.roofMaterial || 'thatch',
    floorMaterial: initialData?.floorMaterial || 'earth',
    numRooms: initialData?.numRooms || 1,
    hasElectricity: initialData?.hasElectricity || false,
    hasPipedWater: initialData?.hasPipedWater || false,
    hasFlushToilet: initialData?.hasFlushToilet || false,
    cookingFuel: initialData?.cookingFuel || 'wood',
    ownsLand: initialData?.ownsLand || false,
    landAreaHectares: initialData?.landAreaHectares || 0,
    ownsLivestock: initialData?.ownsLivestock || false,
    livestockValueUsd: initialData?.livestockValueUsd || 0,
    ownsVehicle: initialData?.ownsVehicle || false,
    vehicleType: initialData?.vehicleType,
    ownsRefrigerator: initialData?.ownsRefrigerator || false,
    ownsTelevision: initialData?.ownsTelevision || false,
    ownsMobilePhone: initialData?.ownsMobilePhone || false,
    ownsComputer: initialData?.ownsComputer || false,
    region: initialData?.region || '',
    district: initialData?.district || '',
    urbanRural: initialData?.urbanRural || 'rural',
    hasBankAccount: initialData?.hasBankAccount || false,
    receivesRemittances: initialData?.receivesRemittances || false,
    hasHealthInsurance: initialData?.hasHealthInsurance || false,
    childrenInSchool: initialData?.childrenInSchool || true,
    foodSecurityScore: initialData?.foodSecurityScore,
  });

  const steps = [
    { id: 'demographics', title: 'Demographics', icon: Users },
    { id: 'housing', title: 'Housing', icon: Home },
    { id: 'assets', title: 'Assets', icon: Car },
    { id: 'location', title: 'Location', icon: MapPin },
    { id: 'additional', title: 'Additional', icon: Briefcase },
  ];

  const updateField = <K extends keyof HouseholdCharacteristics>(
    field: K, 
    value: HouseholdCharacteristics[K]
  ) => {
    setFormData(prev => ({ ...prev, [field]: value }));
  };

  const handleSubmit = async () => {
    setIsSubmitting(true);
    setError(null);
    
    try {
      // Calculate dependency ratio
      const dependents = formData.numChildrenUnder5 + formData.numChildren5to17 + formData.numElderlyOver65;
      const workingAge = formData.householdSize - dependents;
      formData.dependencyRatio = workingAge > 0 ? dependents / workingAge : 1;

      const score = await onSubmit(formData);
      setResult(score);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to calculate PMT score');
    } finally {
      setIsSubmitting(false);
    }
  };

  const canProceed = () => {
    switch (currentStep) {
      case 0: // Demographics
        return formData.householdSize >= 1 && formData.headAge >= 18;
      case 1: // Housing
        return formData.housingType && formData.numRooms >= 1;
      case 2: // Assets
        return true;
      case 3: // Location
        return formData.region && formData.district;
      case 4: // Additional
        return true;
      default:
        return true;
    }
  };

  if (result) {
    return <PMTScoreResult score={result} onReset={() => setResult(null)} />;
  }

  return (
    <Card className="w-full max-w-3xl mx-auto">
      <CardHeader>
        <div className="flex items-center gap-2">
          <Calculator className="h-6 w-6 text-primary" />
          <CardTitle>Proxy Means Test Survey</CardTitle>
        </div>
        <CardDescription>
          Complete the household assessment to calculate eligibility score
        </CardDescription>
      </CardHeader>

      <CardContent>
        {/* Progress Steps */}
        <div className="mb-8">
          <div className="flex justify-between mb-2">
            {steps.map((step, index) => {
              const Icon = step.icon;
              return (
                <div 
                  key={step.id}
                  className={`flex flex-col items-center ${
                    index <= currentStep ? 'text-primary' : 'text-muted-foreground'
                  }`}
                >
                  <div className={`w-10 h-10 rounded-full flex items-center justify-center mb-1 ${
                    index < currentStep ? 'bg-primary text-primary-foreground' :
                    index === currentStep ? 'bg-primary/20 border-2 border-primary' :
                    'bg-muted'
                  }`}>
                    {index < currentStep ? (
                      <CheckCircle className="h-5 w-5" />
                    ) : (
                      <Icon className="h-5 w-5" />
                    )}
                  </div>
                  <span className="text-xs hidden sm:block">{step.title}</span>
                </div>
              );
            })}
          </div>
          <Progress value={(currentStep / (steps.length - 1)) * 100} />
        </div>

        {error && (
          <Alert variant="destructive" className="mb-4">
            <AlertTriangle className="h-4 w-4" />
            <AlertTitle>Error</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        {/* Step 0: Demographics */}
        {currentStep === 0 && (
          <div className="space-y-6">
            <h3 className="text-lg font-medium flex items-center gap-2">
              <Users className="h-5 w-5" />
              Household Demographics
            </h3>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Household Size</Label>
                <Input
                  type="number"
                  min={1}
                  max={50}
                  value={formData.householdSize}
                  onChange={(e) => updateField('householdSize', parseInt(e.target.value) || 1)}
                />
              </div>

              <div className="space-y-2">
                <Label>Head of Household Age</Label>
                <Input
                  type="number"
                  min={18}
                  max={120}
                  value={formData.headAge}
                  onChange={(e) => updateField('headAge', parseInt(e.target.value) || 18)}
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label>Head of Household Gender</Label>
              <RadioGroup
                value={formData.headGender}
                onValueChange={(v) => updateField('headGender', v as 'male' | 'female')}
                className="flex gap-4"
              >
                <div className="flex items-center space-x-2">
                  <RadioGroupItem value="male" id="male" />
                  <Label htmlFor="male">Male</Label>
                </div>
                <div className="flex items-center space-x-2">
                  <RadioGroupItem value="female" id="female" />
                  <Label htmlFor="female">Female</Label>
                </div>
              </RadioGroup>
            </div>

            <div className="space-y-2">
              <Label>Head's Years of Education</Label>
              <Slider
                value={[formData.headEducationYears]}
                onValueChange={([v]) => updateField('headEducationYears', v)}
                max={25}
                step={1}
              />
              <div className="text-sm text-muted-foreground text-right">
                {formData.headEducationYears} years
              </div>
            </div>

            <div className="space-y-2">
              <Label>Employment Status</Label>
              <Select
                value={formData.headEmploymentStatus}
                onValueChange={(v) => updateField('headEmploymentStatus', v)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {EMPLOYMENT_STATUSES.map((status) => (
                    <SelectItem key={status.value} value={status.value}>
                      {status.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Children Under 5</Label>
                <Input
                  type="number"
                  min={0}
                  value={formData.numChildrenUnder5}
                  onChange={(e) => updateField('numChildrenUnder5', parseInt(e.target.value) || 0)}
                />
              </div>
              <div className="space-y-2">
                <Label>Children 5-17</Label>
                <Input
                  type="number"
                  min={0}
                  value={formData.numChildren5to17}
                  onChange={(e) => updateField('numChildren5to17', parseInt(e.target.value) || 0)}
                />
              </div>
              <div className="space-y-2">
                <Label>Elderly (65+)</Label>
                <Input
                  type="number"
                  min={0}
                  value={formData.numElderlyOver65}
                  onChange={(e) => updateField('numElderlyOver65', parseInt(e.target.value) || 0)}
                />
              </div>
              <div className="space-y-2">
                <Label>Disabled Members</Label>
                <Input
                  type="number"
                  min={0}
                  value={formData.numDisabledMembers}
                  onChange={(e) => updateField('numDisabledMembers', parseInt(e.target.value) || 0)}
                />
              </div>
            </div>
          </div>
        )}

        {/* Step 1: Housing */}
        {currentStep === 1 && (
          <div className="space-y-6">
            <h3 className="text-lg font-medium flex items-center gap-2">
              <Home className="h-5 w-5" />
              Housing Characteristics
            </h3>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Housing Type</Label>
                <Select
                  value={formData.housingType}
                  onValueChange={(v) => updateField('housingType', v)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {HOUSING_TYPES.map((type) => (
                      <SelectItem key={type.value} value={type.value}>
                        {type.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label>Number of Rooms</Label>
                <Input
                  type="number"
                  min={1}
                  max={20}
                  value={formData.numRooms}
                  onChange={(e) => updateField('numRooms', parseInt(e.target.value) || 1)}
                />
              </div>
            </div>

            <div className="grid grid-cols-3 gap-4">
              <div className="space-y-2">
                <Label>Wall Material</Label>
                <Select
                  value={formData.wallMaterial}
                  onValueChange={(v) => updateField('wallMaterial', v)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {WALL_MATERIALS.map((m) => (
                      <SelectItem key={m.value} value={m.value}>
                        {m.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label>Roof Material</Label>
                <Select
                  value={formData.roofMaterial}
                  onValueChange={(v) => updateField('roofMaterial', v)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {ROOF_MATERIALS.map((m) => (
                      <SelectItem key={m.value} value={m.value}>
                        {m.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label>Floor Material</Label>
                <Select
                  value={formData.floorMaterial}
                  onValueChange={(v) => updateField('floorMaterial', v)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {FLOOR_MATERIALS.map((m) => (
                      <SelectItem key={m.value} value={m.value}>
                        {m.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-2">
              <Label>Cooking Fuel</Label>
              <Select
                value={formData.cookingFuel}
                onValueChange={(v) => updateField('cookingFuel', v)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {COOKING_FUELS.map((fuel) => (
                    <SelectItem key={fuel.value} value={fuel.value}>
                      {fuel.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-4">
              <Label>Utilities & Amenities</Label>
              <div className="grid grid-cols-3 gap-4">
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="electricity"
                    checked={formData.hasElectricity}
                    onCheckedChange={(c) => updateField('hasElectricity', c as boolean)}
                  />
                  <Label htmlFor="electricity" className="flex items-center gap-1">
                    <Zap className="h-4 w-4" /> Electricity
                  </Label>
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="water"
                    checked={formData.hasPipedWater}
                    onCheckedChange={(c) => updateField('hasPipedWater', c as boolean)}
                  />
                  <Label htmlFor="water" className="flex items-center gap-1">
                    <Droplets className="h-4 w-4" /> Piped Water
                  </Label>
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="toilet"
                    checked={formData.hasFlushToilet}
                    onCheckedChange={(c) => updateField('hasFlushToilet', c as boolean)}
                  />
                  <Label htmlFor="toilet">Flush Toilet</Label>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Step 2: Assets */}
        {currentStep === 2 && (
          <div className="space-y-6">
            <h3 className="text-lg font-medium flex items-center gap-2">
              <Car className="h-5 w-5" />
              Household Assets
            </h3>

            <div className="space-y-4">
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="land"
                  checked={formData.ownsLand}
                  onCheckedChange={(c) => updateField('ownsLand', c as boolean)}
                />
                <Label htmlFor="land">Owns Agricultural Land</Label>
              </div>
              {formData.ownsLand && (
                <div className="ml-6 space-y-2">
                  <Label>Land Area (hectares)</Label>
                  <Input
                    type="number"
                    min={0}
                    step={0.1}
                    value={formData.landAreaHectares}
                    onChange={(e) => updateField('landAreaHectares', parseFloat(e.target.value) || 0)}
                  />
                </div>
              )}
            </div>

            <div className="space-y-4">
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="livestock"
                  checked={formData.ownsLivestock}
                  onCheckedChange={(c) => updateField('ownsLivestock', c as boolean)}
                />
                <Label htmlFor="livestock">Owns Livestock</Label>
              </div>
              {formData.ownsLivestock && (
                <div className="ml-6 space-y-2">
                  <Label>Estimated Livestock Value (USD)</Label>
                  <Input
                    type="number"
                    min={0}
                    value={formData.livestockValueUsd}
                    onChange={(e) => updateField('livestockValueUsd', parseFloat(e.target.value) || 0)}
                  />
                </div>
              )}
            </div>

            <div className="space-y-4">
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="vehicle"
                  checked={formData.ownsVehicle}
                  onCheckedChange={(c) => updateField('ownsVehicle', c as boolean)}
                />
                <Label htmlFor="vehicle">Owns Vehicle</Label>
              </div>
              {formData.ownsVehicle && (
                <div className="ml-6 space-y-2">
                  <Label>Vehicle Type</Label>
                  <Select
                    value={formData.vehicleType}
                    onValueChange={(v) => updateField('vehicleType', v)}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {VEHICLE_TYPES.map((v) => (
                        <SelectItem key={v.value} value={v.value}>
                          {v.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}
            </div>

            <div className="space-y-4">
              <Label>Household Appliances</Label>
              <div className="grid grid-cols-2 gap-4">
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="refrigerator"
                    checked={formData.ownsRefrigerator}
                    onCheckedChange={(c) => updateField('ownsRefrigerator', c as boolean)}
                  />
                  <Label htmlFor="refrigerator">Refrigerator</Label>
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="television"
                    checked={formData.ownsTelevision}
                    onCheckedChange={(c) => updateField('ownsTelevision', c as boolean)}
                  />
                  <Label htmlFor="television" className="flex items-center gap-1">
                    <Tv className="h-4 w-4" /> Television
                  </Label>
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="mobile"
                    checked={formData.ownsMobilePhone}
                    onCheckedChange={(c) => updateField('ownsMobilePhone', c as boolean)}
                  />
                  <Label htmlFor="mobile" className="flex items-center gap-1">
                    <Phone className="h-4 w-4" /> Mobile Phone
                  </Label>
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="computer"
                    checked={formData.ownsComputer}
                    onCheckedChange={(c) => updateField('ownsComputer', c as boolean)}
                  />
                  <Label htmlFor="computer">Computer/Laptop</Label>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Step 3: Location */}
        {currentStep === 3 && (
          <div className="space-y-6">
            <h3 className="text-lg font-medium flex items-center gap-2">
              <MapPin className="h-5 w-5" />
              Geographic Location
            </h3>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Region/State</Label>
                <Input
                  value={formData.region}
                  onChange={(e) => updateField('region', e.target.value)}
                  placeholder="Enter region"
                />
              </div>
              <div className="space-y-2">
                <Label>District</Label>
                <Input
                  value={formData.district}
                  onChange={(e) => updateField('district', e.target.value)}
                  placeholder="Enter district"
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label>Area Type</Label>
              <RadioGroup
                value={formData.urbanRural}
                onValueChange={(v) => updateField('urbanRural', v as 'urban' | 'rural' | 'peri-urban')}
                className="flex gap-4"
              >
                <div className="flex items-center space-x-2">
                  <RadioGroupItem value="urban" id="urban" />
                  <Label htmlFor="urban">Urban</Label>
                </div>
                <div className="flex items-center space-x-2">
                  <RadioGroupItem value="peri-urban" id="peri-urban" />
                  <Label htmlFor="peri-urban">Peri-Urban</Label>
                </div>
                <div className="flex items-center space-x-2">
                  <RadioGroupItem value="rural" id="rural" />
                  <Label htmlFor="rural">Rural</Label>
                </div>
              </RadioGroup>
            </div>
          </div>
        )}

        {/* Step 4: Additional */}
        {currentStep === 4 && (
          <div className="space-y-6">
            <h3 className="text-lg font-medium flex items-center gap-2">
              <Briefcase className="h-5 w-5" />
              Additional Information
            </h3>

            <div className="space-y-4">
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="bank"
                  checked={formData.hasBankAccount}
                  onCheckedChange={(c) => updateField('hasBankAccount', c as boolean)}
                />
                <Label htmlFor="bank">Has Bank Account</Label>
              </div>
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="remittances"
                  checked={formData.receivesRemittances}
                  onCheckedChange={(c) => updateField('receivesRemittances', c as boolean)}
                />
                <Label htmlFor="remittances">Receives Remittances</Label>
              </div>
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="insurance"
                  checked={formData.hasHealthInsurance}
                  onCheckedChange={(c) => updateField('hasHealthInsurance', c as boolean)}
                />
                <Label htmlFor="insurance">Has Health Insurance</Label>
              </div>
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="school"
                  checked={formData.childrenInSchool}
                  onCheckedChange={(c) => updateField('childrenInSchool', c as boolean)}
                />
                <Label htmlFor="school">Children Attending School</Label>
              </div>
            </div>

            <div className="space-y-2">
              <Label>Food Security Score (0-10)</Label>
              <Slider
                value={[formData.foodSecurityScore || 5]}
                onValueChange={([v]) => updateField('foodSecurityScore', v)}
                max={10}
                step={1}
              />
              <div className="flex justify-between text-xs text-muted-foreground">
                <span>Food Insecure</span>
                <span>{formData.foodSecurityScore || 5}</span>
                <span>Food Secure</span>
              </div>
            </div>
          </div>
        )}
      </CardContent>

      <CardFooter className="flex justify-between">
        <Button
          variant="outline"
          onClick={() => currentStep > 0 ? setCurrentStep(currentStep - 1) : onCancel?.()}
        >
          <ChevronLeft className="h-4 w-4 mr-1" />
          {currentStep > 0 ? 'Previous' : 'Cancel'}
        </Button>

        {currentStep < steps.length - 1 ? (
          <Button
            onClick={() => setCurrentStep(currentStep + 1)}
            disabled={!canProceed()}
          >
            Next
            <ChevronRight className="h-4 w-4 ml-1" />
          </Button>
        ) : (
          <Button onClick={handleSubmit} disabled={isSubmitting}>
            {isSubmitting ? 'Calculating...' : 'Calculate PMT Score'}
            <Calculator className="h-4 w-4 ml-1" />
          </Button>
        )}
      </CardFooter>
    </Card>
  );
}

// PMT Score Result Component
export function PMTScoreResult({ score, onReset }: { score: PMTScore; onReset?: () => void }) {
  const categoryColors = {
    extremely_poor: 'bg-red-500',
    poor: 'bg-orange-500',
    vulnerable: 'bg-yellow-500',
    non_poor: 'bg-green-500',
  };

  const categoryLabels = {
    extremely_poor: 'Extremely Poor',
    poor: 'Poor',
    vulnerable: 'Vulnerable',
    non_poor: 'Non-Poor',
  };

  return (
    <Card className="w-full max-w-3xl mx-auto">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <PieChart className="h-6 w-6 text-primary" />
          PMT Assessment Results
        </CardTitle>
        <CardDescription>
          Calculated on {new Date(score.calculatedAt).toLocaleString()}
        </CardDescription>
      </CardHeader>

      <CardContent className="space-y-6">
        {/* Main Score */}
        <div className="text-center p-6 bg-muted rounded-lg">
          <div className="text-5xl font-bold text-primary mb-2">
            {score.pmtScore.toFixed(1)}
          </div>
          <div className="text-sm text-muted-foreground mb-4">PMT Score</div>
          <Badge className={`${categoryColors[score.eligibilityCategory]} text-white text-lg px-4 py-1`}>
            {categoryLabels[score.eligibilityCategory]}
          </Badge>
        </div>

        {/* Estimated Consumption */}
        <div className="flex items-center justify-between p-4 bg-muted/50 rounded-lg">
          <div className="flex items-center gap-2">
            <DollarSign className="h-5 w-5 text-muted-foreground" />
            <span>Estimated Daily Consumption</span>
          </div>
          <span className="font-bold text-lg">
            ${score.estimatedConsumption.toFixed(2)}/day
          </span>
        </div>

        {/* Confidence */}
        <div className="space-y-2">
          <div className="flex justify-between text-sm">
            <span>Confidence Level</span>
            <span>{(score.confidence * 100).toFixed(0)}%</span>
          </div>
          <Progress value={score.confidence * 100} />
        </div>

        {/* Component Scores */}
        <div className="space-y-4">
          <h4 className="font-medium flex items-center gap-2">
            <BarChart3 className="h-5 w-5" />
            Component Breakdown
          </h4>
          
          <div className="space-y-3">
            {Object.entries(score.componentScores).map(([key, value]) => (
              <div key={key} className="space-y-1">
                <div className="flex justify-between text-sm">
                  <span className="capitalize">{key.replace(/([A-Z])/g, ' $1')}</span>
                  <span>{(value * 100).toFixed(0)}%</span>
                </div>
                <Progress value={value * 100} className="h-2" />
              </div>
            ))}
          </div>
        </div>

        {/* Explanation */}
        <div className="space-y-2">
          <h4 className="font-medium flex items-center gap-2">
            <Info className="h-5 w-5" />
            Assessment Factors
          </h4>
          <ul className="space-y-1 text-sm text-muted-foreground">
            {score.explanation.map((item, i) => (
              <li key={i} className="flex items-start gap-2">
                <span className="text-primary">•</span>
                {item}
              </li>
            ))}
          </ul>
        </div>
      </CardContent>

      <CardFooter>
        <Button onClick={onReset} className="w-full">
          Start New Assessment
        </Button>
      </CardFooter>
    </Card>
  );
}

export default PMTSurveyForm;
