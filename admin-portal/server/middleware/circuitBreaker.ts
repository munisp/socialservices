/**
 * Circuit Breaker Pattern Implementation
 * Prevents cascading failures when external services are unavailable
 */

export enum CircuitState {
  CLOSED = "CLOSED",     // Normal operation
  OPEN = "OPEN",         // Failing, reject requests
  HALF_OPEN = "HALF_OPEN" // Testing if service recovered
}

interface CircuitBreakerConfig {
  name: string;
  failureThreshold: number;      // Number of failures before opening
  successThreshold: number;      // Number of successes in half-open to close
  timeout: number;               // Time in ms before trying half-open
  requestTimeout: number;        // Timeout for individual requests
  volumeThreshold: number;       // Minimum requests before calculating failure rate
  failureRateThreshold: number;  // Percentage of failures to trip (0-100)
}

interface CircuitBreakerStats {
  state: CircuitState;
  failures: number;
  successes: number;
  lastFailureTime: number | null;
  lastSuccessTime: number | null;
  totalRequests: number;
  consecutiveFailures: number;
  consecutiveSuccesses: number;
}

// Default configurations for different service types
export const CIRCUIT_BREAKER_CONFIGS: Record<string, Partial<CircuitBreakerConfig>> = {
  keycloak: {
    failureThreshold: 5,
    successThreshold: 3,
    timeout: 30000,
    requestTimeout: 10000,
    volumeThreshold: 10,
    failureRateThreshold: 50,
  },
  permify: {
    failureThreshold: 5,
    successThreshold: 3,
    timeout: 30000,
    requestTimeout: 5000,
    volumeThreshold: 10,
    failureRateThreshold: 50,
  },
  kafka: {
    failureThreshold: 3,
    successThreshold: 2,
    timeout: 60000,
    requestTimeout: 30000,
    volumeThreshold: 5,
    failureRateThreshold: 60,
  },
  redis: {
    failureThreshold: 3,
    successThreshold: 2,
    timeout: 15000,
    requestTimeout: 5000,
    volumeThreshold: 5,
    failureRateThreshold: 50,
  },
  temporal: {
    failureThreshold: 5,
    successThreshold: 3,
    timeout: 60000,
    requestTimeout: 30000,
    volumeThreshold: 10,
    failureRateThreshold: 50,
  },
  tigerbeetle: {
    failureThreshold: 3,
    successThreshold: 2,
    timeout: 30000,
    requestTimeout: 10000,
    volumeThreshold: 5,
    failureRateThreshold: 40,
  },
  mlService: {
    failureThreshold: 5,
    successThreshold: 3,
    timeout: 60000,
    requestTimeout: 30000,
    volumeThreshold: 10,
    failureRateThreshold: 60,
  },
  smsProvider: {
    failureThreshold: 5,
    successThreshold: 3,
    timeout: 60000,
    requestTimeout: 15000,
    volumeThreshold: 10,
    failureRateThreshold: 50,
  },
  emailProvider: {
    failureThreshold: 5,
    successThreshold: 3,
    timeout: 60000,
    requestTimeout: 15000,
    volumeThreshold: 10,
    failureRateThreshold: 50,
  },
  lakehouse: {
    failureThreshold: 5,
    successThreshold: 3,
    timeout: 120000,
    requestTimeout: 60000,
    volumeThreshold: 5,
    failureRateThreshold: 50,
  },
};

class CircuitBreaker {
  private config: CircuitBreakerConfig;
  private stats: CircuitBreakerStats;
  private stateChangeListeners: Array<(state: CircuitState, name: string) => void> = [];

  constructor(config: CircuitBreakerConfig) {
    this.config = config;
    this.stats = {
      state: CircuitState.CLOSED,
      failures: 0,
      successes: 0,
      lastFailureTime: null,
      lastSuccessTime: null,
      totalRequests: 0,
      consecutiveFailures: 0,
      consecutiveSuccesses: 0,
    };
  }

