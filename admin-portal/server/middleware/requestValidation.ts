/**
 * Request Validation Middleware
 * 
 * Validates incoming requests on boundary endpoints to prevent
 * injection attacks and ensure data integrity.
 */

import { z } from "zod";
import { TRPCError } from "@trpc/server";

// Common validation schemas
export const uuidSchema = z.string().uuid("Invalid UUID format");
export const emailSchema = z.string().email("Invalid email format");
export const phoneSchema = z.string().regex(/^\+?[1-9]\d{1,14}$/, "Invalid phone number format");
export const dateSchema = z.string().datetime("Invalid date format");

// Beneficiary validation
export const beneficiaryCreateSchema = z.object({
  firstName: z.string().min(1).max(100).regex(/^[a-zA-Z\s\-']+$/, "Invalid characters in name"),
  lastName: z.string().min(1).max(100).regex(/^[a-zA-Z\s\-']+$/, "Invalid characters in name"),
  dateOfBirth: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, "Date must be YYYY-MM-DD"),
  nationalId: z.string().min(5).max(50).optional(),
  email: emailSchema.optional(),
  phone: phoneSchema.optional(),
  address: z.object({
    street: z.string().max(200).optional(),
    city: z.string().max(100).optional(),
    state: z.string().max(100).optional(),
    postalCode: z.string().max(20).optional(),
    country: z.string().length(2).optional(), // ISO 3166-1 alpha-2
  }).optional(),
  householdId: uuidSchema.optional(),
  programId: uuidSchema.optional(),
});

// National ID verification validation
export const nationalIdVerificationSchema = z.object({
  providerId: z.enum([
    "aadhaar", "nin", "nida", "nadra", "cpf", "rsa_id",
    "ghana_card", "huduma", "bvn", "nrc", "cni", "generic"
  ]),
  nationalId: z.string().min(5).max(50),
  consentToken: z.string().min(10),
  biometricData: z.object({
    type: z.enum(["fingerprint", "face", "iris"]),
    data: z.string().min(100), // Base64 encoded
    quality: z.number().min(0).max(100).optional(),
  }).optional(),
  demographicData: z.object({
    firstName: z.string().optional(),
    lastName: z.string().optional(),
    dateOfBirth: z.string().optional(),
    gender: z.enum(["M", "F", "O"]).optional(),
  }).optional(),
});

// PMT survey validation
export const pmtSurveySchema = z.object({
  householdId: uuidSchema,
  surveyDate: dateSchema,
  location: z.object({
    latitude: z.number().min(-90).max(90),
    longitude: z.number().min(-180).max(180),
    accuracy: z.number().min(0).optional(),
  }),
  housing: z.object({
    wallMaterial: z.string(),
    roofMaterial: z.string(),
    floorMaterial: z.string(),
    rooms: z.number().int().min(1).max(50),
    waterSource: z.string(),
    sanitationType: z.string(),
    cookingFuel: z.string(),
  }),
  assets: z.object({
    television: z.boolean(),
    refrigerator: z.boolean(),
    washingMachine: z.boolean(),
    computer: z.boolean(),
    mobilePhone: z.boolean(),
    motorcycle: z.boolean(),
    car: z.boolean(),
    bicycle: z.boolean(),
    livestock: z.number().int().min(0).optional(),
    landArea: z.number().min(0).optional(),
  }),
  demographics: z.object({
    householdSize: z.number().int().min(1).max(50),
    dependencyRatio: z.number().min(0).max(1),
    femaleHeaded: z.boolean(),
    elderlyPresent: z.boolean(),
    disabledPresent: z.boolean(),
    childrenUnder5: z.number().int().min(0),
    childrenSchoolAge: z.number().int().min(0),
  }),
  income: z.object({
    primarySource: z.string(),
    monthlyIncome: z.number().min(0).optional(),
    employmentStatus: z.string(),
  }),
});

// Disbursement validation
export const disbursementSchema = z.object({
  beneficiaryId: uuidSchema,
  programId: uuidSchema,
  amount: z.number().positive().max(1000000),
  currency: z.string().length(3), // ISO 4217
  paymentMethod: z.enum(["bank_transfer", "mobile_money", "cash", "voucher"]),
  idempotencyKey: z.string().uuid("Idempotency key must be a valid UUID"),
  metadata: z.record(z.string(), z.string()).optional(),
});

// Interoperability consent validation
export const interopConsentSchema = z.object({
  beneficiaryId: uuidSchema,
  sector: z.enum(["health", "education", "tax", "labor"]),
  consentType: z.enum(["read", "write", "full"]),
  validUntil: dateSchema,
  purpose: z.string().min(10).max(500),
});

// Sanitization functions
export function sanitizeString(input: string): string {
  return input
    .replace(/[<>]/g, "") // Remove angle brackets
    .replace(/javascript:/gi, "") // Remove javascript: protocol
    .replace(/on\w+=/gi, "") // Remove event handlers
    .trim();
}

export function sanitizeObject<T extends Record<string, unknown>>(obj: T): T {
  const sanitized: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(obj)) {
    if (typeof value === "string") {
      sanitized[key] = sanitizeString(value);
    } else if (typeof value === "object" && value !== null) {
      sanitized[key] = sanitizeObject(value as Record<string, unknown>);
    } else {
      sanitized[key] = value;
    }
  }
  return sanitized as T;
}

// Validation helper
export function validateInput<T>(schema: z.ZodSchema<T>, input: unknown): T {
  const result = schema.safeParse(input);
  if (!result.success) {
    const errors = result.error.issues.map((e: z.core.$ZodIssue) => `${e.path.join(".")}: ${e.message}`).join(", ");
    throw new TRPCError({
      code: "BAD_REQUEST",
      message: `Validation failed: ${errors}`,
    });
  }
  return result.data;
}

// Rate limiting check (works with Redis)
export async function checkRateLimit(
  key: string,
  limit: number,
  windowSeconds: number,
  redis?: { incr: (key: string) => Promise<number>; expire: (key: string, seconds: number) => Promise<void> }
): Promise<{ allowed: boolean; remaining: number; resetAt: Date }> {
  if (!redis) {
    // No Redis, allow all (development mode)
    return { allowed: true, remaining: limit, resetAt: new Date(Date.now() + windowSeconds * 1000) };
  }

  const count = await redis.incr(key);
  if (count === 1) {
    await redis.expire(key, windowSeconds);
  }

  return {
    allowed: count <= limit,
    remaining: Math.max(0, limit - count),
    resetAt: new Date(Date.now() + windowSeconds * 1000),
  };
}
