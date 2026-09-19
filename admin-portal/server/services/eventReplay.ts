import { getDb } from "../db";
import { eventSnapshots, eventReplays, eventReplayLogs } from "../../drizzle/schema";
import { desc, eq } from "drizzle-orm";

/**
 * Event Replay System
 * Rebuild state from Kafka events, audit changes, and recover from data corruption
 */

export interface EventSnapshot {
  id: number;
  snapshotName: string;
  description?: string;
  eventCount: number;
  startOffset: string;
  endOffset: string;
  topics: string[];
  createdBy: number;
  createdAt: Date;
}

export interface EventReplay {
  id: number;
  replayName: string;
  snapshotId?: number;
  topics: string[];
  startOffset?: string;
  endOffset?: string;
  eventFilter?: any;
  status: "pending" | "running" | "completed" | "failed" | "cancelled";
  progress: number;
  eventsProcessed: number;
  eventsTotal: number;
  errorMessage?: string;
  startedBy: number;
  startedAt: Date;
  completedAt?: Date;
}

/**
 * Create event snapshot
 */
export async function createEventSnapshot(
  snapshotName: string,
  description: string | undefined,
  topics: string[],
  startOffset: string,
  endOffset: string,
  eventCount: number,
  userId: number
): Promise<number | null> {
  const db = await getDb();
  if (!db) return null;

  try {
    const result = await db.insert(eventSnapshots).values({
      snapshotName,
      description: description || null,
      topics: JSON.stringify(topics),
      startOffset,
      endOffset,
      eventCount,
      createdBy: userId,
    });

    console.log(`[EventReplay] Snapshot created: ${snapshotName}`);
    return result[0].insertId;
  } catch (error) {
    console.error("[EventReplay] Failed to create snapshot:", error);
    return null;
  }
}

/**
 * Get all snapshots
 */
export async function getAllSnapshots(): Promise<EventSnapshot[]> {
  const db = await getDb();
  if (!db) return [];

  try {
    const results = await db
      .select()
      .from(eventSnapshots)
      .orderBy(desc(eventSnapshots.createdAt))
      .limit(100);

    return results.map((r) => ({
      id: r.id,
      snapshotName: r.snapshotName,
      description: r.description || undefined,
      eventCount: r.eventCount,
      startOffset: r.startOffset,
      endOffset: r.endOffset,
      topics: JSON.parse(r.topics),
      createdBy: r.createdBy,
      createdAt: r.createdAt,
    }));
  } catch (error) {
    console.error("[EventReplay] Failed to get snapshots:", error);
    return [];
  }
}

/**
 * Start event replay
 */
export async function startEventReplay(
  replayName: string,
  topics: string[],
  userId: number,
  options?: {
    snapshotId?: number;
    startOffset?: string;
    endOffset?: string;
    eventFilter?: any;
  }
): Promise<number | null> {
  const db = await getDb();
  if (!db) return null;

  try {
    const result = await db.insert(eventReplays).values({
      replayName,
      snapshotId: options?.snapshotId || null,
      topics: JSON.stringify(topics),
      startOffset: options?.startOffset || null,
      endOffset: options?.endOffset || null,
      eventFilter: options?.eventFilter ? JSON.stringify(options.eventFilter) : null,
      status: "pending",
      startedBy: userId,
    });

    const replayId = result[0].insertId;

    // Start replay process in background
    processEventReplay(replayId).catch((error) => {
      console.error(`[EventReplay] Replay ${replayId} failed:`, error);
    });

    console.log(`[EventReplay] Replay started: ${replayName} (${replayId})`);
    return replayId;
  } catch (error) {
    console.error("[EventReplay] Failed to start replay:", error);
    return null;
  }
}

/**
 * Process event replay (background task)
 */
