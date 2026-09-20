/**
 * Kafka Dead Letter Queue (DLQ) Handler
 * 
 * Handles failed messages by routing them to dead letter topics
 * for later analysis and reprocessing.
 */

import { v4 as uuidv4 } from "uuid";
import { Kafka, type Producer } from "kafkajs";

export interface DLQMessage {
  id: string;
  originalTopic: string;
  originalPartition: number;
  originalOffset: string;
  originalKey?: string;
  originalValue: unknown;
  originalHeaders: Record<string, string>;
  error: {
    message: string;
    stack?: string;
    code?: string;
  };
  retryCount: number;
  maxRetries: number;
  firstFailedAt: Date;
  lastFailedAt: Date;
  metadata: {
    consumerGroup: string;
    consumerId: string;
    correlationId?: string;
  };
}

export interface DLQConfig {
  dlqTopicSuffix: string;
  maxRetries: number;
  retryDelayMs: number;
  retryBackoffMultiplier: number;
  maxRetryDelayMs: number;
}

const DEFAULT_DLQ_CONFIG: DLQConfig = {
  dlqTopicSuffix: ".dlq",
  maxRetries: 3,
  retryDelayMs: 1000,
  retryBackoffMultiplier: 2,
  maxRetryDelayMs: 60000,
};

// In-memory DLQ store (production would use Kafka)
const dlqStore = new Map<string, DLQMessage[]>();
let dlqProducer: Producer | null = null;
async function getDLQProducer(): Promise<Producer | null> {
  const brokers = process.env.KAFKA_BROKERS;
  if (!brokers) return null;
  if (!dlqProducer) { dlqProducer = new Kafka({ clientId: "admin-portal-dlq", brokers: brokers.split(",") }).producer({ allowAutoTopicCreation: true }); await dlqProducer.connect(); }
  return dlqProducer;
}

/**
 * Get DLQ topic name for a given topic
 */
export function getDLQTopicName(originalTopic: string, config: Partial<DLQConfig> = {}): string {
  const suffix = config.dlqTopicSuffix || DEFAULT_DLQ_CONFIG.dlqTopicSuffix;
  return `${originalTopic}${suffix}`;
}

/**
 * Send message to DLQ
 */
export async function sendToDLQ(
  message: Omit<DLQMessage, "id" | "firstFailedAt" | "lastFailedAt">,
  config: Partial<DLQConfig> = {}
): Promise<string> {
  const dlqTopic = getDLQTopicName(message.originalTopic, config);
  
  const dlqMessage: DLQMessage = {
    ...message,
    id: uuidv4(),
    firstFailedAt: new Date(),
    lastFailedAt: new Date(),
  };

  // Get or create DLQ for this topic
  if (!dlqStore.has(dlqTopic)) {
    dlqStore.set(dlqTopic, []);
  }
  
  dlqStore.get(dlqTopic)!.push(dlqMessage);

  console.log(
    `[DLQ] Message sent to ${dlqTopic}: id=${dlqMessage.id}, ` +
    `originalTopic=${message.originalTopic}, error=${message.error.message}`
  );

  const producer = await getDLQProducer();
  if (producer) await producer.send({ topic: dlqTopic, messages: [{ key: dlqMessage.originalKey, value: JSON.stringify(dlqMessage) }] });
  else if (process.env.NODE_ENV === "production") throw new Error("KAFKA_BROKERS is required for durable dead-letter storage");

  return dlqMessage.id;
}

/**
 * Handle message processing with automatic DLQ routing
 */
export async function withDLQ<T>(
  topic: string,
  partition: number,
  offset: string,
  key: string | undefined,
  value: unknown,
  headers: Record<string, string>,
  consumerGroup: string,
  consumerId: string,
  processor: () => Promise<T>,
  config: Partial<DLQConfig> = {}
): Promise<T | null> {
  const mergedConfig = { ...DEFAULT_DLQ_CONFIG, ...config };
  let retryCount = 0;
  let lastError: Error | null = null;

  while (retryCount <= mergedConfig.maxRetries) {
    try {
      return await processor();
    } catch (error) {
      lastError = error as Error;
      retryCount++;

      if (retryCount <= mergedConfig.maxRetries) {
        // Calculate delay with exponential backoff
        const delay = Math.min(
          mergedConfig.retryDelayMs * Math.pow(mergedConfig.retryBackoffMultiplier, retryCount - 1),
          mergedConfig.maxRetryDelayMs
        );

        console.log(
          `[DLQ] Retry ${retryCount}/${mergedConfig.maxRetries} for topic=${topic}, ` +
          `partition=${partition}, offset=${offset}, delay=${delay}ms`
        );

        await new Promise(resolve => setTimeout(resolve, delay));
      }
    }
  }

  // All retries exhausted, send to DLQ
  await sendToDLQ({
    originalTopic: topic,
    originalPartition: partition,
    originalOffset: offset,
    originalKey: key,
    originalValue: value,
    originalHeaders: headers,
    error: {
      message: lastError?.message || "Unknown error",
      stack: lastError?.stack,
    },
    retryCount: mergedConfig.maxRetries,
    maxRetries: mergedConfig.maxRetries,
    metadata: {
      consumerGroup,
      consumerId,
      correlationId: headers["x-correlation-id"],
    },
  }, config);

  return null;
}

