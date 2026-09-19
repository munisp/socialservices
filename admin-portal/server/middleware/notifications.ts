/**
 * Notification Middleware
 * Delivery tracking, retry logic, and notification preferences
 */

import crypto from "crypto";

// ============================================================================
// Types
// ============================================================================

type NotificationChannel = "email" | "sms" | "push" | "in_app" | "webhook";
type NotificationStatus = "pending" | "queued" | "sent" | "delivered" | "failed" | "bounced" | "unsubscribed";
type NotificationPriority = "low" | "normal" | "high" | "critical";

interface NotificationTemplate {
  id: string;
  name: string;
  channel: NotificationChannel;
  subject?: string;
  body: string;
  variables: string[];
}

interface NotificationRequest {
  id: string;
  templateId?: string;
  channel: NotificationChannel;
  recipient: {
    userId?: string;
    email?: string;
    phone?: string;
    deviceToken?: string;
    webhookUrl?: string;
  };
  subject?: string;
  body: string;
  data?: Record<string, any>;
  priority: NotificationPriority;
  scheduledAt?: Date;
  expiresAt?: Date;
  idempotencyKey?: string;
}

interface NotificationRecord {
  id: string;
  request: NotificationRequest;
  status: NotificationStatus;
  attempts: number;
  lastAttemptAt?: Date;
  nextRetryAt?: Date;
  deliveredAt?: Date;
  failureReason?: string;
  providerMessageId?: string;
  createdAt: Date;
  updatedAt: Date;
}

interface RetryConfig {
  maxAttempts: number;
  initialDelayMs: number;
  maxDelayMs: number;
  backoffMultiplier: number;
  retryableErrors: string[];
}

interface DeliveryWebhook {
  notificationId: string;
  status: NotificationStatus;
  timestamp: Date;
  providerData?: Record<string, any>;
}

// ============================================================================
// Configuration
// ============================================================================

const DEFAULT_RETRY_CONFIG: RetryConfig = {
  maxAttempts: 5,
  initialDelayMs: 1000,
  maxDelayMs: 3600000, // 1 hour
  backoffMultiplier: 2,
  retryableErrors: [
    "ETIMEDOUT",
    "ECONNRESET",
    "ECONNREFUSED",
    "RATE_LIMITED",
    "SERVICE_UNAVAILABLE",
    "TEMPORARY_FAILURE",
  ],
};

const CHANNEL_CONFIGS: Record<NotificationChannel, RetryConfig> = {
  email: {
    ...DEFAULT_RETRY_CONFIG,
    maxAttempts: 5,
  },
  sms: {
    ...DEFAULT_RETRY_CONFIG,
    maxAttempts: 3,
    maxDelayMs: 1800000, // 30 minutes
  },
  push: {
    ...DEFAULT_RETRY_CONFIG,
    maxAttempts: 3,
    maxDelayMs: 900000, // 15 minutes
  },
  in_app: {
    ...DEFAULT_RETRY_CONFIG,
    maxAttempts: 2,
    maxDelayMs: 300000, // 5 minutes
  },
  webhook: {
    ...DEFAULT_RETRY_CONFIG,
    maxAttempts: 10,
    maxDelayMs: 7200000, // 2 hours
  },
};

// ============================================================================
// Storage
// ============================================================================

// In-memory storage (use database in production)
const notificationRecords = new Map<string, NotificationRecord>();
const userPreferences = new Map<string, Map<NotificationChannel, boolean>>();
const deadLetterQueue: NotificationRecord[] = [];
const deliveryCallbacks = new Map<string, (status: NotificationStatus, data?: any) => void>();

// ============================================================================
// Notification Queue
// ============================================================================

/**
 * Generate notification ID
 */
function generateNotificationId(): string {
  return `notif_${Date.now()}_${crypto.randomBytes(8).toString("hex")}`;
}

/**
 * Create notification record
 */
