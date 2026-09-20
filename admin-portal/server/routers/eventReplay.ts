import { router, protectedProcedure, adminProcedure } from "../_core/trpc";
import { TRPCError } from "@trpc/server";
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
  createSnapshot: adminProcedure
    .input(
      z.object({
        snapshotName: z.string(),
        description: z.string().optional(),
        topics: z.array(z.string().min(1)).min(1),
        startOffset: z.string().regex(/^\d+$/),
        endOffset: z.string().regex(/^\d+$/),
        eventCount: z.number().int().nonnegative(),
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
  startReplay: adminProcedure
    .input(
      z.object({
        replayName: z.string(),
        topics: z.array(z.string().min(1)).min(1),
        snapshotId: z.number().optional(),
        startOffset: z.string().regex(/^\d+$/).optional(),
        endOffset: z.string().regex(/^\d+$/),
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
      if (!replayId) throw new TRPCError({ code: "PRECONDITION_FAILED", message: "Replay could not be started; verify Kafka connectivity and the bounded offset range" });
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
  cancelReplay: adminProcedure
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