async function processEventReplay(replayId: number): Promise<void> {
  const db = await getDb();
  if (!db) return;

  try {
    // Update status to running
    await db
      .update(eventReplays)
      .set({ status: "running" })
      .where(eq(eventReplays.id, replayId));

    // Get replay configuration
    const replayResults = await db
      .select()
      .from(eventReplays)
      .where(eq(eventReplays.id, replayId))
      .limit(1);

    if (replayResults.length === 0) {
      throw new Error("Replay not found");
    }

    const replay = replayResults[0];
    const topics = JSON.parse(replay.topics);

    // Simulate event processing (in production, this would consume from Kafka)
    const totalEvents = 1000; // Placeholder
    await db
      .update(eventReplays)
      .set({ eventsTotal: totalEvents })
      .where(eq(eventReplays.id, replayId));

    for (let i = 0; i < totalEvents; i++) {
      // Process event (placeholder)
      const eventOffset = `${i}`;
      const eventType = topics[i % topics.length];

      // Log event processing
      await db.insert(eventReplayLogs).values({
        replayId,
        eventOffset,
        eventType,
        eventData: JSON.stringify({ placeholder: true }),
        success: true,
      });

      // Update progress
      const progress = Math.floor(((i + 1) / totalEvents) * 100);
      await db
        .update(eventReplays)
        .set({
          eventsProcessed: i + 1,
          progress,
        })
        .where(eq(eventReplays.id, replayId));

      // Small delay to simulate processing
      if (i % 100 === 0) {
        await new Promise((resolve) => setTimeout(resolve, 10));
      }
    }

    // Mark as completed
    await db
      .update(eventReplays)
      .set({
        status: "completed",
        completedAt: new Date(),
      })
      .where(eq(eventReplays.id, replayId));

    console.log(`[EventReplay] Replay ${replayId} completed successfully`);
  } catch (error: any) {
    console.error(`[EventReplay] Replay ${replayId} failed:`, error);

    await db
      .update(eventReplays)
      .set({
        status: "failed",
        errorMessage: error.message,
        completedAt: new Date(),
      })
      .where(eq(eventReplays.id, replayId));
  }
}

/**
 * Get replay status
 */
export async function getReplayStatus(replayId: number): Promise<EventReplay | null> {
  const db = await getDb();
  if (!db) return null;

  try {
    const results = await db
      .select()
      .from(eventReplays)
      .where(eq(eventReplays.id, replayId))
      .limit(1);

    if (results.length === 0) {
      return null;
    }

    const r = results[0];
    return {
      id: r.id,
      replayName: r.replayName,
      snapshotId: r.snapshotId || undefined,
      topics: JSON.parse(r.topics),
      startOffset: r.startOffset || undefined,
      endOffset: r.endOffset || undefined,
      eventFilter: r.eventFilter ? JSON.parse(r.eventFilter) : undefined,
      status: r.status,
      progress: r.progress,
      eventsProcessed: r.eventsProcessed,
      eventsTotal: r.eventsTotal,
      errorMessage: r.errorMessage || undefined,
      startedBy: r.startedBy,
      startedAt: r.startedAt,
      completedAt: r.completedAt || undefined,
    };
  } catch (error) {
    console.error("[EventReplay] Failed to get replay status:", error);
    return null;
  }
}

/**
 * Get all replays
 */
export async function getAllReplays(): Promise<EventReplay[]> {
  const db = await getDb();
  if (!db) return [];

  try {
    const results = await db
      .select()
      .from(eventReplays)
      .orderBy(desc(eventReplays.startedAt))
      .limit(100);

    return results.map((r) => ({
      id: r.id,
      replayName: r.replayName,
      snapshotId: r.snapshotId || undefined,
      topics: JSON.parse(r.topics),
      startOffset: r.startOffset || undefined,
      endOffset: r.endOffset || undefined,
      eventFilter: r.eventFilter ? JSON.parse(r.eventFilter) : undefined,
      status: r.status,
      progress: r.progress,
      eventsProcessed: r.eventsProcessed,
      eventsTotal: r.eventsTotal,
      errorMessage: r.errorMessage || undefined,
      startedBy: r.startedBy,
      startedAt: r.startedAt,
      completedAt: r.completedAt || undefined,
    }));
  } catch (error) {
    console.error("[EventReplay] Failed to get replays:", error);
    return [];
  }
}

/**
 * Cancel replay
 */
export async function cancelReplay(replayId: number): Promise<void> {
  const db = await getDb();
  if (!db) return;

  try {
    await db
      .update(eventReplays)
      .set({
        status: "cancelled",
        completedAt: new Date(),
      })
      .where(eq(eventReplays.id, replayId));

    console.log(`[EventReplay] Replay ${replayId} cancelled`);
  } catch (error) {
    console.error("[EventReplay] Failed to cancel replay:", error);
  }
}

/**
 * Get replay logs
 */
export async function getReplayLogs(replayId: number, limit: number = 100): Promise<any[]> {
  const db = await getDb();
  if (!db) return [];

  try {
    const results = await db
      .select()
      .from(eventReplayLogs)
      .where(eq(eventReplayLogs.replayId, replayId))
      .orderBy(desc(eventReplayLogs.processedAt))
      .limit(limit);

    return results.map((r) => ({
      id: r.id,
      replayId: r.replayId,
      eventOffset: r.eventOffset,
      eventType: r.eventType,
      eventData: r.eventData ? JSON.parse(r.eventData) : undefined,
      processedAt: r.processedAt,
      success: r.success,
      errorMessage: r.errorMessage || undefined,
    }));
  } catch (error) {
    console.error("[EventReplay] Failed to get replay logs:", error);
    return [];
  }
}
