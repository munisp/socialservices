/**
 * APISIX API Gateway Integration
 * 
 * APISIX is deployed as an external service and configured via Admin API.
 * This file provides helper functions to configure routes, rate limiting, and authentication.
 * 
 * APISIX Configuration:
 * - Admin API: http://localhost:9180
 * - Gateway: http://localhost:9080
 * - Dashboard: http://localhost:9000
 */

interface ApisixRoute {
  id?: string;
  name: string;
  uri: string;
  methods?: string[];
  upstream: {
    type: "roundrobin" | "chash" | "ewma" | "least_conn";
    nodes: Record<string, number>; // { "host:port": weight }
  };
  plugins?: Record<string, any>;
}

interface RateLimitConfig {
  count: number;
  time_window: number;
  key?: "remote_addr" | "consumer_name" | "server_addr";
  rejected_code?: number;
  rejected_msg?: string;
}

/**
 * APISIX Admin API client
 */
class ApisixClient {
  private adminUrl: string;
  private adminKey: string;

  constructor() {
    this.adminUrl = process.env.APISIX_ADMIN_URL || "http://localhost:9180";
    const adminKey = process.env.APISIX_ADMIN_KEY;
    if (!adminKey && process.env.NODE_ENV === "production") {
      throw new Error("APISIX_ADMIN_KEY must be configured in production");
    }
    this.adminKey = adminKey || ""; // Empty key will cause auth failures, which is safer than a default
  }

  private async request(method: string, path: string, body?: any): Promise<any> {
    const url = `${this.adminUrl}${path}`;

    try {
      const response = await fetch(url, {
        method,
        headers: {
          "X-API-KEY": this.adminKey,
          "Content-Type": "application/json",
        },
        body: body ? JSON.stringify(body) : undefined,
      });

      if (!response.ok) {
        const error = await response.text();
        throw new Error(`APISIX API error: ${response.status} ${error}`);
      }

      return response.json();
    } catch (error) {
      console.error(`[APISIX] Request failed: ${method} ${path}`, error);
      throw error;
    }
  }

  /**
   * Create or update route
   */
  async createRoute(route: ApisixRoute): Promise<any> {
    const path = route.id ? `/apisix/admin/routes/${route.id}` : "/apisix/admin/routes";
    const method = route.id ? "PUT" : "POST";

    return this.request(method, path, route);
  }

  /**
   * Delete route
   */
  async deleteRoute(id: string): Promise<any> {
    return this.request("DELETE", `/apisix/admin/routes/${id}`);
  }

  /**
   * List all routes
   */
  async listRoutes(): Promise<any> {
    return this.request("GET", "/apisix/admin/routes");
  }

  /**
   * Create consumer (for authentication)
   */
  async createConsumer(username: string, plugins: Record<string, any>): Promise<any> {
    return this.request("PUT", `/apisix/admin/consumers/${username}`, { plugins });
  }

  /**
   * Health check
   */
  async healthCheck(): Promise<boolean> {
    try {
      await this.request("GET", "/apisix/admin/routes");
      return true;
    } catch (error) {
      return false;
    }
  }
}

/**
 * Initialize APISIX routes for the platform
 */
