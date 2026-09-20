// @ts-ignore - Permify types not fully compatible
import { PermifyClient } from "@permify/permify-node";

/**
 * Permify Authorization Integration
 * Fine-grained, relationship-based access control (ReBAC)
 */

let permifyClient: PermifyClient | null = null;

/**
 * Get Permify client instance
 */
export function getPermifyClient(): PermifyClient | null {
  if (permifyClient) {
    return permifyClient;
  }

  const permifyUrl = process.env.PERMIFY_URL;
  if (!permifyUrl) {
    console.warn("[Permify] PERMIFY_URL not configured, authorization disabled");
    return null;
  }

  try {
    permifyClient = new PermifyClient({
      endpoint: permifyUrl,
    });

    console.log("[Permify] Client initialized");
    return permifyClient;
  } catch (error) {
    console.error("[Permify] Failed to initialize client:", error);
    return null;
  }
}

/**
 * Authorization schema for the platform
 * 
 * Entities: user, beneficiary, program, transaction, document
 * Relations: owner, editor, viewer, member
 * Actions: view, edit, delete, approve, manage
 */
export const PERMIFY_SCHEMA = `
entity user {}

entity beneficiary {
  relation owner @user
  relation editor @user
  relation viewer @user
  
  action view = viewer or editor or owner
  action edit = editor or owner
  action delete = owner
  action approve = owner
}

entity program {
  relation admin @user
  relation manager @user
  relation viewer @user
  
  action view = viewer or manager or admin
  action edit = manager or admin
  action delete = admin
  action manage_beneficiaries = manager or admin
  action configure = admin
}

entity transaction {
  relation beneficiary_owner @user
  relation program_admin @user
  relation auditor @user
  
  action view = beneficiary_owner or program_admin or auditor
  action approve = program_admin
  action audit = auditor
}

entity document {
  relation owner @user
  relation shared_with @user
  
  action view = owner or shared_with
  action edit = owner
  action delete = owner
  action share = owner
}

entity organization {
  relation admin @user
  relation member @user
  
  action view = member or admin
  action manage = admin
  action invite_member = admin
}
`;

/**
 * Write authorization schema to Permify
 */
export async function writeAuthorizationSchema(): Promise<void> {
  const client = getPermifyClient();
  if (!client) {
    if (process.env.NODE_ENV === "production") throw new Error("Permify is required in production");
    console.warn("[Permify] Client not available, skipping development schema write");
    return;
  }

  try {
    await client.schema.write({
      tenantId: "default",
      schema: PERMIFY_SCHEMA,
    });

    console.log("[Permify] Authorization schema written successfully");
  } catch (error) {
    console.error("[Permify] Failed to write schema:", error);
    throw error;
  }
}

/**
 * Create relationship (tuple)
 */
export async function createRelationship(params: {
  entity: string;
  entityId: string;
  relation: string;
  subject: string;
  subjectId: string;
}): Promise<void> {
  const client = getPermifyClient();
  if (!client) {
    if (process.env.NODE_ENV === "production") throw new Error("Permify relationship write refused: service unavailable");
    console.warn("[Permify] Client not available, skipping development relationship creation");
    return;
  }

  try {
    await client.data.write({
      tenantId: "default",
      metadata: {
        schemaVersion: "",
      },
      tuples: [
        {
          entity: {
            type: params.entity,
            id: params.entityId,
          },
          relation: params.relation,
          subject: {
            type: params.subject,
            id: params.subjectId,
          },
        },
      ],
    });

    console.log(
      `[Permify] Relationship created: ${params.entity}:${params.entityId}#${params.relation}@${params.subject}:${params.subjectId}`
    );
  } catch (error) {
    console.error("[Permify] Failed to create relationship:", error);
    throw error;
  }
}

/**
 * Delete relationship
 */
export async function deleteRelationship(params: {
  entity: string;
  entityId: string;
  relation: string;
  subject: string;
  subjectId: string;
}): Promise<void> {
  const client = getPermifyClient();
  if (!client) {
    if (process.env.NODE_ENV === "production") throw new Error("Permify relationship delete refused: service unavailable");
    return;
  }

  try {
    await client.data.delete({
      tenantId: "default",
      tupleFilter: {
        entity: {
          type: params.entity,
          ids: [params.entityId],
        },
        relation: params.relation,
        subject: {
          type: params.subject,
          ids: [params.subjectId],
        },
      },
    });

    console.log(`[Permify] Relationship deleted: ${params.entity}:${params.entityId}#${params.relation}`);
  } catch (error) {
    console.error("[Permify] Failed to delete relationship:", error);
    throw error;
  }
}

/**
 * Check permission
 */
