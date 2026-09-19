import { Client } from "@opensearch-project/opensearch";

/**
 * OpenSearch client for full-text search across all entities
 * 
 * For local development, you can run OpenSearch with Docker:
 * docker run -d -p 9200:9200 -p 9600:9600 -e "discovery.type=single-node" -e "OPENSEARCH_INITIAL_ADMIN_PASSWORD=Admin123!" opensearchproject/opensearch:latest
 * 
 * For production, set OPENSEARCH_URL environment variable
 */

let _client: Client | null = null;

export function getOpenSearchClient(): Client | null {
  if (_client) return _client;

  const opensearchUrl = process.env.OPENSEARCH_NODE;
  
  // If no OpenSearch URL configured, run in mock mode
  if (!opensearchUrl) {
    console.log("[OpenSearch] No OPENSEARCH_NODE configured - running in mock mode (search disabled)");
    return null;
  }

  const username = process.env.OPENSEARCH_USERNAME || "admin";
  const password = process.env.OPENSEARCH_PASSWORD || "Admin123!";

  try {
    _client = new Client({
      node: opensearchUrl,
      auth: {
        username,
        password,
      },
      ssl: {
        rejectUnauthorized: false, // For development only
      },
    });

    console.log(`[OpenSearch] Connected to ${opensearchUrl}`);
    return _client;
  } catch (error) {
    console.warn("[OpenSearch] Failed to connect:", error);
    return null;
  }
}

// Index names
export const INDICES = {
  PROGRAMS: "admin_portal_programs",
  USERS: "admin_portal_users",
  MCC_CODES: "admin_portal_mcc_codes",
  AUDIT_LOGS: "admin_portal_audit_logs",
} as const;

// Index mappings
export const INDEX_MAPPINGS: Record<string, any> = {
  [INDICES.PROGRAMS]: {
    properties: {
      id: { type: "integer" },
      name: { type: "text", analyzer: "standard" },
      description: { type: "text", analyzer: "standard" },
      benefitType: { type: "keyword" },
      status: { type: "keyword" },
      createdAt: { type: "date" },
      updatedAt: { type: "date" },
      mccRules: {
        type: "nested",
        properties: {
          mccCode: { type: "keyword" },
          mccDescription: { type: "text" },
        },
      },
    },
  },
  [INDICES.USERS]: {
    properties: {
      id: { type: "integer" },
      name: { type: "text", analyzer: "standard" },
      email: { type: "keyword" },
      role: { type: "keyword" },
      loginMethod: { type: "keyword" },
      createdAt: { type: "date" },
      lastSignedIn: { type: "date" },
    },
  },
  [INDICES.MCC_CODES]: {
    properties: {
      id: { type: "integer" },
      mccCode: { type: "keyword" },
      description: { type: "text", analyzer: "standard" },
      category: { type: "keyword" },
      createdAt: { type: "date" },
    },
  },
  [INDICES.AUDIT_LOGS]: {
    properties: {
      id: { type: "integer" },
      action: { type: "keyword" },
      targetType: { type: "keyword" },
      targetId: { type: "integer" },
      performedBy: { type: "integer" },
      performedByName: { type: "text" },
      justification: { type: "text", analyzer: "standard" },
      timestamp: { type: "date" },
    },
  },
};

/**
 * Initialize OpenSearch indices with proper mappings
 */
export async function initializeIndices(): Promise<void> {
  const client = getOpenSearchClient();
  if (!client) {
    console.warn("[OpenSearch] Client not available, skipping index initialization");
    return;
  }

  for (const [indexName, mapping] of Object.entries(INDEX_MAPPINGS)) {
    try {
      const { body: exists } = await client.indices.exists({ index: indexName });
      
      if (!exists) {
        await client.indices.create({
          index: indexName,
          body: {
            mappings: mapping,
          },
        });
        console.log(`[OpenSearch] Created index: ${indexName}`);
      } else {
        console.log(`[OpenSearch] Index already exists: ${indexName}`);
      }
    } catch (error) {
      console.error(`[OpenSearch] Failed to create index ${indexName}:`, error);
    }
  }
}

/**
 * Index a document in OpenSearch
 */
export async function indexDocument(
  index: string,
  id: string | number,
  document: Record<string, any>
): Promise<void> {
  const client = getOpenSearchClient();
  if (!client) return;

  try {
    await client.index({
      index,
      id: String(id),
      body: document,
      refresh: true,
    });
  } catch (error) {
    console.error(`[OpenSearch] Failed to index document in ${index}:`, error);
  }
}

/**
 * Delete a document from OpenSearch
 */
export async function deleteDocument(index: string, id: string | number): Promise<void> {
  const client = getOpenSearchClient();
  if (!client) return;

  try {
    await client.delete({
      index,
      id: String(id),
    });
  } catch (error) {
    console.error(`[OpenSearch] Failed to delete document from ${index}:`, error);
  }
}

/**
 * Search across all indices
 */
export async function globalSearch(query: string, filters?: {
  types?: string[];
  dateRange?: { from?: string; to?: string };
}): Promise<any[]> {
  const client = getOpenSearchClient();
  if (!client) return [];

  const indices = filters?.types?.length
    ? filters.types.map((type) => INDICES[type.toUpperCase() as keyof typeof INDICES])
    : Object.values(INDICES);

  try {
    const { body } = await client.search({
      index: indices.join(","),
      body: {
        query: {
          bool: {
            must: [
              {
                multi_match: {
                  query,
                  fields: ["name^2", "description", "email", "mccCode", "justification"],
                  type: "best_fields",
                  fuzziness: "AUTO",
                },
              },
            ],
            filter: filters?.dateRange
              ? [
                  {
                    range: {
                      createdAt: {
                        gte: filters.dateRange.from,
                        lte: filters.dateRange.to,
                      },
                    },
                  },
                ]
              : [],
          },
        },
        highlight: {
          fields: {
            name: {},
            description: {},
            email: {},
            justification: {},
          },
        },
        size: 50,
      },
    });

    return body.hits.hits.map((hit: any) => ({
      index: hit._index,
      id: hit._id,
      score: hit._score,
      source: hit._source,
      highlights: hit.highlight,
    }));
  } catch (error) {
    console.error("[OpenSearch] Search failed:", error);
    return [];
  }
}
