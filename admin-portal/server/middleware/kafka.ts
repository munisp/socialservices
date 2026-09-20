import { Kafka, Producer, Consumer, EachMessagePayload, logLevel } from "kafkajs";
import { withDLQ } from "./kafkaDLQ";

/**
 * Kafka Event Streaming Integration
 * Distributed event streaming platform for event sourcing and async processing
 */

// Event topics
export const TOPICS = {
  BENEFICIARY_CREATED: "beneficiary.created",
  BENEFICIARY_UPDATED: "beneficiary.updated",
  BENEFICIARY_DELETED: "beneficiary.deleted",
  TRANSACTION_COMPLETED: "transaction.completed",
  TRANSACTION_FAILED: "transaction.failed",
  DISBURSEMENT_SCHEDULED: "disbursement.scheduled",
  DISBURSEMENT_PROCESSED: "disbursement.processed",
  DISBURSEMENT_FAILED: "disbursement.failed",
  FRAUD_DETECTED: "fraud.detected",
  KYC_VERIFIED: "kyc.verified",
  KYC_REJECTED: "kyc.rejected",
  PROGRAM_CREATED: "program.created",
  PROGRAM_UPDATED: "program.updated",
  APPROVAL_REQUESTED: "approval.requested",
  APPROVAL_COMPLETED: "approval.completed",
} as const;

export type EventTopic = (typeof TOPICS)[keyof typeof TOPICS];

// Event schema
export interface DomainEvent<T = any> {
  id: string;
  type: EventTopic;
  timestamp: Date;
  userId?: number;
  data: T;
  metadata?: Record<string, any>;
}

let kafka: Kafka | null = null;
let producer: Producer | null = null;
const consumers: Map<string, Consumer> = new Map();

/**
 * Initialize Kafka client
 */
export function getKafkaClient(): Kafka | null {
  if (kafka) {
    return kafka;
  }

  const brokers = process.env.KAFKA_BROKERS;
  if (!brokers) {
    console.warn("[Kafka] KAFKA_BROKERS not configured, event streaming disabled");
    return null;
  }

  try {
    kafka = new Kafka({
      clientId: process.env.KAFKA_CLIENT_ID || "admin-portal",
      brokers: brokers.split(","),
      logLevel: logLevel.ERROR,
      retry: {
        initialRetryTime: 100,
        retries: 8,
      },
    });

    console.log("[Kafka] Client initialized");
    return kafka;
  } catch (error) {
    console.error("[Kafka] Failed to initialize client:", error);
    return null;
  }
}

/**
 * Get or create producer
 */
async function getProducer(): Promise<Producer | null> {
  if (producer) {
    return producer;
  }

  const kafka = getKafkaClient();
  if (!kafka) {
    return null;
  }

  try {
    producer = kafka.producer({
      allowAutoTopicCreation: true,
      transactionTimeout: 30000,
    });

    await producer.connect();
    console.log("[Kafka] Producer connected");

    producer.on("producer.disconnect", () => {
      console.log("[Kafka] Producer disconnected");
    });

    return producer;
  } catch (error) {
    console.error("[Kafka] Failed to connect producer:", error);
    return null;
  }
}

/**
 * Publish event to Kafka
 */
