import { Connection, Client, WorkflowClient } from "@temporalio/client";

/**
 * Temporal Workflow Orchestration Integration
 * Durable execution for long-running workflows and business processes
 */

let temporalConnection: Connection | null = null;
let temporalClient: WorkflowClient | null = null;

/**
 * Get Temporal connection
 */
export async function getTemporalConnection(): Promise<Connection | null> {
  if (temporalConnection) {
    return temporalConnection;
  }

  const temporalAddress = process.env.TEMPORAL_ADDRESS || "localhost:7233";

  if (!process.env.TEMPORAL_ADDRESS) {
    console.warn("[Temporal] TEMPORAL_ADDRESS not configured, workflow orchestration disabled");
    return null;
  }

  try {
    temporalConnection = await Connection.connect({
      address: temporalAddress,
    });

    console.log(`[Temporal] Connected to ${temporalAddress}`);
    return temporalConnection;
  } catch (error) {
    console.error("[Temporal] Failed to connect:", error);
    return null;
  }
}

/**
 * Get Temporal client
 */
export async function getTemporalClient(): Promise<WorkflowClient | null> {
  if (temporalClient) {
    return temporalClient;
  }

  const connection = await getTemporalConnection();
  if (!connection) {
    return null;
  }

  try {
    temporalClient = new WorkflowClient({
      connection,
      namespace: process.env.TEMPORAL_NAMESPACE || "default",
    });

    console.log("[Temporal] Client initialized");
    return temporalClient;
  } catch (error) {
    console.error("[Temporal] Failed to create client:", error);
    return null;
  }
}

/**
 * Start workflow
 */
export async function startWorkflow<T>(
  workflowType: string,
  workflowId: string,
  args: any[],
  options?: {
    taskQueue?: string;
    searchAttributes?: Record<string, any>;
  }
): Promise<string | null> {
  const client = await getTemporalClient();
  if (!client) {
    console.warn(`[Temporal] Client not available, cannot start workflow: ${workflowType}`);
    return null;
  }

  try {
    const handle = await client.start(workflowType as any, {
      workflowId,
      taskQueue: options?.taskQueue || "default",
      args,
      searchAttributes: options?.searchAttributes,
    });

    console.log(`[Temporal] Workflow started: ${workflowType} (${workflowId})`);
    return handle.workflowId;
  } catch (error) {
    console.error(`[Temporal] Failed to start workflow: ${workflowType}`, error);
    return null;
  }
}

/**
 * Get workflow result
 */
export async function getWorkflowResult<T>(workflowId: string): Promise<T | null> {
  const client = await getTemporalClient();
  if (!client) {
    return null;
  }

  try {
    const handle = client.getHandle(workflowId);
    const result = await handle.result();
    console.log(`[Temporal] Workflow completed: ${workflowId}`);
    return result as T;
  } catch (error) {
    console.error(`[Temporal] Failed to get workflow result: ${workflowId}`, error);
    return null;
  }
}

/**
 * Cancel workflow
 */
export async function cancelWorkflow(workflowId: string): Promise<void> {
  const client = await getTemporalClient();
  if (!client) {
    return;
  }

  try {
    const handle = client.getHandle(workflowId);
    await handle.cancel();
    console.log(`[Temporal] Workflow cancelled: ${workflowId}`);
  } catch (error) {
    console.error(`[Temporal] Failed to cancel workflow: ${workflowId}`, error);
  }
}

/**
 * Terminate workflow
 */
export async function terminateWorkflow(workflowId: string, reason?: string): Promise<void> {
  const client = await getTemporalClient();
  if (!client) {
    return;
  }

  try {
    const handle = client.getHandle(workflowId);
    await handle.terminate(reason);
    console.log(`[Temporal] Workflow terminated: ${workflowId}`);
  } catch (error) {
    console.error(`[Temporal] Failed to terminate workflow: ${workflowId}`, error);
  }
}

/**
 * Query workflow
 */
export async function queryWorkflow<T>(
  workflowId: string,
  queryType: string,
  ...args: any[]
): Promise<T | null> {
  const client = await getTemporalClient();
  if (!client) {
    return null;
  }

  try {
    const handle = client.getHandle(workflowId);
    const result = await handle.query(queryType as any, ...args);
    console.log(`[Temporal] Workflow queried: ${workflowId}.${queryType}`);
    return result as T;
  } catch (error) {
    console.error(`[Temporal] Failed to query workflow: ${workflowId}.${queryType}`, error);
    return null;
  }
}

