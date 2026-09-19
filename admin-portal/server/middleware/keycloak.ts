/**
 * Keycloak SSO Integration
 * Enterprise identity and access management with OAuth2/OIDC
 */

interface KeycloakConfig {
  realm: string;
  authServerUrl: string;
  sslRequired: string;
  resource: string;
  credentials: {
    secret: string;
  };
  confidentialPort: number;
}

interface KeycloakUser {
  sub: string; // Subject (user ID)
  email?: string;
  name?: string;
  preferred_username?: string;
  given_name?: string;
  family_name?: string;
  roles?: string[];
}

interface KeycloakToken {
  access_token: string;
  refresh_token?: string;
  expires_in: number;
  refresh_expires_in?: number;
  token_type: string;
}

/**
 * Keycloak client configuration
 */
export function getKeycloakConfig(): KeycloakConfig | null {
  const keycloakUrl = process.env.KEYCLOAK_URL;
  const keycloakRealm = process.env.KEYCLOAK_REALM || "social-protection";
  const keycloakClientId = process.env.KEYCLOAK_CLIENT_ID || "admin-portal";
  const keycloakClientSecret = process.env.KEYCLOAK_CLIENT_SECRET;

  if (!keycloakUrl || !keycloakClientSecret) {
    console.warn("[Keycloak] KEYCLOAK_URL or KEYCLOAK_CLIENT_SECRET not configured, SSO disabled");
    return null;
  }

  return {
    realm: keycloakRealm,
    authServerUrl: keycloakUrl,
    sslRequired: "external",
    resource: keycloakClientId,
    credentials: {
      secret: keycloakClientSecret,
    },
    confidentialPort: 0,
  };
}

/**
 * Get Keycloak login URL
 */
export function getKeycloakLoginUrl(redirectUri: string): string {
  const config = getKeycloakConfig();
  if (!config) {
    throw new Error("Keycloak not configured");
  }

  const params = new URLSearchParams({
    client_id: config.resource,
    redirect_uri: redirectUri,
    response_type: "code",
    scope: "openid profile email",
  });

  return `${config.authServerUrl}/realms/${config.realm}/protocol/openid-connect/auth?${params}`;
}

/**
 * Exchange authorization code for tokens
 */
export async function exchangeCodeForToken(code: string, redirectUri: string): Promise<KeycloakToken> {
  const config = getKeycloakConfig();
  if (!config) {
    throw new Error("Keycloak not configured");
  }

  const tokenUrl = `${config.authServerUrl}/realms/${config.realm}/protocol/openid-connect/token`;

  const params = new URLSearchParams({
    grant_type: "authorization_code",
    code,
    redirect_uri: redirectUri,
    client_id: config.resource,
    client_secret: config.credentials.secret,
  });

  try {
    const response = await fetch(tokenUrl, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body: params,
    });

    if (!response.ok) {
      const error = await response.text();
      throw new Error(`Token exchange failed: ${error}`);
    }

    return response.json();
  } catch (error) {
    console.error("[Keycloak] Token exchange error:", error);
    throw error;
  }
}

/**
 * Verify and decode access token
 */
export async function verifyToken(accessToken: string): Promise<KeycloakUser> {
  const config = getKeycloakConfig();
  if (!config) {
    throw new Error("Keycloak not configured");
  }

  const userInfoUrl = `${config.authServerUrl}/realms/${config.realm}/protocol/openid-connect/userinfo`;

  try {
    const response = await fetch(userInfoUrl, {
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
    });

    if (!response.ok) {
      throw new Error("Token verification failed");
    }

    const userInfo: KeycloakUser = await response.json();

    // Get user roles from token
    const roles = await getUserRoles(accessToken);
    userInfo.roles = roles;

    return userInfo;
  } catch (error) {
    console.error("[Keycloak] Token verification error:", error);
    throw error;
  }
}

/**
 * Get user roles from token
 */
async function getUserRoles(accessToken: string): Promise<string[]> {
  try {
    // Decode JWT token (simple base64 decode, no verification here as we already verified)
    const parts = accessToken.split(".");
    if (parts.length !== 3) {
      return [];
    }

    const payload = JSON.parse(Buffer.from(parts[1], "base64").toString());

    // Extract roles from realm_access and resource_access
    const realmRoles = payload.realm_access?.roles || [];
    const config = getKeycloakConfig();
    const clientRoles = config ? payload.resource_access?.[config.resource]?.roles || [] : [];

    return [...realmRoles, ...clientRoles];
  } catch (error) {
    console.error("[Keycloak] Failed to extract roles:", error);
    return [];
  }
}

/**
 * Refresh access token
 */
