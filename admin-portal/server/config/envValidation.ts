/**
 * Environment Variable Validation
 * 
 * Validates required environment variables at startup and provides
 * clear error messages for missing configuration.
 */

interface EnvConfig {
  name: string;
  required: boolean;
  requiredInProd: boolean;
  description: string;
  defaultValue?: string;
}

const ENV_CONFIGS: EnvConfig[] = [
  // Database
  { name: "DATABASE_URL", required: true, requiredInProd: true, description: "PostgreSQL connection string" },
  
  // Redis
  { name: "REDIS_URL", required: false, requiredInProd: true, description: "Redis connection URL", defaultValue: "redis://localhost:6379" },
  
  // Authentication
  { name: "KEYCLOAK_URL", required: false, requiredInProd: true, description: "Keycloak server URL" },
  { name: "KEYCLOAK_REALM", required: false, requiredInProd: true, description: "Keycloak realm name" },
  { name: "KEYCLOAK_CLIENT_ID", required: false, requiredInProd: true, description: "Keycloak client ID" },
  { name: "KEYCLOAK_CLIENT_SECRET", required: false, requiredInProd: true, description: "Keycloak client secret" },
  
  // Authorization
  { name: "PERMIFY_URL", required: false, requiredInProd: true, description: "Permify authorization service URL" },
  
  // API Gateway
  { name: "APISIX_ADMIN_URL", required: false, requiredInProd: false, description: "APISIX admin API URL" },
  { name: "APISIX_ADMIN_KEY", required: false, requiredInProd: true, description: "APISIX admin API key" },
  
  // Message Queue
  { name: "KAFKA_BROKERS", required: false, requiredInProd: true, description: "Kafka broker addresses" },
  
  // Workflow Engine
  { name: "TEMPORAL_URL", required: false, requiredInProd: true, description: "Temporal server URL" },
  { name: "TEMPORAL_NAMESPACE", required: false, requiredInProd: true, description: "Temporal namespace" },
  
  // Go Orchestrator
  { name: "GO_ORCHESTRATOR_URL", required: false, requiredInProd: true, description: "Go orchestrator API URL" },
  
  // PMT Service
  { name: "PMT_SERVICE_URL", required: false, requiredInProd: true, description: "PMT calculation service URL" },
  
  // Session
  { name: "SESSION_SECRET", required: false, requiredInProd: true, description: "Session encryption secret (min 32 chars)" },
  
  // CSRF
  { name: "CSRF_SECRET", required: false, requiredInProd: true, description: "CSRF token secret" },
];

export interface ValidationResult {
  valid: boolean;
  errors: string[];
  warnings: string[];
  config: Record<string, string | undefined>;
}

/**
 * Validate environment variables
 */
export function validateEnvironment(): ValidationResult {
  const isProduction = process.env.NODE_ENV === "production";
  const errors: string[] = [];
  const warnings: string[] = [];
  const config: Record<string, string | undefined> = {};

  for (const envConfig of ENV_CONFIGS) {
    const value = process.env[envConfig.name];
    config[envConfig.name] = value || envConfig.defaultValue;

    if (!value) {
      if (envConfig.required) {
        errors.push(`Missing required environment variable: ${envConfig.name} - ${envConfig.description}`);
      } else if (isProduction && envConfig.requiredInProd) {
        errors.push(`Missing production-required environment variable: ${envConfig.name} - ${envConfig.description}`);
      } else if (envConfig.requiredInProd) {
        warnings.push(`Environment variable not set (required in production): ${envConfig.name} - ${envConfig.description}`);
      }
    }
  }

  // Additional validation rules
  if (process.env.SESSION_SECRET && process.env.SESSION_SECRET.length < 32) {
    errors.push("SESSION_SECRET must be at least 32 characters long");
  }

  return {
    valid: errors.length === 0,
    errors,
    warnings,
    config,
  };
}

/**
 * Validate and fail fast if configuration is invalid
 */
export function validateOrFail(): void {
  const result = validateEnvironment();

  // Log warnings
  for (const warning of result.warnings) {
    console.warn(`[Config] WARNING: ${warning}`);
  }

  // Fail on errors
  if (!result.valid) {
    console.error("[Config] Environment validation failed:");
    for (const error of result.errors) {
      console.error(`  - ${error}`);
    }
    
    if (process.env.NODE_ENV === "production") {
      process.exit(1);
    } else {
      console.warn("[Config] Continuing in development mode despite configuration errors");
    }
  } else {
    console.log("[Config] Environment validation passed");
  }
}

/**
 * Get validated config value with type safety
 */
export function getConfig(name: string): string {
  const value = process.env[name];
  const config = ENV_CONFIGS.find(c => c.name === name);
  
  if (!value && config?.defaultValue) {
    return config.defaultValue;
  }
  
  if (!value) {
    throw new Error(`Configuration not available: ${name}`);
  }
  
  return value;
}

/**
 * Get optional config value
 */
export function getOptionalConfig(name: string, defaultValue?: string): string | undefined {
  return process.env[name] || defaultValue;
}