/**
 * Signal workflow
 */
export async function signalWorkflow(
  workflowId: string,
  signalName: string,
  ...args: any[]
): Promise<void> {
  const client = await getTemporalClient();
  if (!client) {
    return;
  }

  try {
    const handle = client.getHandle(workflowId);
    await handle.signal(signalName as any, ...args);
    console.log(`[Temporal] Workflow signaled: ${workflowId}.${signalName}`);
  } catch (error) {
    console.error(`[Temporal] Failed to signal workflow: ${workflowId}.${signalName}`, error);
  }
}

/**
 * Example workflow definitions
 */

// Disbursement approval workflow
export async function startDisbursementApprovalWorkflow(
  disbursementId: string,
  amount: number,
  beneficiaryId: string
): Promise<string | null> {
  return startWorkflow(
    "disbursementApproval",
    `disbursement-${disbursementId}`,
    [{ disbursementId, amount, beneficiaryId }],
    {
      taskQueue: "disbursement-queue",
      searchAttributes: {
        disbursementId: [disbursementId],
        beneficiaryId: [beneficiaryId],
      },
    }
  );
}

// KYC verification workflow
export async function startKycVerificationWorkflow(
  beneficiaryId: string,
  documents: string[]
): Promise<string | null> {
  return startWorkflow(
    "kycVerification",
    `kyc-${beneficiaryId}`,
    [{ beneficiaryId, documents }],
    {
      taskQueue: "kyc-queue",
      searchAttributes: {
        beneficiaryId: [beneficiaryId],
      },
    }
  );
}

// Fraud investigation workflow
export async function startFraudInvestigationWorkflow(
  alertId: string,
  transactionId: string,
  severity: string
): Promise<string | null> {
  return startWorkflow(
    "fraudInvestigation",
    `fraud-${alertId}`,
    [{ alertId, transactionId, severity }],
    {
      taskQueue: "fraud-queue",
      searchAttributes: {
        alertId: [alertId],
        severity: [severity],
      },
    }
  );
}

// Benefit card issuance workflow
export async function startCardIssuanceWorkflow(
  beneficiaryId: string,
  programId: string
): Promise<string | null> {
  return startWorkflow(
    "cardIssuance",
    `card-${beneficiaryId}-${programId}`,
    [{ beneficiaryId, programId }],
    {
      taskQueue: "card-queue",
      searchAttributes: {
        beneficiaryId: [beneficiaryId],
        programId: [programId],
      },
    }
  );
}

// Monthly disbursement workflow
export async function startMonthlyDisbursementWorkflow(
  month: string,
  programId: string
): Promise<string | null> {
  return startWorkflow(
    "monthlyDisbursement",
    `monthly-disbursement-${month}-${programId}`,
    [{ month, programId }],
    {
      taskQueue: "disbursement-queue",
      searchAttributes: {
        month: [month],
        programId: [programId],
      },
    }
  );
}

/**
 * Workflow status helpers
 */

export async function getDisbursementApprovalStatus(disbursementId: string): Promise<any> {
  return queryWorkflow(`disbursement-${disbursementId}`, "getStatus");
}

export async function approveDisbursement(disbursementId: string, approverId: string): Promise<void> {
  await signalWorkflow(`disbursement-${disbursementId}`, "approve", approverId);
}

export async function rejectDisbursement(disbursementId: string, reason: string): Promise<void> {
  await signalWorkflow(`disbursement-${disbursementId}`, "reject", reason);
}

/**
 * Health check
 */
export async function temporalHealthCheck(): Promise<boolean> {
  try {
    const connection = await getTemporalConnection();
    return connection !== null;
  } catch (error) {
    console.error("[Temporal] Health check failed:", error);
    return false;
  }
}

/**
 * Close connection
 */
export async function closeTemporalConnection(): Promise<void> {
  if (temporalConnection) {
    await temporalConnection.close();
    temporalConnection = null;
    temporalClient = null;
    console.log("[Temporal] Connection closed");
  }
}

// Graceful shutdown
process.on("SIGTERM", async () => {
  await closeTemporalConnection();
});

process.on("SIGINT", async () => {
  await closeTemporalConnection();
});
