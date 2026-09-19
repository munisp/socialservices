// @ts-ignore - Fluvio client types not fully compatible
const Fluvio = require("@fluvio/client");

/**
 * Fluvio Real-time Data Streaming
 * Lightweight real-time data streaming for analytics and live dashboards
 */

// Stream topics
export const STREAMS = {
  TRANSACTIONS: "transactions-stream",
  FRAUD_ALERTS: "fraud-alerts-stream",
  SYSTEM_METRICS: "system-metrics-stream",
  USER_ACTIVITY: "user-activity-stream",
  ANALYTICS_EVENTS: "analytics-events-stream",
} as const;

export type StreamTopic = (typeof STREAMS)[keyof typeof STREAMS];

let fluvioClient: Awaited<ReturnType<typeof Fluvio.connect>> | null = null;

/**
 * Get Fluvio client instance
 */
export async function getFluvioClient() {
  if (fluvioClient) {
    return fluvioClient;
  }

  const fluvioCluster = process.env.FLUVIO_CLUSTER;
  if (!fluvioCluster) {
    console.warn("[Fluvio] FLUVIO_CLUSTER not configured, streaming disabled");
    return null;
  }

  try {
    fluvioClient = await Fluvio.connect(fluvioCluster);
    console.log("[Fluvio] Connected successfully");
    return fluvioClient;
  } catch (error) {
    console.error("[Fluvio] Failed to connect:", error);
    return null;
  }
}

/**
 * Produce message to stream
 */
export async function produceToStream(topic: StreamTopic, data: any): Promise<void> {
  const client = await getFluvioClient();
  if (!client) {
    console.warn(`[Fluvio] Client not available, skipping stream: ${topic}`);
    return;
  }

  try {
    const producer = await client.topicProducer(topic);

    const message = {
      timestamp: new Date().toISOString(),
      data,
    };

    await producer.send(topic, JSON.stringify(message));
    console.log(`[Fluvio] Produced to ${topic}`);
  } catch (error) {
    console.error(`[Fluvio] Failed to produce to ${topic}:`, error);
  }
}

/**
 * Consume from stream
 */
export async function consumeFromStream(
  topic: StreamTopic,
  handler: (data: any) => void | Promise<void>
): Promise<void> {
  const client = await getFluvioClient();
  if (!client) {
    console.warn(`[Fluvio] Client not available, cannot consume: ${topic}`);
    return;
  }

  try {
    const consumer = await client.partitionConsumer(topic, 0);

    console.log(`[Fluvio] Consuming from ${topic}...`);

    await consumer.stream(async (record: any) => {
      try {
        const message = JSON.parse(record.valueString());
        await handler(message.data);
      } catch (error) {
        console.error(`[Fluvio] Error processing message from ${topic}:`, error);
      }
    });
  } catch (error) {
    console.error(`[Fluvio] Failed to consume from ${topic}:`, error);
  }
}

/**
 * Stream transaction data for real-time monitoring
 */
export async function streamTransaction(transaction: any): Promise<void> {
  await produceToStream(STREAMS.TRANSACTIONS, {
    id: transaction.id,
    beneficiaryId: transaction.beneficiaryId,
    amount: transaction.amount,
    mccCode: transaction.mccCode,
    status: transaction.status,
    timestamp: transaction.createdAt,
  });
}

/**
 * Stream fraud alert for real-time notifications
 */
export async function streamFraudAlert(alert: any): Promise<void> {
  await produceToStream(STREAMS.FRAUD_ALERTS, {
    id: alert.id,
    beneficiaryId: alert.beneficiaryId,
    severity: alert.severity,
    alertType: alert.alertType,
    description: alert.description,
    timestamp: alert.detectedAt,
  });
}

/**
 * Stream system metrics for monitoring
 */
export async function streamSystemMetric(metric: {
  name: string;
  value: number;
  unit?: string;
  tags?: Record<string, string>;
}): Promise<void> {
  await produceToStream(STREAMS.SYSTEM_METRICS, metric);
}

/**
 * Stream user activity for analytics
 */
export async function streamUserActivity(activity: {
  userId: number;
  action: string;
  resource?: string;
  metadata?: Record<string, any>;
}): Promise<void> {
  await produceToStream(STREAMS.USER_ACTIVITY, activity);
}

/**
 * Stream analytics event
 */
export async function streamAnalyticsEvent(event: {
  eventType: string;
  properties: Record<string, any>;
}): Promise<void> {
  await produceToStream(STREAMS.ANALYTICS_EVENTS, event);
}

/**
 * Initialize Fluvio consumers for real-time processing
 */
export async function initializeFluvioConsumers(): Promise<void> {
  console.log("[Fluvio] Initializing consumers...");

  // Transaction stream consumer
  consumeFromStream(STREAMS.TRANSACTIONS, async (transaction) => {
    console.log(`[Fluvio] Transaction received: ${transaction.id}`);
    // Update real-time dashboard
    // Trigger fraud detection
    // Update analytics
  });

  // Fraud alert stream consumer
  consumeFromStream(STREAMS.FRAUD_ALERTS, async (alert) => {
    console.log(`[Fluvio] Fraud alert received: ${alert.id}`);
    // Send real-time notification
    // Update dashboard
    // Trigger investigation
  });

  // System metrics stream consumer
  consumeFromStream(STREAMS.SYSTEM_METRICS, async (metric) => {
    console.log(`[Fluvio] System metric: ${metric.name} = ${metric.value}`);
    // Update monitoring dashboard
    // Check thresholds
    // Trigger alerts
  });

  console.log("[Fluvio] All consumers initialized");
}

/**
 * Health check
 */
export async function fluvioHealthCheck(): Promise<boolean> {
  try {
    const client = await getFluvioClient();
    return client !== null;
  } catch (error) {
    console.error("[Fluvio] Health check failed:", error);
    return false;
  }
}

// Auto-initialize if Fluvio is configured
if (process.env.FLUVIO_CLUSTER) {
  initializeFluvioConsumers().catch((error) => {
    console.error("[Fluvio] Failed to initialize consumers:", error);
  });
}