export async function checkPermission(params: {
  entity: string;
  entityId: string;
  action: string;
  userId: string;
  fallbackToRBAC?: boolean;
}): Promise<boolean> {
  const client = getPermifyClient();
  if (!client) {
    // SECURITY: Default deny when Permify is not available
    // Only allow if explicitly configured to fall back to RBAC and user has admin role
    if (params.fallbackToRBAC) {
      console.warn("[Permify] Client not available, falling back to RBAC check");
      // In fallback mode, only system admins can proceed
      // This should be checked against the user's role from the auth context
      return false; // Default deny - caller must explicitly handle RBAC fallback
    }
    console.error("[Permify] Client not available - ACCESS DENIED (default deny policy)");
    return false;
  }

  try {
    const result = await client.permission.check({
      tenantId: "default",
      metadata: {
        schemaVersion: "",
        snapToken: "",
        depth: 20,
      },
      entity: {
        type: params.entity,
        id: params.entityId,
      },
      permission: params.action,
      subject: {
        type: "user",
        id: params.userId,
      },
    });

    const allowed = result.can === "RESULT_ALLOWED";
    console.log(
      `[Permify] Permission check: ${params.userId} can ${params.action} on ${params.entity}:${params.entityId} = ${allowed}`
    );

    return allowed;
  } catch (error) {
    console.error("[Permify] Permission check error:", error);
    return false;
  }
}

/**
 * Batch check permissions
 */
export async function checkPermissions(
  checks: Array<{
    entity: string;
    entityId: string;
    action: string;
    userId: string;
  }>
): Promise<boolean[]> {
  const results = await Promise.all(checks.map((check) => checkPermission(check)));
  return results;
}

/**
 * Get user permissions for entity
 */
export async function getUserPermissions(params: {
  entity: string;
  entityId: string;
  userId: string;
}): Promise<string[]> {
  const client = getPermifyClient();
  if (!client) {
    return [];
  }

  const actions = ["view", "edit", "delete", "approve", "manage"];
  const permissions: string[] = [];

  for (const action of actions) {
    const allowed = await checkPermission({
      entity: params.entity,
      entityId: params.entityId,
      action,
      userId: params.userId,
    });

    if (allowed) {
      permissions.push(action);
    }
  }

  return permissions;
}

/**
 * List entities user has access to
 */
export async function listAccessibleEntities(params: {
  entity: string;
  action: string;
  userId: string;
}): Promise<string[]> {
  const client = getPermifyClient();
  if (!client) {
    return [];
  }

  try {
    const result = await client.data.read({
      tenantId: "default",
      metadata: {
        snapToken: "",
      },
      filter: {
        entity: {
          type: params.entity,
        },
        subject: {
          type: "user",
          ids: [params.userId],
        },
      },
    });

    // Extract entity IDs from tuples
    const entityIds = result.tuples?.map((tuple: any) => tuple.entity?.id || "").filter((id: string) => id !== "") || [];

    console.log(`[Permify] User ${params.userId} has access to ${entityIds.length} ${params.entity} entities`);
    return entityIds;
  } catch (error) {
    console.error("[Permify] Failed to list accessible entities:", error);
    return [];
  }
}

/**
 * Helper functions for common permission scenarios
 */

// Beneficiary permissions
export async function grantBeneficiaryAccess(beneficiaryId: string, userId: string, role: "owner" | "editor" | "viewer"): Promise<void> {
  await createRelationship({
    entity: "beneficiary",
    entityId: beneficiaryId,
    relation: role,
    subject: "user",
    subjectId: userId,
  });
}

export async function canViewBeneficiary(beneficiaryId: string, userId: string): Promise<boolean> {
  return checkPermission({
    entity: "beneficiary",
    entityId: beneficiaryId,
    action: "view",
    userId,
  });
}

export async function canEditBeneficiary(beneficiaryId: string, userId: string): Promise<boolean> {
  return checkPermission({
    entity: "beneficiary",
    entityId: beneficiaryId,
    action: "edit",
    userId,
  });
}

// Program permissions
export async function grantProgramAccess(programId: string, userId: string, role: "admin" | "manager" | "viewer"): Promise<void> {
  await createRelationship({
    entity: "program",
    entityId: programId,
    relation: role,
    subject: "user",
    subjectId: userId,
  });
}

export async function canManageProgram(programId: string, userId: string): Promise<boolean> {
  return checkPermission({
    entity: "program",
    entityId: programId,
    action: "configure",
    userId,
  });
}

// Document permissions
export async function shareDocument(documentId: string, userId: string): Promise<void> {
  await createRelationship({
    entity: "document",
    entityId: documentId,
    relation: "shared_with",
    subject: "user",
    subjectId: userId,
  });
}

export async function canViewDocument(documentId: string, userId: string): Promise<boolean> {
  return checkPermission({
    entity: "document",
    entityId: documentId,
    action: "view",
    userId,
  });
}

/**
 * Health check
 */
export async function permifyHealthCheck(): Promise<boolean> {
  const client = getPermifyClient();
  if (!client) {
    return false;
  }

  try {
    // Try to read schema as health check
    await client.schema.read({
      tenantId: "default",
      metadata: {
        schemaVersion: "",
      },
    });
    return true;
  } catch (error) {
    console.error("[Permify] Health check failed:", error);
    return false;
  }
}

// Auto-initialize schema if Permify is configured
if (process.env.PERMIFY_URL) {
  writeAuthorizationSchema().catch((error) => {
    console.error("[Permify] Failed to initialize schema:", error);
  });
}