  /**
   * Execute a function with circuit breaker protection
   */
  async execute<T>(fn: () => Promise<T>, fallback?: () => T | Promise<T>): Promise<T> {
    // Check if circuit should transition from OPEN to HALF_OPEN
    if (this.stats.state === CircuitState.OPEN) {
      if (this.shouldAttemptReset()) {
        this.transitionTo(CircuitState.HALF_OPEN);
      } else {
        if (fallback) {
          console.warn(`[CircuitBreaker:${this.config.name}] Circuit OPEN, using fallback`);
          return fallback();
        }
        throw new CircuitBreakerOpenError(
          `Circuit breaker ${this.config.name} is OPEN`,
          this.getTimeUntilRetry()
        );
      }
    }

    this.stats.totalRequests++;

    try {
      // Execute with timeout
      const result = await this.executeWithTimeout(fn);
      this.onSuccess();
      return result;
    } catch (error) {
      this.onFailure(error);
      
      if (fallback && this.stats.state === CircuitState.OPEN) {
        console.warn(`[CircuitBreaker:${this.config.name}] Using fallback after failure`);
        return fallback();
      }
      
      throw error;
    }
  }

  /**
   * Execute function with timeout
   */
  private async executeWithTimeout<T>(fn: () => Promise<T>): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      const timeoutId = setTimeout(() => {
        reject(new Error(`Request timeout after ${this.config.requestTimeout}ms`));
      }, this.config.requestTimeout);

      fn()
        .then((result) => {
          clearTimeout(timeoutId);
          resolve(result);
        })
        .catch((error) => {
          clearTimeout(timeoutId);
          reject(error);
        });
    });
  }

  /**
   * Handle successful request
   */
  private onSuccess(): void {
    this.stats.successes++;
    this.stats.consecutiveSuccesses++;
    this.stats.consecutiveFailures = 0;
    this.stats.lastSuccessTime = Date.now();

    if (this.stats.state === CircuitState.HALF_OPEN) {
      if (this.stats.consecutiveSuccesses >= this.config.successThreshold) {
        this.transitionTo(CircuitState.CLOSED);
        this.resetStats();
      }
    }

    console.debug(`[CircuitBreaker:${this.config.name}] Success (consecutive: ${this.stats.consecutiveSuccesses})`);
  }

  /**
   * Handle failed request
   */
  private onFailure(error: unknown): void {
    this.stats.failures++;
    this.stats.consecutiveFailures++;
    this.stats.consecutiveSuccesses = 0;
    this.stats.lastFailureTime = Date.now();

    console.warn(`[CircuitBreaker:${this.config.name}] Failure (consecutive: ${this.stats.consecutiveFailures}):`, error);

    if (this.stats.state === CircuitState.HALF_OPEN) {
      // Any failure in half-open immediately opens the circuit
      this.transitionTo(CircuitState.OPEN);
    } else if (this.stats.state === CircuitState.CLOSED) {
      // Check if we should open the circuit
      if (this.shouldTrip()) {
        this.transitionTo(CircuitState.OPEN);
      }
    }
  }

  /**
   * Check if circuit should trip open
   */
  private shouldTrip(): boolean {
    // Check consecutive failures
    if (this.stats.consecutiveFailures >= this.config.failureThreshold) {
      return true;
    }

    // Check failure rate if we have enough requests
    if (this.stats.totalRequests >= this.config.volumeThreshold) {
      const failureRate = (this.stats.failures / this.stats.totalRequests) * 100;
      if (failureRate >= this.config.failureRateThreshold) {
        return true;
      }
    }

    return false;
  }

  /**
   * Check if we should attempt to reset (transition to half-open)
   */
  private shouldAttemptReset(): boolean {
    if (!this.stats.lastFailureTime) {
      return true;
    }
    return Date.now() - this.stats.lastFailureTime >= this.config.timeout;
  }

  /**
   * Get time until retry is allowed
   */
  private getTimeUntilRetry(): number {
    if (!this.stats.lastFailureTime) {
      return 0;
    }
    const elapsed = Date.now() - this.stats.lastFailureTime;
    return Math.max(0, this.config.timeout - elapsed);
  }

  /**
   * Transition to a new state
   */
  private transitionTo(newState: CircuitState): void {
    const oldState = this.stats.state;
    this.stats.state = newState;
    
    console.info(`[CircuitBreaker:${this.config.name}] State transition: ${oldState} -> ${newState}`);
    
    // Notify listeners
    this.stateChangeListeners.forEach(listener => {
      try {
        listener(newState, this.config.name);
      } catch (error) {
        console.error(`[CircuitBreaker:${this.config.name}] Listener error:`, error);
      }
    });
  }

  /**
   * Reset statistics
   */
  private resetStats(): void {
    this.stats.failures = 0;
    this.stats.successes = 0;
    this.stats.totalRequests = 0;
    this.stats.consecutiveFailures = 0;
    this.stats.consecutiveSuccesses = 0;
  }

  /**
   * Get current state
   */
  getState(): CircuitState {
    return this.stats.state;
  }

  /**
   * Get current statistics
   */
  getStats(): CircuitBreakerStats {
    return { ...this.stats };
  }

  /**
   * Add state change listener
   */
  onStateChange(listener: (state: CircuitState, name: string) => void): void {
    this.stateChangeListeners.push(listener);
  }

  /**
   * Force circuit to open (for testing or manual intervention)
   */
  forceOpen(): void {
    this.transitionTo(CircuitState.OPEN);
    this.stats.lastFailureTime = Date.now();
  }

  /**
   * Force circuit to close (for testing or manual intervention)
   */
  forceClose(): void {
    this.transitionTo(CircuitState.CLOSED);
    this.resetStats();
  }

  /**
   * Get health status
   */
  getHealth(): { healthy: boolean; state: CircuitState; failureRate: number } {
    const failureRate = this.stats.totalRequests > 0
      ? (this.stats.failures / this.stats.totalRequests) * 100
      : 0;

    return {
      healthy: this.stats.state === CircuitState.CLOSED,
      state: this.stats.state,
      failureRate,
    };
  }
}