export function createNotification(request: Omit<NotificationRequest, "id">): NotificationRecord {
  const id = generateNotificationId();
  const now = new Date();
  
  const record: NotificationRecord = {
    id,
    request: { ...request, id },
    status: "pending",
    attempts: 0,
    createdAt: now,
    updatedAt: now,
  };
  
  notificationRecords.set(id, record);
  
  console.info(`[Notifications] Created notification ${id} for ${request.channel}`);
  
  return record;
}

/**
 * Queue notification for sending
 */
export async function queueNotification(request: Omit<NotificationRequest, "id">): Promise<NotificationRecord> {
  // Check idempotency
  if (request.idempotencyKey) {
    for (const record of notificationRecords.values()) {
      if (record.request.idempotencyKey === request.idempotencyKey) {
        console.info(`[Notifications] Duplicate notification detected: ${request.idempotencyKey}`);
        return record;
      }
    }
  }
  
  // Check user preferences
  if (request.recipient.userId) {
    const prefs = userPreferences.get(request.recipient.userId);
    if (prefs && prefs.get(request.channel) === false) {
      console.info(`[Notifications] User ${request.recipient.userId} has disabled ${request.channel} notifications`);
      const record = createNotification(request);
      record.status = "unsubscribed";
      return record;
    }
  }
  
  const record = createNotification(request);
  record.status = "queued";
  
  // Schedule for immediate or delayed sending
  if (request.scheduledAt && request.scheduledAt > new Date()) {
    record.nextRetryAt = request.scheduledAt;
  } else {
    // Process immediately
    await processNotification(record);
  }
  
  return record;
}

/**
 * Process notification (attempt to send)
 */
async function processNotification(record: NotificationRecord): Promise<void> {
  const config = CHANNEL_CONFIGS[record.request.channel];
  
  // Check if expired
  if (record.request.expiresAt && new Date() > record.request.expiresAt) {
    record.status = "failed";
    record.failureReason = "Notification expired";
    record.updatedAt = new Date();
    console.warn(`[Notifications] Notification ${record.id} expired`);
    return;
  }
  
  // Check max attempts
  if (record.attempts >= config.maxAttempts) {
    record.status = "failed";
    record.failureReason = "Max retry attempts exceeded";
    record.updatedAt = new Date();
    moveToDeadLetterQueue(record);
    return;
  }
  
  record.attempts++;
  record.lastAttemptAt = new Date();
  record.updatedAt = new Date();
  
  try {
    const result = await sendNotification(record);
    
    if (result.success) {
      record.status = "sent";
      record.providerMessageId = result.messageId;
      record.updatedAt = new Date();
      
      console.info(`[Notifications] Notification ${record.id} sent successfully`);
      
      // Trigger delivery callback if registered
      const callback = deliveryCallbacks.get(record.id);
      if (callback) {
        callback("sent", { messageId: result.messageId });
      }
    } else {
      throw new Error(result.error || "Unknown error");
    }
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : "Unknown error";
    record.failureReason = errorMessage;
    record.updatedAt = new Date();
    
    console.error(`[Notifications] Notification ${record.id} failed: ${errorMessage}`);
    
    // Check if error is retryable
    const isRetryable = config.retryableErrors.some(e => errorMessage.includes(e));
    
    if (isRetryable && record.attempts < config.maxAttempts) {
      // Schedule retry with exponential backoff
      const delay = Math.min(
        config.initialDelayMs * Math.pow(config.backoffMultiplier, record.attempts - 1),
        config.maxDelayMs
      );
      record.nextRetryAt = new Date(Date.now() + delay);
      record.status = "queued";
      
      console.info(`[Notifications] Scheduling retry for ${record.id} in ${delay}ms`);
    } else {
      record.status = "failed";
      moveToDeadLetterQueue(record);
    }
  }
}

/**
 * Send notification via appropriate channel
 */
