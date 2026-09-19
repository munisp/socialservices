import { DaprClient, DaprServer, CommunicationProtocolEnum } from "@dapr/dapr";

/**
 * Dapr Microservices Runtime Integration
 * Portable, event-driven runtime for building resilient microservices
 */

let daprClient: DaprClient | null = null;
let daprServer: DaprServer | null = null;

/**
 * Get Dapr client instance
 */
export function getDaprClient(): DaprClient | null {
  if (daprClient) {
    return daprClient;
  }

  const daprHost = process.env.DAPR_HOST || "localhost";
  const daprPort = process.env.DAPR_HTTP_PORT || "3500";

  if (!process.env.DAPR_HTTP_PORT) {
    console.warn("[Dapr] DAPR_HTTP_PORT not configured, Dapr disabled");
    return null;
  }

  try {
    daprClient = new DaprClient({
      daprHost,
      daprPort,
      communicationProtocol: CommunicationProtocolEnum.HTTP,
    });

    console.log(`[Dapr] Client initialized (${daprHost}:${daprPort})`);
    return daprClient;
  } catch (error) {
    console.error("[Dapr] Failed to initialize client:", error);
    return null;
  }
}

/**
 * Get Dapr server instance
 */
export function getDaprServer(): DaprServer | null {
  if (daprServer) {
    return daprServer;
  }

  const serverHost = process.env.DAPR_SERVER_HOST || "127.0.0.1";
  const serverPort = process.env.DAPR_SERVER_PORT || "3001";

  if (!process.env.DAPR_HTTP_PORT) {
    console.warn("[Dapr] DAPR_HTTP_PORT not configured, Dapr server disabled");
    return null;
  }

  try {
    daprServer = new DaprServer({
      serverHost,
      serverPort,
      communicationProtocol: CommunicationProtocolEnum.HTTP,
    });

    console.log(`[Dapr] Server initialized (${serverHost}:${serverPort})`);
    return daprServer;
  } catch (error) {
    console.error("[Dapr] Failed to initialize server:", error);
    return null;
  }
}

/**
 * Service-to-service invocation
 */
export async function invokeService<T>(
  appId: string,
  methodName: string,
  data?: any
): Promise<T | null> {
  const client = getDaprClient();
  if (!client) {
    console.warn(`[Dapr] Client not available, cannot invoke ${appId}.${methodName}`);
    return null;
  }

  try {
    const result = await client.invoker.invoke(appId, methodName, "post" as any, data);
    console.log(`[Dapr] Service invocation: ${appId}.${methodName}`);
    return result as T;
  } catch (error) {
    console.error(`[Dapr] Service invocation failed: ${appId}.${methodName}`, error);
    return null;
  }
}

/**
 * Publish event to pub/sub
 */
export async function publishEvent(
  pubsubName: string,
  topic: string,
  data: any
): Promise<void> {
  const client = getDaprClient();
  if (!client) {
    console.warn(`[Dapr] Client not available, cannot publish to ${topic}`);
    return;
  }

  try {
    await client.pubsub.publish(pubsubName, topic, data);
    console.log(`[Dapr] Event published: ${pubsubName}/${topic}`);
  } catch (error) {
    console.error(`[Dapr] Failed to publish event: ${pubsubName}/${topic}`, error);
  }
}

/**
 * Subscribe to pub/sub topic
 */
export async function subscribeToTopic(
  pubsubName: string,
  topic: string,
  handler: (data: any) => Promise<void>
): Promise<void> {
  const server = getDaprServer();
  if (!server) {
    console.warn(`[Dapr] Server not available, cannot subscribe to ${topic}`);
    return;
  }

  try {
    await server.pubsub.subscribe(pubsubName, topic, async (data: any) => {
      console.log(`[Dapr] Event received: ${pubsubName}/${topic}`);
      await handler(data);
    });

    console.log(`[Dapr] Subscribed to: ${pubsubName}/${topic}`);
  } catch (error) {
    console.error(`[Dapr] Failed to subscribe: ${pubsubName}/${topic}`, error);
  }
}

/**
 * Save state
 */
export async function saveState(
  storeName: string,
  key: string,
  value: any
): Promise<void> {
  const client = getDaprClient();
  if (!client) {
    console.warn(`[Dapr] Client not available, cannot save state: ${key}`);
    return;
  }

  try {
    await client.state.save(storeName, [
      {
        key,
        value,
      },
    ]);

    console.log(`[Dapr] State saved: ${storeName}/${key}`);
  } catch (error) {
    console.error(`[Dapr] Failed to save state: ${storeName}/${key}`, error);
  }
}