/**
 * Custom error for circuit breaker open state
 */
export class CircuitBreakerOpenError extends Error {
  public retryAfter: number;

  constructor(message: string, retryAfter: number) {
    super(message);
    this.name = "CircuitBreakerOpenError";
    this.retryAfter = retryAfter;
  }
}

// Circuit breaker registry
const circuitBreakers = new Map<string, CircuitBreaker>();

/**
 * Get or create a circuit breaker for a service
 */
export function getCircuitBreaker(serviceName: string): CircuitBreaker {
  if (!circuitBreakers.has(serviceName)) {
    const defaultConfig: CircuitBreakerConfig = {
      name: serviceName,
      failureThreshold: 5,
      successThreshold: 3,
      timeout: 30000,
      requestTimeout: 10000,
      volumeThreshold: 10,
      failureRateThreshold: 50,
    };

    const serviceConfig = CIRCUIT_BREAKER_CONFIGS[serviceName] || {};
    const config = { ...defaultConfig, ...serviceConfig, name: serviceName };

    circuitBreakers.set(serviceName, new CircuitBreaker(config));
  }

  return circuitBreakers.get(serviceName)!;
}

/**
 * Execute a function with circuit breaker protection
 */
export async function withCircuitBreaker<T>(
  serviceName: string,
  fn: () => Promise<T>,
  fallback?: () => T | Promise<T>
): Promise<T> {
  const breaker = getCircuitBreaker(serviceName);
  return breaker.execute(fn, fallback);
}

/**
 * Get health status of all circuit breakers
 */
export function getAllCircuitBreakerHealth(): Record<string, { healthy: boolean; state: CircuitState; failureRate: number }> {
  const health: Record<string, { healthy: boolean; state: CircuitState; failureRate: number }> = {};
  
  for (const [name, breaker] of circuitBreakers) {
    health[name] = breaker.getHealth();
  }
  
  return health;
}

/**
 * Register a global state change listener
 */
export function onCircuitBreakerStateChange(
  listener: (state: CircuitState, serviceName: string) => void
): void {
  for (const breaker of circuitBreakers.values()) {
    breaker.onStateChange(listener);
  }
}

/**
 * Reset all circuit breakers (for testing)
 */
export function resetAllCircuitBreakers(): void {
  for (const breaker of circuitBreakers.values()) {
    breaker.forceClose();
  }
}

/**
 * Wrapper for fetch with circuit breaker
 */
export async function fetchWithCircuitBreaker(
  serviceName: string,
  url: string,
  options?: RequestInit,
  fallbackResponse?: Response
): Promise<Response> {
  return withCircuitBreaker(
    serviceName,
    async () => {
      const response = await fetch(url, options);
      if (!response.ok && response.status >= 500) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      return response;
    },
    fallbackResponse ? () => fallbackResponse : undefined
  );
}

export { CircuitBreaker };