async function sendNotification(record: NotificationRecord): Promise<{ success: boolean; messageId?: string; error?: string }> {
  const { channel, recipient, subject, body, data } = record.request;
  
  switch (channel) {
    case "email":
      return sendEmail(recipient.email!, subject!, body, data);
    
    case "sms":
      return sendSMS(recipient.phone!, body, data);
    
    case "push":
      return sendPushNotification(recipient.deviceToken!, subject!, body, data);
    
    case "in_app":
      return sendInAppNotification(recipient.userId!, subject!, body, data);
    
    case "webhook":
      return sendWebhook(recipient.webhookUrl!, { subject, body, data });
    
    default:
      return { success: false, error: `Unknown channel: ${channel}` };
  }
}

/**
 * Send email notification
 */
async function sendEmail(
  to: string,
  subject: string,
  body: string,
  data?: Record<string, any>
): Promise<{ success: boolean; messageId?: string; error?: string }> {
  // Integration with email provider (SendGrid, SES, etc.)
  const emailProvider = process.env.EMAIL_PROVIDER || "console";
  
  if (emailProvider === "console") {
    console.info(`[Email] To: ${to}, Subject: ${subject}, Body: ${body.substring(0, 100)}...`);
    return { success: true, messageId: `email_${Date.now()}` };
  }
  
  // Real email provider integration
  try {
    const response = await fetch(process.env.EMAIL_API_URL || "http://localhost:3001/email", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${process.env.EMAIL_API_KEY}`,
      },
      body: JSON.stringify({ to, subject, body, data }),
    });
    
    if (!response.ok) {
      const error = await response.text();
      return { success: false, error };
    }
    
    const result = await response.json();
    return { success: true, messageId: result.messageId };
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : "Email send failed" };
  }
}

/**
 * Send SMS notification
 */
async function sendSMS(
  to: string,
  body: string,
  data?: Record<string, any>
): Promise<{ success: boolean; messageId?: string; error?: string }> {
  const smsProvider = process.env.SMS_PROVIDER || "console";
  
  if (smsProvider === "console") {
    console.info(`[SMS] To: ${to}, Body: ${body.substring(0, 100)}...`);
    return { success: true, messageId: `sms_${Date.now()}` };
  }
  
  // Real SMS provider integration (Twilio, etc.)
  try {
    const response = await fetch(process.env.SMS_API_URL || "http://localhost:3001/sms", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${process.env.SMS_API_KEY}`,
      },
      body: JSON.stringify({ to, body, data }),
    });
    
    if (!response.ok) {
      const error = await response.text();
      return { success: false, error };
    }
    
    const result = await response.json();
    return { success: true, messageId: result.messageId };
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : "SMS send failed" };
  }
}

/**
 * Send push notification
 */