/**
 * Get state
 */
export async function getState<T>(
  storeName: string,
  key: string
): Promise<T | null> {
  const client = getDaprClient();
  if (!client) {
    console.warn(`[Dapr] Client not available, cannot get state: ${key}`);
    return null;
  }

  try {
    const result = await client.state.get(storeName, key);
    console.log(`[Dapr] State retrieved: ${storeName}/${key}`);
    return result as T;
  } catch (error) {
    console.error(`[Dapr] Failed to get state: ${storeName}/${key}`, error);
    return null;
  }
}

/**
 * Delete state
 */
export async function deleteState(
  storeName: string,
  key: string
): Promise<void> {
  const client = getDaprClient();
  if (!client) {
    return;
  }

  try {
    await client.state.delete(storeName, key);
    console.log(`[Dapr] State deleted: ${storeName}/${key}`);
  } catch (error) {
    console.error(`[Dapr] Failed to delete state: ${storeName}/${key}`, error);
  }
}

/**
 * Get secret
 */
export async function getSecret(
  secretStoreName: string,
  secretKey: string
): Promise<string | null> {
  const client = getDaprClient();
  if (!client) {
    return null;
  }

  try {
    const result: any = await client.secret.get(secretStoreName, secretKey);
    console.log(`[Dapr] Secret retrieved: ${secretStoreName}/${secretKey}`);
    return result[secretKey] || null;
  } catch (error) {
    console.error(`[Dapr] Failed to get secret: ${secretStoreName}/${secretKey}`, error);
    return null;
  }
}

/**
 * Invoke output binding
 */
export async function invokeBinding(
  bindingName: string,
  operation: string,
  data: any
): Promise<void> {
  const client = getDaprClient();
  if (!client) {
    return;
  }

  try {
    await client.binding.send(bindingName, operation, data);
    console.log(`[Dapr] Binding invoked: ${bindingName}.${operation}`);
  } catch (error) {
    console.error(`[Dapr] Failed to invoke binding: ${bindingName}.${operation}`, error);
  }
}

/**
 * Start Dapr server
 */
export async function startDaprServer(): Promise<void> {
  const server = getDaprServer();
  if (!server) {
    return;
  }

  try {
    await server.start();
    console.log("[Dapr] Server started");
  } catch (error) {
    console.error("[Dapr] Failed to start server:", error);
  }
}

/**
 * Stop Dapr server
 */
export async function stopDaprServer(): Promise<void> {
  if (!daprServer) {
    return;
  }

  try {
    await daprServer.stop();
    console.log("[Dapr] Server stopped");
  } catch (error) {
    console.error("[Dapr] Failed to stop server:", error);
  }
}

/**
 * Health check
 */
export async function daprHealthCheck(): Promise<boolean> {
  const client = getDaprClient();
  if (!client) {
    return false;
  }

  try {
    // Try to get metadata as health check
    const daprHost = process.env.DAPR_HOST || "localhost";
    const daprPort = process.env.DAPR_HTTP_PORT || "3500";
    const response = await fetch(`http://${daprHost}:${daprPort}/v1.0/metadata`);
    return response.ok;
  } catch (error) {
    console.error("[Dapr] Health check failed:", error);
    return false;
  }
}

/**
 * Example: Publish beneficiary event via Dapr
 */
export async function publishBeneficiaryEvent(
  eventType: string,
  beneficiary: any
): Promise<void> {
  await publishEvent("pubsub", "beneficiary-events", {
    type: eventType,
    data: beneficiary,
    timestamp: new Date().toISOString(),
  });
}

/**
 * Example: Get cached data via Dapr state store
 */
export async function getCachedBeneficiary(beneficiaryId: string): Promise<any> {
  return getState("statestore", `beneficiary:${beneficiaryId}`);
}

/**
 * Example: Save cached data via Dapr state store
 */
export async function cacheBeneficiary(beneficiaryId: string, data: any): Promise<void> {
  await saveState("statestore", `beneficiary:${beneficiaryId}`, data);
}

// Auto-initialize if Dapr is configured
if (process.env.DAPR_HTTP_PORT) {
  getDaprClient();
  startDaprServer().catch((error) => {
    console.error("[Dapr] Failed to start server:", error);
  });
}

// Graceful shutdown
process.on("SIGTERM", async () => {
  await stopDaprServer();
});

process.on("SIGINT", async () => {
  await stopDaprServer();
});