export async function publishEvent<T>(
  topic: EventTopic,
  data: T,
  options?: {
    userId?: number;
    metadata?: Record<string, any>;
    key?: string;
  }
): Promise<void> {
  const producer = await getProducer();
  if (!producer) {
    if (process.env.NODE_ENV === "production") throw new Error(`Kafka producer unavailable for required event ${topic}`);
    console.warn(`[Kafka] Producer not available, skipping development event: ${topic}`);
    return;
  }

  const event: DomainEvent<T> = {
    id: `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
    type: topic,
    timestamp: new Date(),
    userId: options?.userId,
    data,
    metadata: options?.metadata,
  };

  try {
    await producer.send({
      topic,
      messages: [
        {
          key: options?.key,
          value: JSON.stringify(event),
          timestamp: event.timestamp.getTime().toString(),
        },
      ],
    });

    console.log(`[Kafka] Published event: ${topic}`);
  } catch (error) {
    console.error(`[Kafka] Failed to publish event ${topic}:`, error);
    throw error;
  }
}

/**
 * Create consumer for topic
 */
export async function createConsumer(
  groupId: string,
  topics: EventTopic[],
  handler: (event: DomainEvent) => Promise<void>
): Promise<void> {
  const kafka = getKafkaClient();
  if (!kafka) {
    console.warn(`[Kafka] Client not available, cannot create consumer: ${groupId}`);
    return;
  }

  if (consumers.has(groupId)) {
    console.warn(`[Kafka] Consumer ${groupId} already exists`);
    return;
  }

  try {
    const consumer = kafka.consumer({
      groupId,
      sessionTimeout: 30000,
      heartbeatInterval: 3000,
    });

    await consumer.connect();
    console.log(`[Kafka] Consumer ${groupId} connected`);

    await consumer.subscribe({ topics, fromBeginning: false });
    console.log(`[Kafka] Consumer ${groupId} subscribed to: ${topics.join(", ")}`);

    await consumer.run({
      eachMessage: async ({ topic, partition, message }: EachMessagePayload) => {
        try {
          if (!message.value) {
            console.warn(`[Kafka] Empty message received from ${topic}`);
            return;
          }

          const event: DomainEvent = JSON.parse(message.value.toString());
          console.log(`[Kafka] Processing event: ${event.type} (${event.id})`);

          const processed = await withDLQ(topic, partition, message.offset, message.key?.toString(), event,
            Object.fromEntries(Object.entries(message.headers ?? {}).map(([key,value]) => [key,value?.toString() ?? ""])), groupId, groupId, () => handler(event));
          if (processed === null) console.error(`[Kafka] Event moved to DLQ after retries: ${event.id}`);

          console.log(`[Kafka] Event processed successfully: ${event.id}`);
        } catch (error) {
          console.error(`[Kafka] Error processing message from ${topic}:`, error);
          throw error;
        }
      },
    });

    consumers.set(groupId, consumer);
  } catch (error) {
    console.error(`[Kafka] Failed to create consumer ${groupId}:`, error);
    throw error;
  }
}

/**
 * Stop consumer
 */
export async function stopConsumer(groupId: string): Promise<void> {
  const consumer = consumers.get(groupId);
  if (!consumer) {
    return;
  }

  try {
    await consumer.disconnect();
    consumers.delete(groupId);
    console.log(`[Kafka] Consumer ${groupId} stopped`);
  } catch (error) {
    console.error(`[Kafka] Failed to stop consumer ${groupId}:`, error);
  }
}

/**
 * Event handlers for common events
 */

// Beneficiary events
export async function handleBeneficiaryCreated(event: DomainEvent): Promise<void> {
  console.log(`[Kafka] Beneficiary created: ${event.data.id}`);
  // Send welcome notification
  // Update analytics
  // Trigger KYC workflow
}

export async function handleBeneficiaryUpdated(event: DomainEvent): Promise<void> {
  console.log(`[Kafka] Beneficiary updated: ${event.data.id}`);
  // Invalidate cache
  // Update search index
  // Audit log
}

// Transaction events
export async function handleTransactionCompleted(event: DomainEvent): Promise<void> {
  console.log(`[Kafka] Transaction completed: ${event.data.id}`);
  // Update beneficiary balance
  // Check fraud patterns
  // Update analytics
  // Send receipt notification
}

export async function handleTransactionFailed(event: DomainEvent): Promise<void> {
  console.log(`[Kafka] Transaction failed: ${event.data.id}`);
  // Send failure notification
  // Log error
  // Trigger retry workflow
}

// Disbursement events
export async function handleDisbursementProcessed(event: DomainEvent): Promise<void> {
  console.log(`[Kafka] Disbursement processed: ${event.data.id}`);
  // Update beneficiary accounts
  // Send success notifications
  // Update program metrics
}

// Fraud events
export async function handleFraudDetected(event: DomainEvent): Promise<void> {
  console.log(`[Kafka] Fraud detected: ${event.data.alertId}`);
  // Send urgent notification to admins
  // Block suspicious transactions
  // Trigger investigation workflow
}

// KYC events
export async function handleKycVerified(event: DomainEvent): Promise<void> {
  console.log(`[Kafka] KYC verified: ${event.data.beneficiaryId}`);
  // Update beneficiary status
  // Trigger benefit card issuance
  // Send approval notification
}

/**
 * Initialize event consumers
 */
export async function initializeKafkaConsumers(): Promise<void> {
  console.log("[Kafka] Initializing consumers...");

  // Beneficiary consumer
  await createConsumer(
    "beneficiary-processor",
    [TOPICS.BENEFICIARY_CREATED, TOPICS.BENEFICIARY_UPDATED],
    async (event) => {
      if (event.type === TOPICS.BENEFICIARY_CREATED) {
        await handleBeneficiaryCreated(event);
      } else if (event.type === TOPICS.BENEFICIARY_UPDATED) {
        await handleBeneficiaryUpdated(event);
      }
    }
  );

  // Transaction consumer
  await createConsumer(
    "transaction-processor",
    [TOPICS.TRANSACTION_COMPLETED, TOPICS.TRANSACTION_FAILED],
    async (event) => {
      if (event.type === TOPICS.TRANSACTION_COMPLETED) {
        await handleTransactionCompleted(event);
      } else if (event.type === TOPICS.TRANSACTION_FAILED) {
        await handleTransactionFailed(event);
      }
    }
  );

  // Disbursement consumer
  await createConsumer(
    "disbursement-processor",
    [TOPICS.DISBURSEMENT_SCHEDULED, TOPICS.DISBURSEMENT_PROCESSED, TOPICS.DISBURSEMENT_FAILED],
    async (event) => {
      if (event.type === TOPICS.DISBURSEMENT_PROCESSED) {
        await handleDisbursementProcessed(event);
      }
    }
  );

  // Fraud consumer
  await createConsumer("fraud-processor", [TOPICS.FRAUD_DETECTED], async (event) => {
    await handleFraudDetected(event);
  });

  // KYC consumer
  await createConsumer("kyc-processor", [TOPICS.KYC_VERIFIED, TOPICS.KYC_REJECTED], async (event) => {
    if (event.type === TOPICS.KYC_VERIFIED) {
      await handleKycVerified(event);
    }
  });

  console.log("[Kafka] All consumers initialized");
}

/**
 * Cleanup on shutdown
 */
export async function shutdownKafka(): Promise<void> {
  console.log("[Kafka] Shutting down...");

  // Stop all consumers
  const groupIds = Array.from(consumers.keys());
  for (const groupId of groupIds) {
    await stopConsumer(groupId);
  }

  // Disconnect producer
  if (producer) {
    await producer.disconnect();
    producer = null;
    console.log("[Kafka] Producer disconnected");
  }

  console.log("[Kafka] Shutdown complete");
}

// Auto-initialize if Kafka is configured
if (process.env.KAFKA_BROKERS) {
  initializeKafkaConsumers().catch((error) => {
    console.error("[Kafka] Failed to initialize consumers:", error);
  });
}

// Graceful shutdown
process.on("SIGTERM", async () => {
  await shutdownKafka();
  process.exit(0);
});

process.on("SIGINT", async () => {
  await shutdownKafka();
  process.exit(0);
});