async function sendPushNotification(
  deviceToken: string,
  title: string,
  body: string,
  data?: Record<string, any>
): Promise<{ success: boolean; messageId?: string; error?: string }> {
  const pushProvider = process.env.PUSH_PROVIDER || "console";
  
  if (pushProvider === "console") {
    console.info(`[Push] Token: ${deviceToken.substring(0, 10)}..., Title: ${title}, Body: ${body.substring(0, 50)}...`);
    return { success: true, messageId: `push_${Date.now()}` };
  }
  
  // Real push provider integration (FCM, APNS, etc.)
  try {
    const response = await fetch(process.env.PUSH_API_URL || "http://localhost:3001/push", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${process.env.PUSH_API_KEY}`,
      },
      body: JSON.stringify({ deviceToken, title, body, data }),
    });
    
    if (!response.ok) {
      const error = await response.text();
      return { success: false, error };
    }
    
    const result = await response.json();
    return { success: true, messageId: result.messageId };
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : "Push send failed" };
  }
}

/**
 * Send in-app notification
 */
async function sendInAppNotification(
  userId: string,
  title: string,
  body: string,
  data?: Record<string, any>
): Promise<{ success: boolean; messageId?: string; error?: string }> {
  // Store in-app notification for user
  const messageId = `inapp_${Date.now()}`;
  
  // In production, this would store to database and potentially trigger WebSocket
  console.info(`[InApp] User: ${userId}, Title: ${title}, Body: ${body.substring(0, 50)}...`);
  
  return { success: true, messageId };
}

/**
 * Send webhook notification
 */
async function sendWebhook(
  url: string,
  payload: Record<string, any>
): Promise<{ success: boolean; messageId?: string; error?: string }> {
  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Webhook-Signature": generateWebhookSignature(payload),
      },
      body: JSON.stringify(payload),
    });
    
    if (!response.ok) {
      const error = await response.text();
      return { success: false, error: `HTTP ${response.status}: ${error}` };
    }
    
    return { success: true, messageId: `webhook_${Date.now()}` };
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : "Webhook send failed" };
  }
}

/**
 * Generate webhook signature
 */
function generateWebhookSignature(payload: Record<string, any>): string {
  const secret = process.env.WEBHOOK_SECRET || "webhook-secret";
  return crypto
    .createHmac("sha256", secret)
    .update(JSON.stringify(payload))
    .digest("hex");
}

// ============================================================================
// Dead Letter Queue
// ============================================================================

/**
 * Move failed notification to dead letter queue
 */
function moveToDeadLetterQueue(record: NotificationRecord): void {
  deadLetterQueue.push(record);
  
  console.warn(`[Notifications] Moved notification ${record.id} to dead letter queue`);
  
  // Trim DLQ if too large
  if (deadLetterQueue.length > 10000) {
    deadLetterQueue.splice(0, deadLetterQueue.length - 10000);
  }
}

/**
 * Get dead letter queue contents
 */
export function getDeadLetterQueue(): NotificationRecord[] {
  return [...deadLetterQueue];
}

/**
 * Retry notification from dead letter queue
 */
export async function retryFromDeadLetterQueue(notificationId: string): Promise<boolean> {
  const index = deadLetterQueue.findIndex(r => r.id === notificationId);
  if (index === -1) {
    return false;
  }
  
  const record = deadLetterQueue[index];
  deadLetterQueue.splice(index, 1);
  
  // Reset retry state
  record.attempts = 0;
  record.status = "queued";
  record.failureReason = undefined;
  record.nextRetryAt = undefined;
  record.updatedAt = new Date();
  
  await processNotification(record);
  return true;
}

// ============================================================================
// Delivery Tracking
// ============================================================================

/**
 * Handle delivery status webhook from provider
 */
export function handleDeliveryWebhook(webhook: DeliveryWebhook): void {
  const record = notificationRecords.get(webhook.notificationId);
  if (!record) {
    // Try to find by provider message ID
    for (const r of notificationRecords.values()) {
      if (r.providerMessageId === webhook.notificationId) {
        updateDeliveryStatus(r, webhook);
        return;
      }
    }
    console.warn(`[Notifications] Unknown notification for delivery webhook: ${webhook.notificationId}`);
    return;
  }
  
  updateDeliveryStatus(record, webhook);
}

/**
 * Update delivery status
 */
function updateDeliveryStatus(record: NotificationRecord, webhook: DeliveryWebhook): void {
  record.status = webhook.status;
  record.updatedAt = new Date();
  
  if (webhook.status === "delivered") {
    record.deliveredAt = webhook.timestamp;
  }
  
  console.info(`[Notifications] Delivery status update for ${record.id}: ${webhook.status}`);
  
  // Trigger callback if registered
  const callback = deliveryCallbacks.get(record.id);
  if (callback) {
    callback(webhook.status, webhook.providerData);
  }
}

/**
 * Register delivery callback
 */
export function onDeliveryStatus(
  notificationId: string,
  callback: (status: NotificationStatus, data?: any) => void
): void {
  deliveryCallbacks.set(notificationId, callback);
}

/**
 * Get notification status
 */
export function getNotificationStatus(notificationId: string): NotificationRecord | null {
  return notificationRecords.get(notificationId) || null;
}

/**
 * Get notifications for user
 */
export function getUserNotifications(userId: string): NotificationRecord[] {
  const results: NotificationRecord[] = [];
  for (const record of notificationRecords.values()) {
    if (record.request.recipient.userId === userId) {
      results.push(record);
    }
  }
  return results.sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime());
}

// ============================================================================
// User Preferences
// ============================================================================

/**
 * Set user notification preference
 */
export function setUserPreference(userId: string, channel: NotificationChannel, enabled: boolean): void {
  if (!userPreferences.has(userId)) {
    userPreferences.set(userId, new Map());
  }
  userPreferences.get(userId)!.set(channel, enabled);
}

/**
 * Get user notification preferences
 */
export function getUserPreferences(userId: string): Record<NotificationChannel, boolean> {
  const prefs = userPreferences.get(userId);
  const defaults: Record<NotificationChannel, boolean> = {
    email: true,
    sms: true,
    push: true,
    in_app: true,
    webhook: true,
  };
  
  if (!prefs) {
    return defaults;
  }
  
  return {
    email: prefs.get("email") ?? true,
    sms: prefs.get("sms") ?? true,
    push: prefs.get("push") ?? true,
    in_app: prefs.get("in_app") ?? true,
    webhook: prefs.get("webhook") ?? true,
  };
}

// ============================================================================
// Retry Processing
// ============================================================================

/**
 * Process pending retries
 */
async function processRetries(): Promise<void> {
  const now = new Date();
  
  for (const record of notificationRecords.values()) {
    if (record.status === "queued" && record.nextRetryAt && record.nextRetryAt <= now) {
      await processNotification(record);
    }
  }
}

// Run retry processor every 10 seconds
setInterval(processRetries, 10000);

// ============================================================================
// Metrics
// ============================================================================

/**
 * Get notification metrics
 */
export function getNotificationMetrics(): {
  total: number;
  byStatus: Record<NotificationStatus, number>;
  byChannel: Record<NotificationChannel, number>;
  deliveryRate: number;
  averageDeliveryTime: number;
} {
  const byStatus: Record<NotificationStatus, number> = {
    pending: 0,
    queued: 0,
    sent: 0,
    delivered: 0,
    failed: 0,
    bounced: 0,
    unsubscribed: 0,
  };
  
  const byChannel: Record<NotificationChannel, number> = {
    email: 0,
    sms: 0,
    push: 0,
    in_app: 0,
    webhook: 0,
  };
  
  let deliveredCount = 0;
  let sentCount = 0;
  let totalDeliveryTime = 0;
  let deliveryTimeCount = 0;
  
  for (const record of notificationRecords.values()) {
    byStatus[record.status]++;
    byChannel[record.request.channel]++;
    
    if (record.status === "delivered") {
      deliveredCount++;
      if (record.deliveredAt) {
        totalDeliveryTime += record.deliveredAt.getTime() - record.createdAt.getTime();
        deliveryTimeCount++;
      }
    }
    
    if (record.status === "sent" || record.status === "delivered") {
      sentCount++;
    }
  }
  
  return {
    total: notificationRecords.size,
    byStatus,
    byChannel,
    deliveryRate: sentCount > 0 ? deliveredCount / sentCount : 0,
    averageDeliveryTime: deliveryTimeCount > 0 ? totalDeliveryTime / deliveryTimeCount : 0,
  };
}

// ============================================================================
// Cleanup
// ============================================================================

/**
 * Clean up old notification records
 */
export function cleanupOldRecords(maxAgeDays: number = 30): number {
  const cutoff = new Date();
  cutoff.setDate(cutoff.getDate() - maxAgeDays);
  
  let cleaned = 0;
  for (const [id, record] of notificationRecords.entries()) {
    if (record.createdAt < cutoff) {
      notificationRecords.delete(id);
      cleaned++;
    }
  }
  
  return cleaned;
}

// Run cleanup daily
setInterval(() => cleanupOldRecords(30), 24 * 60 * 60 * 1000);