export async function initializeApisixRoutes(): Promise<void> {
  const client = new ApisixClient();

  const backendHost = process.env.BACKEND_HOST || "localhost:3000";

  // Main tRPC API route
  const trpcRoute: ApisixRoute = {
    id: "trpc-api",
    name: "tRPC API",
    uri: "/api/trpc/*",
    methods: ["GET", "POST", "OPTIONS"],
    upstream: {
      type: "roundrobin",
      nodes: {
        [backendHost]: 1,
      },
    },
    plugins: {
      // CORS - restrict to configured origins in production
      cors: {
        allow_origins: process.env.CORS_ALLOWED_ORIGINS || (process.env.NODE_ENV === "production" 
          ? "https://admin.socialprotection.gov" 
          : "http://localhost:3000,http://localhost:5173"),
        allow_methods: "GET,POST,OPTIONS",
        allow_headers: "Content-Type,Authorization,X-Correlation-ID,X-Request-ID",
        allow_credential: true,
        max_age: 3600,
      },
      // Rate limiting: 1000 requests per minute per IP
      "limit-count": {
        count: 1000,
        time_window: 60,
        key: "remote_addr",
        rejected_code: 429,
        rejected_msg: "Too many requests",
      },
      // Request/response logging
      "http-logger": {
        uri: "http://localhost:9200/apisix/logs",
        batch_max_size: 100,
        inactive_timeout: 5,
      },
    },
  };

  // OAuth callback route
  const oauthRoute: ApisixRoute = {
    id: "oauth-callback",
    name: "OAuth Callback",
    uri: "/api/oauth/*",
    methods: ["GET", "POST"],
    upstream: {
      type: "roundrobin",
      nodes: {
        [backendHost]: 1,
      },
    },
    plugins: {
      cors: {
        allow_origins: process.env.CORS_ALLOWED_ORIGINS || (process.env.NODE_ENV === "production" 
          ? "https://admin.socialprotection.gov" 
          : "http://localhost:3000,http://localhost:5173"),
        allow_methods: "GET,POST",
        allow_headers: "Content-Type",
        allow_credential: true,
      },
    },
  };

  // Static assets route
  const staticRoute: ApisixRoute = {
    id: "static-assets",
    name: "Static Assets",
    uri: "/assets/*",
    methods: ["GET"],
    upstream: {
      type: "roundrobin",
      nodes: {
        [backendHost]: 1,
      },
    },
    plugins: {
      // Cache static assets for 1 hour
      "proxy-cache": {
        cache_ttl: 3600,
        cache_bypass: ["$arg_nocache"],
      },
    },
  };

  // Mojaloop callback routes - these receive async callbacks from Mojaloop hub
  const orchestratorHost = process.env.ORCHESTRATOR_HOST || "localhost:8080";
  
  // Mojaloop parties callback route (party lookup responses)
  const mojaloopPartiesRoute: ApisixRoute = {
    id: "mojaloop-parties",
    name: "Mojaloop Parties Callback",
    uri: "/parties/*",
    methods: ["PUT"],
    upstream: {
      type: "roundrobin",
      nodes: {
        [orchestratorHost]: 1,
      },
    },
    plugins: {
      // Rate limiting: 10000 requests per minute (high volume for callbacks)
      "limit-count": {
        count: 10000,
        time_window: 60,
        key: "remote_addr",
        rejected_code: 429,
        rejected_msg: "Too many requests",
      },
      // Request logging for audit
      "http-logger": {
        uri: process.env.LOG_COLLECTOR_URL || "http://localhost:9200/mojaloop/parties",
        batch_max_size: 100,
        inactive_timeout: 5,
      },
      // JWS signature verification (Mojaloop security requirement)
      // Note: In production, configure with actual Mojaloop hub public keys
      ...(process.env.MOJALOOP_JWS_PUBLIC_KEY ? {
        "jwt-auth": {
          key: process.env.MOJALOOP_JWS_PUBLIC_KEY,
        },
      } : {}),
    },
  };

  // Mojaloop quotes callback route (quote responses)
  const mojaloopQuotesRoute: ApisixRoute = {
    id: "mojaloop-quotes",
    name: "Mojaloop Quotes Callback",
    uri: "/quotes/*",
    methods: ["PUT"],
    upstream: {
      type: "roundrobin",
      nodes: {
        [orchestratorHost]: 1,
      },
    },
    plugins: {
      "limit-count": {
        count: 10000,
        time_window: 60,
        key: "remote_addr",
        rejected_code: 429,
      },
      "http-logger": {
        uri: process.env.LOG_COLLECTOR_URL || "http://localhost:9200/mojaloop/quotes",
        batch_max_size: 100,
        inactive_timeout: 5,
      },
    },
  };

  // Mojaloop transfers callback route (transfer fulfilment responses)
  const mojaloopTransfersRoute: ApisixRoute = {
    id: "mojaloop-transfers",
    name: "Mojaloop Transfers Callback",
    uri: "/transfers/*",
    methods: ["PUT"],
    upstream: {
      type: "roundrobin",
      nodes: {
        [orchestratorHost]: 1,
      },
    },
    plugins: {
      "limit-count": {
        count: 10000,
        time_window: 60,
        key: "remote_addr",
        rejected_code: 429,
      },
      "http-logger": {
        uri: process.env.LOG_COLLECTOR_URL || "http://localhost:9200/mojaloop/transfers",
        batch_max_size: 100,
        inactive_timeout: 5,
      },
    },
  };

  // Mojaloop bulk quotes callback route
  const mojaloopBulkQuotesRoute: ApisixRoute = {
    id: "mojaloop-bulk-quotes",
    name: "Mojaloop Bulk Quotes Callback",
    uri: "/bulkQuotes/*",
    methods: ["PUT"],
    upstream: {
      type: "roundrobin",
      nodes: {
        [orchestratorHost]: 1,
      },
    },
    plugins: {
      "limit-count": {
        count: 5000,
        time_window: 60,
        key: "remote_addr",
        rejected_code: 429,
      },
    },
  };

  // Mojaloop bulk transfers callback route
  const mojaloopBulkTransfersRoute: ApisixRoute = {
    id: "mojaloop-bulk-transfers",
    name: "Mojaloop Bulk Transfers Callback",
    uri: "/bulkTransfers/*",
    methods: ["PUT"],
    upstream: {
      type: "roundrobin",
      nodes: {
        [orchestratorHost]: 1,
      },
    },
    plugins: {
      "limit-count": {
        count: 5000,
        time_window: 60,
        key: "remote_addr",
        rejected_code: 429,
      },
    },
  };

  // Payment API routes (disbursement, settlement)
  const paymentApiRoute: ApisixRoute = {
    id: "payment-api",
    name: "Payment API",
    uri: "/api/payments/*",
    methods: ["GET", "POST", "PUT"],
    upstream: {
      type: "roundrobin",
      nodes: {
        [orchestratorHost]: 1,
      },
    },
    plugins: {
      // CORS for admin portal
      cors: {
        allow_origins: process.env.CORS_ALLOWED_ORIGINS || (process.env.NODE_ENV === "production" 
          ? "https://admin.socialprotection.gov" 
          : "http://localhost:3000,http://localhost:5173"),
        allow_methods: "GET,POST,PUT,OPTIONS",
        allow_headers: "Content-Type,Authorization,X-Correlation-ID,X-Idempotency-Key",
        allow_credential: true,
        max_age: 3600,
      },
      // Rate limiting: 100 requests per minute per IP (payment operations are sensitive)
      "limit-count": {
        count: 100,
        time_window: 60,
        key: "remote_addr",
        rejected_code: 429,
        rejected_msg: "Too many payment requests",
      },
      // Require authentication via Keycloak
      ...(process.env.KEYCLOAK_REALM ? {
        "openid-connect": {
          discovery: `${process.env.KEYCLOAK_URL || "http://localhost:8180"}/realms/${process.env.KEYCLOAK_REALM}/.well-known/openid-configuration`,
          client_id: process.env.KEYCLOAK_CLIENT_ID || "social-protection-api",
          client_secret: process.env.KEYCLOAK_CLIENT_SECRET,
          bearer_only: true,
          realm: process.env.KEYCLOAK_REALM,
        },
      } : {}),
    },
  };

  // Settlement API routes (Treasury/CBN reporting)
  const settlementApiRoute: ApisixRoute = {
    id: "settlement-api",
    name: "Settlement API",
    uri: "/api/settlements/*",
    methods: ["GET", "POST"],
    upstream: {
      type: "roundrobin",
      nodes: {
        [orchestratorHost]: 1,
      },
    },
    plugins: {
      cors: {
        allow_origins: process.env.CORS_ALLOWED_ORIGINS || (process.env.NODE_ENV === "production" 
          ? "https://admin.socialprotection.gov" 
          : "http://localhost:3000,http://localhost:5173"),
        allow_methods: "GET,POST,OPTIONS",
        allow_headers: "Content-Type,Authorization,X-Correlation-ID",
        allow_credential: true,
        max_age: 3600,
      },
      // Rate limiting: 50 requests per minute (settlement reports are heavy)
      "limit-count": {
        count: 50,
        time_window: 60,
        key: "remote_addr",
        rejected_code: 429,
      },
      // Require authentication
      ...(process.env.KEYCLOAK_REALM ? {
        "openid-connect": {
          discovery: `${process.env.KEYCLOAK_URL || "http://localhost:8180"}/realms/${process.env.KEYCLOAK_REALM}/.well-known/openid-configuration`,
          client_id: process.env.KEYCLOAK_CLIENT_ID || "social-protection-api",
          client_secret: process.env.KEYCLOAK_CLIENT_SECRET,
          bearer_only: true,
          realm: process.env.KEYCLOAK_REALM,
        },
      } : {}),
    },
  };

  try {
    console.log("[APISIX] Initializing routes...");

    await client.createRoute(trpcRoute);
    console.log("[APISIX] Created tRPC API route");

    await client.createRoute(oauthRoute);
    console.log("[APISIX] Created OAuth callback route");

    await client.createRoute(staticRoute);
    console.log("[APISIX] Created static assets route");

    // Mojaloop callback routes
    await client.createRoute(mojaloopPartiesRoute);
    console.log("[APISIX] Created Mojaloop parties callback route");

    await client.createRoute(mojaloopQuotesRoute);
    console.log("[APISIX] Created Mojaloop quotes callback route");

    await client.createRoute(mojaloopTransfersRoute);
    console.log("[APISIX] Created Mojaloop transfers callback route");

    await client.createRoute(mojaloopBulkQuotesRoute);
    console.log("[APISIX] Created Mojaloop bulk quotes callback route");

    await client.createRoute(mojaloopBulkTransfersRoute);
    console.log("[APISIX] Created Mojaloop bulk transfers callback route");

    // Payment and settlement API routes
    await client.createRoute(paymentApiRoute);
    console.log("[APISIX] Created payment API route");

    await client.createRoute(settlementApiRoute);
    console.log("[APISIX] Created settlement API route");

    console.log("[APISIX] All routes initialized successfully");
  } catch (error) {
    console.error("[APISIX] Failed to initialize routes:", error);
  }
}