/**
 * Get messages from DLQ for a topic
 */
export function getDLQMessages(
  originalTopic: string,
  config: Partial<DLQConfig> = {}
): DLQMessage[] {
  const dlqTopic = getDLQTopicName(originalTopic, config);
  return dlqStore.get(dlqTopic) || [];
}

/**
 * Get all DLQ messages across all topics
 */
export function getAllDLQMessages(): Record<string, DLQMessage[]> {
  const result: Record<string, DLQMessage[]> = {};
  for (const [topic, messages] of dlqStore) {
    result[topic] = messages;
  }
  return result;
}

/**
 * Get DLQ stats
 */
export function getDLQStats(): {
  totalMessages: number;
  messagesByTopic: Record<string, number>;
  oldestMessage?: Date;
  newestMessage?: Date;
} {
  let totalMessages = 0;
  const messagesByTopic: Record<string, number> = {};
  let oldestMessage: Date | undefined;
  let newestMessage: Date | undefined;

  for (const [topic, messages] of dlqStore) {
    totalMessages += messages.length;
    messagesByTopic[topic] = messages.length;

    for (const msg of messages) {
      if (!oldestMessage || msg.firstFailedAt < oldestMessage) {
        oldestMessage = msg.firstFailedAt;
      }
      if (!newestMessage || msg.lastFailedAt > newestMessage) {
        newestMessage = msg.lastFailedAt;
      }
    }
  }

  return {
    totalMessages,
    messagesByTopic,
    oldestMessage,
    newestMessage,
  };
}

/**
 * Reprocess a DLQ message
 */
export async function reprocessDLQMessage(
  dlqTopic: string,
  messageId: string,
  processor: (message: DLQMessage) => Promise<void>
): Promise<boolean> {
  const messages = dlqStore.get(dlqTopic);
  if (!messages) {
    console.warn(`[DLQ] Topic not found: ${dlqTopic}`);
    return false;
  }

  const messageIndex = messages.findIndex(m => m.id === messageId);
  if (messageIndex === -1) {
    console.warn(`[DLQ] Message not found: ${messageId}`);
    return false;
  }

  const message = messages[messageIndex];

  try {
    await processor(message);
    
    // Remove from DLQ on success
    messages.splice(messageIndex, 1);
    console.log(`[DLQ] Successfully reprocessed message: ${messageId}`);
    return true;
  } catch (error) {
    // Update last failed time
    message.lastFailedAt = new Date();
    message.retryCount++;
    message.error = {
      message: (error as Error).message,
      stack: (error as Error).stack,
    };
    
    console.error(`[DLQ] Reprocessing failed for message: ${messageId}`, error);
    return false;
  }
}

/**
 * Purge old DLQ messages
 */
export function purgeDLQMessages(
  olderThan: Date,
  topic?: string
): number {
  let purged = 0;

  const topicsToProcess = topic 
    ? [getDLQTopicName(topic)] 
    : Array.from(dlqStore.keys());

  for (const dlqTopic of topicsToProcess) {
    const messages = dlqStore.get(dlqTopic);
    if (!messages) continue;

    const remaining = messages.filter(m => m.firstFailedAt >= olderThan);
    purged += messages.length - remaining.length;
    
    if (remaining.length === 0) {
      dlqStore.delete(dlqTopic);
    } else {
      dlqStore.set(dlqTopic, remaining);
    }
  }

  console.log(`[DLQ] Purged ${purged} messages older than ${olderThan.toISOString()}`);
  return purged;
}

// DLQ topics for the platform
export const DLQ_TOPICS = {
  BENEFICIARY_EVENTS: "beneficiary-events.dlq",
  DISBURSEMENT_EVENTS: "disbursement-events.dlq",
  WORKFLOW_EVENTS: "workflow-events.dlq",
  AUDIT_EVENTS: "audit-events.dlq",
  SYNC_EVENTS: "sync-events.dlq",
  NOTIFICATION_EVENTS: "notification-events.dlq",
} as const;

export default {
  getDLQTopicName,
  sendToDLQ,
  withDLQ,
  getDLQMessages,
  getAllDLQMessages,
  getDLQStats,
  reprocessDLQMessage,
  purgeDLQMessages,
  DLQ_TOPICS,
};
