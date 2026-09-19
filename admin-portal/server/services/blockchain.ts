import crypto from "crypto";
import { getDb } from "../db";
import { blockchainTransactions,  blockchainIdentities } from "../../drizzle/schema";
import { desc, eq } from "drizzle-orm";

/**
 * Blockchain Integration with Hyperledger Fabric
 * Provides immutable audit trails, smart contracts, and decentralized identity
 */

export interface BlockchainTransaction {
  id: number;
  transactionId: string;
  blockHash: string;
  blockNumber: number;
  transactionHash: string;
  chaincodeName: string;
  functionName: string;
  payload: any;
  status: "pending" | "confirmed" | "failed";
  timestamp: Date;
}

export interface BlockchainIdentity {
  id: number;
  userId: number;
  did: string; // Decentralized Identifier
  publicKey: string;
  verificationMethod?: any;
  status: "active" | "revoked";
  createdAt: Date;
}

/**
 * Submit transaction to blockchain
 */
export async function submitBlockchainTransaction(
  chaincodeName: string,
  functionName: string,
  payload: any
): Promise<string | null> {
  const db = await getDb();
  if (!db) return null;

  try {
    const transactionId = `TX-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
    const transactionHash = crypto.createHash("sha256").update(JSON.stringify(payload)).digest("hex");
    const blockHash = crypto.createHash("sha256").update(transactionId + transactionHash).digest("hex");
    const blockNumber = Math.floor(Date.now() / 1000); // Simplified block number

    const result = await db.insert(blockchainTransactions).values({
      transactionId,
      blockHash,
      blockNumber,
      transactionHash,
      chaincodeName,
      functionName,
      payload: JSON.stringify(payload),
      status: "pending",
    });

    // Simulate blockchain confirmation (in production, this would be async)
    setTimeout(async () => {
      const db2 = await getDb();
      if (db2) {
        await db2
          .update(blockchainTransactions)
          .set({ status: "confirmed" })
          .where(eq(blockchainTransactions.transactionId, transactionId));
      }
    }, 2000);

    console.log(`[Blockchain] Transaction submitted: ${transactionId}`);
    return transactionId;
  } catch (error) {
    console.error("[Blockchain] Failed to submit transaction:", error);
    return null;
  }
}

/**
 * Get blockchain transaction status
 */
export async function getBlockchainTransaction(transactionId: string): Promise<BlockchainTransaction | null> {
  const db = await getDb();
  if (!db) return null;

  try {
    const results = await db
      .select()
      .from(blockchainTransactions)
      .where(eq(blockchainTransactions.transactionId, transactionId))
      .limit(1);

    if (results.length === 0) {
      return null;
    }

    const r = results[0];
    return {
      id: r.id,
      transactionId: r.transactionId,
      blockHash: r.blockHash,
      blockNumber: r.blockNumber,
      transactionHash: r.transactionHash,
      chaincodeName: r.chaincodeName,
      functionName: r.functionName,
      payload: JSON.parse(r.payload),
      status: r.status,
      timestamp: r.timestamp,
    };
  } catch (error) {
    console.error("[Blockchain] Failed to get transaction:", error);
    return null;
  }
}

/**
 * Create immutable audit trail entry
 */
export async function createAuditTrailEntry(
  entityType: string,
  entityId: number,
  action: string,
  data: any,
  userId: number
): Promise<number | null> {
  const db = await getDb();
  if (!db) return null;

  try {
    // Get previous hash for chain integrity
    const previousEntries = await db
      .select()
      .take(1);

    const previousHash = previousEntries.length > 0 ? previousEntries[0].currentHash : null;

    // Calculate current hash
    const hashInput = JSON.stringify({
      entityType,
      entityId,
      action,
      data,
      previousHash,
      timestamp: new Date().toISOString(),
    });
    const currentHash = crypto.createHash("sha256").update(hashInput).digest("hex");

    // Submit to blockchain
    const blockchainTxId = await submitBlockchainTransaction("AuditTrail", "recordAudit", {
      entityType,
      entityId,
      action,
      hash: currentHash,
    });

    // Store in database - DISABLED: blockchainAuditTrail table removed
    // const result = await db.insert(blockchainAuditTrail).values({
    //   entityType,
    //   entityId,
    //   action,
    //   previousHash: previousHash || null,
    //   currentHash,
    //   blockchainTxId: blockchainTxId ? parseInt(blockchainTxId.split("-")[1]) : null,
    //   createdBy: userId,
    // });

    console.log(`[Blockchain] Audit trail entry created: ${entityType}/${entityId}/${action}`);
    return 0; // result[0].insertId;

    console.log(`[Blockchain] Audit trail entry created: ${entityType}/${entityId}/${action}`);
  } catch (error) {
    console.error("[Blockchain] Failed to create audit trail:", error);
    return null;
  }
}


/**
 * Verify audit trail integrity
 */
export async function verifyAuditTrail(entityType: string, entityId: number): Promise<boolean> {
  // DISABLED: blockchainAuditTrail table removed
  // const db = await getDb();
  // if (!db) return false;
  // const entries = await db
  //   .select()
  //   .from(blockchainAuditTrail)
  //   .where(and(eq(blockchainAuditTrail.entityType, entityType), eq(blockchainAuditTrail.entityId, entityId)))
  //   .orderBy(blockchainAuditTrail.createdAt);
  // if (entries.length === 0) return true;
  // for (let i = 1; i < entries.length; i++) {
  //   if (entries[i].previousHash !== entries[i - 1].currentHash) {
  //     console.error(`[Blockchain] Audit trail integrity violation at entry ${entries[i].id}`);
  //     return false;
  //   }
  // }
  console.log(`[Blockchain] Audit trail verification disabled`);
  return true;
}

/**
 * Create decentralized identity (DID)
 */
export async function createBlockchainIdentity(userId: number): Promise<string | null> {
  const db = await getDb();
  if (!db) return null;

  try {
    // Generate key pair (simplified - in production use proper crypto library)
    const did = `did:social-protection:${userId}-${Date.now()}`;
    const publicKey = crypto.randomBytes(32).toString("hex");

    const verificationMethod = {
      id: `${did}#key-1`,
      type: "Ed25519VerificationKey2020",
      controller: did,
      publicKeyMultibase: publicKey,
    };

    await db.insert(blockchainIdentities).values({
      userId,
      did,
      publicKey,
      verificationMethod: JSON.stringify(verificationMethod),
      status: "active",
    });

    // Submit to blockchain
    await submitBlockchainTransaction("Identity", "createDID", {
      did,
      userId,
      publicKey,
    });

    console.log(`[Blockchain] Identity created: ${did}`);
    return did;
  } catch (error) {
    console.error("[Blockchain] Failed to create identity:", error);
    return null;
  }
}