/**
 * Create rate limit for specific endpoint
 */
export async function createRateLimit(
  routeId: string,
  config: RateLimitConfig
): Promise<void> {
  const client = new ApisixClient();

  try {
    const routes = await client.listRoutes();
    const route = routes.list?.find((r: any) => r.value.id === routeId);

    if (!route) {
      throw new Error(`Route ${routeId} not found`);
    }

    const updatedRoute = {
      ...route.value,
      plugins: {
        ...route.value.plugins,
        "limit-count": config,
      },
    };

    await client.createRoute(updatedRoute);
    console.log(`[APISIX] Rate limit configured for route ${routeId}`);
  } catch (error) {
    console.error(`[APISIX] Failed to create rate limit for ${routeId}:`, error);
  }
}

/**
 * Add circuit breaker to route
 */
export async function addCircuitBreaker(
  routeId: string,
  config: {
    break_response_code: number;
    max_breaker_sec: number;
    unhealthy: {
      http_statuses: number[];
      failures: number;
    };
    healthy: {
      http_statuses: number[];
      successes: number;
    };
  }
): Promise<void> {
  const client = new ApisixClient();

  try {
    const routes = await client.listRoutes();
    const route = routes.list?.find((r: any) => r.value.id === routeId);

    if (!route) {
      throw new Error(`Route ${routeId} not found`);
    }

    const updatedRoute = {
      ...route.value,
      plugins: {
        ...route.value.plugins,
        "api-breaker": config,
      },
    };

    await client.createRoute(updatedRoute);
    console.log(`[APISIX] Circuit breaker configured for route ${routeId}`);
  } catch (error) {
    console.error(`[APISIX] Failed to add circuit breaker to ${routeId}:`, error);
  }
}

