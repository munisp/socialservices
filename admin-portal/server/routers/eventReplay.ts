import { router, protectedProcedure } from "../_core/trpc";
import { z } from "zod";
import {
  createEventSnapshot,
  getAllSnapshots,
  startEventReplay,
  getReplayStatus,
  getAllReplays,
  cancelReplay,
  getReplayLogs,
} from "../services/eventReplay";

export const eventReplayRouter = router({
  // Create snapshot
  createSnapshot: protectedProcedure
    .input(
      z.object({
        snapshotName: z.string(),
        description: z.string().optional(),
        topics: z.array(z.string()),
        startOffset: z.string(),
        endOffset: z.string(),
        eventCount: z.number(),
      })
    )
    .mutation(async ({ input, ctx }) => {
      const snapshotId = await createEventSnapshot(
        input.snapshotName,
        input.description,
        input.topics,
        input.startOffset,
        input.endOffset,
        input.eventCount,
        ctx.user.id
      );
      return { snapshotId };
    }),

  // Get all snapshots
  getAllSnapshots: protectedProcedure.query(async () => {
    return await getAllSnapshots();
  }),

  // Start replay
  startReplay: protectedProcedure
    .input(
      z.object({
        replayName: z.string(),
        topics: z.array(z.string()),
        snapshotId: z.number().optional(),
        startOffset: z.string().optional(),
        endOffset: z.string().optional(),
        eventFilter: z.any().optional(),
      })
    )
    .mutation(async ({ input, ctx }) => {
      const replayId = await startEventReplay(input.replayName, input.topics, ctx.user.id, {
        snapshotId: input.snapshotId,
        startOffset: input.startOffset,
        endOffset: input.endOffset,
        eventFilter: input.eventFilter,
      });
      return { replayId };
    }),

  // Get replay status
  getReplayStatus: protectedProcedure
    .input(z.object({ replayId: z.number() }))
    .query(async ({ input }) => {
      return await getReplayStatus(input.replayId);
    }),

  // Get all replays
  getAllReplays: protectedProcedure.query(async () => {
    return await getAllReplays();
  }),

  // Cancel replay
  cancelReplay: protectedProcedure
    .input(z.object({ replayId: z.number() }))
    .mutation(async ({ input }) => {
      await cancelReplay(input.replayId);
      return { success: true };
    }),

  // Get replay logs
  getReplayLogs: protectedProcedure
    .input(
      z.object({
        replayId: z.number(),
        limit: z.number().optional(),
      })
    )
    .query(async ({ input }) => {
      return await getReplayLogs(input.replayId, input.limit);
    }),
});