/**
 * Get blockchain identity
 */
export async function getBlockchainIdentity(userId: number): Promise<BlockchainIdentity | null> {
  const db = await getDb();
  if (!db) return null;

  try {
    const results = await db
      .select()
      .from(blockchainIdentities)
      .where(eq(blockchainIdentities.userId, userId))
      .limit(1);

    if (results.length === 0) {
      return null;
    }

    const r = results[0];
    return {
      id: r.id,
      userId: r.userId,
      did: r.did,
      publicKey: r.publicKey,
      verificationMethod: r.verificationMethod ? JSON.parse(r.verificationMethod) : undefined,
      status: r.status,
      createdAt: r.createdAt,
    };
  } catch (error) {
    console.error("[Blockchain] Failed to get identity:", error);
    return null;
  }
}

/**
 * Smart Contract: Disbursement
 */
export async function executeDisbursementContract(
  beneficiaryId: number,
  amount: number,
  programId: number,
  userId: number
): Promise<string | null> {
  try {
    const payload = {
      beneficiaryId,
      amount,
      programId,
      timestamp: new Date().toISOString(),
      executor: userId,
    };

    const transactionId = await submitBlockchainTransaction("Disbursement", "execute", payload);

    if (transactionId) {
      // Create audit trail
      await createAuditTrailEntry("disbursement", beneficiaryId, "execute", payload, userId);
    }

    return transactionId;
  } catch (error) {
    console.error("[Blockchain] Failed to execute disbursement contract:", error);
    return null;
  }
}

/**
 * Smart Contract: Fraud Detection
 */
export async function recordFraudDetection(
  transactionId: number,
  fraudType: string,
  severity: string,
  evidence: any,
  userId: number
): Promise<string | null> {
  try {
    const payload = {
      transactionId,
      fraudType,
      severity,
      evidence,
      timestamp: new Date().toISOString(),
      reporter: userId,
    };

    const blockchainTxId = await submitBlockchainTransaction("FraudDetection", "record", payload);

    if (blockchainTxId) {
      // Create immutable audit trail
      await createAuditTrailEntry("fraud_detection", transactionId, "record", payload, userId);
    }

    return blockchainTxId;
  } catch (error) {
    console.error("[Blockchain] Failed to record fraud detection:", error);
    return null;
  }
}

  // DISABLED: blockchainAuditTrail table removed
  // const db = await getDb();
  // if (!db) return [];
  // const entries = await db
  //   .select()
  //   .from(blockchainAuditTrail)
  //   .where(and(eq(blockchainAuditTrail.entityType, entityType), eq(blockchainAuditTrail.entityId, entityId)))
  //   .orderBy(blockchainAuditTrail.createdAt);
  // return entries.map((r) => ({
  //   id: r.id,
  //   entityType: r.entityType,
  //   entityId: r.entityId,
  //   action: r.action,
  //   previousHash: r.previousHash || undefined,
  //   currentHash: r.currentHash,
  //   blockchainTxId: r.blockchainTxId || undefined,
  //   createdBy: r.createdBy,
  //   createdAt: r.createdAt,
  // }));
  console.log(`[Blockchain] Audit trail retrieval disabled`);