/**
 * Health check
 */
export async function apisixHealthCheck(): Promise<boolean> {
  const client = new ApisixClient();
  return client.healthCheck();
}

/**
 * APISIX configuration for Docker Compose
 */
export const apisixDockerConfig = `
version: "3.8"

services:
  apisix:
    image: apache/apisix:3.8.0-debian
    container_name: apisix
    restart: always
    volumes:
      - ./apisix_conf/config.yaml:/usr/local/apisix/conf/config.yaml:ro
    depends_on:
      - etcd
    ports:
      - "9080:9080"   # Gateway
      - "9180:9180"   # Admin API
      - "9443:9443"   # HTTPS Gateway
    networks:
      - apisix

  etcd:
    image: bitnami/etcd:3.5.12
    container_name: etcd
    restart: always
    environment:
      ETCD_ENABLE_V2: "true"
      ALLOW_NONE_AUTHENTICATION: "yes"
      ETCD_ADVERTISE_CLIENT_URLS: "http://etcd:2379"
      ETCD_LISTEN_CLIENT_URLS: "http://0.0.0.0:2379"
    ports:
      - "2379:2379"
    networks:
      - apisix

  apisix-dashboard:
    image: apache/apisix-dashboard:3.0.1-alpine
    container_name: apisix-dashboard
    restart: always
    ports:
      - "9000:9000"
    networks:
      - apisix

networks:
  apisix:
    driver: bridge
`;

// Auto-initialize routes if APISIX is configured
if (process.env.APISIX_ADMIN_URL) {
  initializeApisixRoutes().catch((error) => {
    console.error("[APISIX] Auto-initialization failed:", error);
  });
}
