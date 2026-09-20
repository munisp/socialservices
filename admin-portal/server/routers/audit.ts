import crypto from "crypto";
import { TRPCError } from "@trpc/server";
import { z } from "zod";
import { adminProcedure, router } from "../_core/trpc";
import * as db from "../db";

const filtersSchema = z.object({ eventType: z.string().optional(), userId: z.number().optional(), targetId: z.string().optional(), startDate: z.date().optional(), endDate: z.date().optional(), searchQuery: z.string().optional(), limit: z.number().int().min(1).max(1000).default(50), offset: z.number().int().min(0).default(0) });

type AuditRow = Awaited<ReturnType<typeof db.getAllAdminActionAudit>>[number];
function details(row: AuditRow): Record<string, unknown> { return row.details && typeof row.details === "object" ? row.details as Record<string, unknown> : {}; }
function filterRows(rows: AuditRow[], input: z.infer<typeof filtersSchema>): AuditRow[] {
  const query = input.searchQuery?.toLowerCase();
  return rows.filter((row) => (!input.eventType || row.action === input.eventType) && (!input.userId || row.performedBy === input.userId) && (!input.startDate || row.performedAt >= input.startDate) && (!input.endDate || row.performedAt <= input.endDate) && (!input.targetId || Object.values(details(row)).some((value) => String(value) === input.targetId)) && (!query || [row.action, row.justification, JSON.stringify(row.details)].some((value) => value?.toLowerCase().includes(query))));
}
function escapeCsv(value: unknown): string { const text = value == null ? "" : typeof value === "string" ? value : JSON.stringify(value); return `"${text.replaceAll('"', '""')}"`; }

export const auditRouter = router({
  searchLogs: adminProcedure.input(filtersSchema).query(async ({ input }) => {
    const filtered = filterRows(await db.getAllAdminActionAudit(), input); return { logs: filtered.slice(input.offset, input.offset + input.limit), total: filtered.length, hasMore: input.offset + input.limit < filtered.length };
  }),
  getLogDetails: adminProcedure.input(z.object({ logId: z.number().int().positive() })).query(async ({ input }) => {
    const row = await db.getAdminActionAuditById(input.logId); if (!row) throw new TRPCError({ code: "NOT_FOUND", message: "Audit event not found" }); return row;
  }),
  getEntityAuditTrail: adminProcedure.input(z.object({ targetId: z.string(), limit: z.number().int().min(1).max(1000).default(100) })).query(async ({ input }) => filterRows(await db.getAllAdminActionAudit(), { targetId: input.targetId, limit: input.limit, offset: 0 }).slice(0, input.limit)),
  getUserActivity: adminProcedure.input(z.object({ userId: z.number(), startDate: z.date().optional(), endDate: z.date().optional(), limit: z.number().int().min(1).max(1000).default(100) })).query(async ({ input }) => {
    const actions = filterRows(await db.getAllAdminActionAudit(), { ...input, offset: 0 }).slice(0, input.limit); const byAction: Record<string, number> = {}; for (const row of actions) byAction[row.action] = (byAction[row.action] ?? 0) + 1; return { userId: input.userId, totalActions: actions.length, actions, summary: { byAction } };
  }),
  exportLogs: adminProcedure.input(z.object({ format: z.enum(["csv", "json"]), filters: filtersSchema.partial().default({}), startDate: z.date(), endDate: z.date() })).mutation(async ({ input, ctx }) => {
    const rows = filterRows(await db.getAllAdminActionAudit(), { ...input.filters, startDate: input.startDate, endDate: input.endDate, limit: 1000, offset: 0 }); const content = input.format === "json" ? JSON.stringify(rows, null, 2) : ["id,action,targetUserId,details,justification,performedBy,performedAt", ...rows.map((row) => [row.id, row.action, row.targetUserId, row.details, row.justification, row.performedBy, row.performedAt.toISOString()].map(escapeCsv).join(","))].join("\n"); const checksum = crypto.createHash("sha256").update(content).digest("hex"); await db.logAdminAction({ action: "audit_export", targetUserId: null, details: { format: input.format, recordCount: rows.length, checksum }, justification: "Administrator exported audit records", performedBy: ctx.user.id }); return { format: input.format, recordCount: rows.length, checksum, content };
  }),
  getComplianceDashboard: adminProcedure.input(z.object({ startDate: z.date(), endDate: z.date() })).query(async ({ input }) => {
    const rows = filterRows(await db.getAllAdminActionAudit(), { startDate: input.startDate, endDate: input.endDate, limit: 1000, offset: 0 }); const byAction: Record<string, number> = {}; const byActor: Record<string, number> = {}; for (const row of rows) { byAction[row.action] = (byAction[row.action] ?? 0) + 1; byActor[String(row.performedBy)] = (byActor[String(row.performedBy)] ?? 0) + 1; } return { totalEvents: rows.length, byAction, byActor, limitation: "The current audit schema does not capture standardized severity, outcome, IP address, or review status." };
  }),
  getStatistics: adminProcedure.input(z.object({ startDate: z.date(), endDate: z.date(), groupBy: z.enum(["day", "month"]).default("day") })).query(async ({ input }) => {
    const rows = filterRows(await db.getAllAdminActionAudit(), { startDate: input.startDate, endDate: input.endDate, limit: 1000, offset: 0 }); const groups = new Map<string, number>(); for (const row of rows) { const iso = row.performedAt.toISOString(); const key = input.groupBy === "month" ? iso.slice(0, 7) : iso.slice(0, 10); groups.set(key, (groups.get(key) ?? 0) + 1); } return { dataPoints: [...groups.entries()].sort().map(([timestamp, totalEvents]) => ({ timestamp, totalEvents })), summary: { totalEvents: rows.length } };
  }),
});