export async function refreshAccessToken(refreshToken: string): Promise<KeycloakToken> {
  const config = getKeycloakConfig();
  if (!config) {
    throw new Error("Keycloak not configured");
  }

  const tokenUrl = `${config.authServerUrl}/realms/${config.realm}/protocol/openid-connect/token`;

  const params = new URLSearchParams({
    grant_type: "refresh_token",
    refresh_token: refreshToken,
    client_id: config.resource,
    client_secret: config.credentials.secret,
  });

  try {
    const response = await fetch(tokenUrl, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body: params,
    });

    if (!response.ok) {
      throw new Error("Token refresh failed");
    }

    return response.json();
  } catch (error) {
    console.error("[Keycloak] Token refresh error:", error);
    throw error;
  }
}

/**
 * Logout user
 */
export async function logoutUser(refreshToken: string): Promise<void> {
  const config = getKeycloakConfig();
  if (!config) {
    return;
  }

  const logoutUrl = `${config.authServerUrl}/realms/${config.realm}/protocol/openid-connect/logout`;

  const params = new URLSearchParams({
    client_id: config.resource,
    client_secret: config.credentials.secret,
    refresh_token: refreshToken,
  });

  try {
    await fetch(logoutUrl, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body: params,
    });

    console.log("[Keycloak] User logged out");
  } catch (error) {
    console.error("[Keycloak] Logout error:", error);
  }
}

/**
 * Create user in Keycloak
 */
export async function createKeycloakUser(user: {
  username: string;
  email: string;
  firstName?: string;
  lastName?: string;
  enabled?: boolean;
}): Promise<string> {
  const config = getKeycloakConfig();
  if (!config) {
    throw new Error("Keycloak not configured");
  }

  // Get admin token first
  const adminToken = await getAdminToken();

  const createUserUrl = `${config.authServerUrl}/admin/realms/${config.realm}/users`;

  try {
    const response = await fetch(createUserUrl, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${adminToken}`,
      },
      body: JSON.stringify({
        username: user.username,
        email: user.email,
        firstName: user.firstName,
        lastName: user.lastName,
        enabled: user.enabled ?? true,
        emailVerified: false,
      }),
    });

    if (!response.ok) {
      const error = await response.text();
      throw new Error(`Failed to create user: ${error}`);
    }

    // Extract user ID from Location header
    const location = response.headers.get("Location");
    const userId = location?.split("/").pop() || "";

    console.log(`[Keycloak] User created: ${userId}`);
    return userId;
  } catch (error) {
    console.error("[Keycloak] User creation error:", error);
    throw error;
  }
}

/**
 * Get admin access token
 */
async function getAdminToken(): Promise<string> {
  const config = getKeycloakConfig();
  if (!config) {
    throw new Error("Keycloak not configured");
  }

  const tokenUrl = `${config.authServerUrl}/realms/${config.realm}/protocol/openid-connect/token`;

  const params = new URLSearchParams({
    grant_type: "client_credentials",
    client_id: config.resource,
    client_secret: config.credentials.secret,
  });

  try {
    const response = await fetch(tokenUrl, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body: params,
    });

    if (!response.ok) {
      throw new Error("Failed to get admin token");
    }

    const data: KeycloakToken = await response.json();
    return data.access_token;
  } catch (error) {
    console.error("[Keycloak] Admin token error:", error);
    throw error;
  }
}

/**
 * Assign role to user
 */
export async function assignRoleToUser(userId: string, roleName: string): Promise<void> {
  const config = getKeycloakConfig();
  if (!config) {
    throw new Error("Keycloak not configured");
  }

  const adminToken = await getAdminToken();

  // Get role ID
  const rolesUrl = `${config.authServerUrl}/admin/realms/${config.realm}/roles/${roleName}`;
  const roleResponse = await fetch(rolesUrl, {
    headers: {
      Authorization: `Bearer ${adminToken}`,
    },
  });

  if (!roleResponse.ok) {
    throw new Error(`Role ${roleName} not found`);
  }

  const role = await roleResponse.json();

  // Assign role to user
  const assignUrl = `${config.authServerUrl}/admin/realms/${config.realm}/users/${userId}/role-mappings/realm`;

  try {
    await fetch(assignUrl, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${adminToken}`,
      },
      body: JSON.stringify([role]),
    });

    console.log(`[Keycloak] Role ${roleName} assigned to user ${userId}`);
  } catch (error) {
    console.error("[Keycloak] Role assignment error:", error);
    throw error;
  }
}

/**
 * Health check
 */
export async function keycloakHealthCheck(): Promise<boolean> {
  const config = getKeycloakConfig();
  if (!config) {
    return false;
  }

  try {
    const response = await fetch(`${config.authServerUrl}/realms/${config.realm}`);
    return response.ok;
  } catch (error) {
    console.error("[Keycloak] Health check failed:", error);
    return false;
  }
}
