/**
 * SQLite Database Service for Offline Storage
 * Provides local data persistence for the mobile app
 */

import SQLite from 'react-native-sqlite-storage';

SQLite.enablePromise(true);

const DATABASE_NAME = 'SocialProtection.db';
const DATABASE_VERSION = 1;

let db: SQLite.SQLiteDatabase | null = null;

export interface BeneficiaryRecord {
  id: string;
  firstName: string;
  lastName: string;
  dateOfBirth: string;
  gender: string;
  nationalId: string;
  phoneNumber?: string;
  email?: string;
  address?: string;
  latitude?: number;
  longitude?: number;
  householdId?: string;
  status: string;
  enrollmentDate?: string;
  biometricCaptured: boolean;
  documentsVerified: boolean;
  eligibilityScore?: number;
  proxyMeansScore?: number;
  version: number;
  syncStatus: 'synced' | 'pending' | 'conflict';
  localUpdatedAt: number;
  serverUpdatedAt?: number;
  createdAt: number;
}

export interface HouseholdRecord {
  id: string;
  headId: string;
  name: string;
  address?: string;
  latitude?: number;
  longitude?: number;
  memberCount: number;
  incomeLevel?: string;
  housingType?: string;
  proxyMeansScore?: number;
  status: string;
  version: number;
  syncStatus: 'synced' | 'pending' | 'conflict';
  localUpdatedAt: number;
  serverUpdatedAt?: number;
  createdAt: number;
}

export interface SyncQueueItem {
  id: string;
  entityType: string;
  entityId: string;
  operation: string;
  data: string;
  checksum: string;
  clientVersion: number;
  serverVersion: number;
  status: string;
  retryCount: number;
  errorMessage?: string;
  createdAt: number;
  syncedAt?: number;
}

export interface PMTSurveyRecord {
  id: string;
  householdId: string;
  beneficiaryId?: string;
  surveyData: string;
  pmtScore?: number;
  eligibilityCategory?: string;
  calculatedAt?: number;
  syncStatus: 'synced' | 'pending';
  createdAt: number;
  updatedAt: number;
}

export async function initDatabase(): Promise<void> {
  try {
    db = await SQLite.openDatabase({
      name: DATABASE_NAME,
      location: 'default',
    });

    await createTables();
    console.log('Database initialized successfully');
  } catch (error) {
    console.error('Failed to initialize database:', error);
    throw error;
  }
}

async function createTables(): Promise<void> {
  if (!db) throw new Error('Database not initialized');

  const queries = [
    `CREATE TABLE IF NOT EXISTS beneficiaries (
      id TEXT PRIMARY KEY,
      firstName TEXT NOT NULL,
      lastName TEXT NOT NULL,
      dateOfBirth TEXT,
      gender TEXT,
      nationalId TEXT,
      phoneNumber TEXT,
      email TEXT,
      address TEXT,
      latitude REAL,
      longitude REAL,
      householdId TEXT,
      status TEXT DEFAULT 'pending',
      enrollmentDate TEXT,
      biometricCaptured INTEGER DEFAULT 0,
      documentsVerified INTEGER DEFAULT 0,
      eligibilityScore REAL,
      proxyMeansScore REAL,
      version INTEGER DEFAULT 1,
      syncStatus TEXT DEFAULT 'pending',
      localUpdatedAt INTEGER,
      serverUpdatedAt INTEGER,
      createdAt INTEGER
    )`,
    `CREATE INDEX IF NOT EXISTS idx_beneficiaries_sync ON beneficiaries(syncStatus)`,
    `CREATE INDEX IF NOT EXISTS idx_beneficiaries_household ON beneficiaries(householdId)`,
    `CREATE INDEX IF NOT EXISTS idx_beneficiaries_national_id ON beneficiaries(nationalId)`,

    `CREATE TABLE IF NOT EXISTS households (
      id TEXT PRIMARY KEY,
      headId TEXT,
      name TEXT NOT NULL,
      address TEXT,
      latitude REAL,
      longitude REAL,
      memberCount INTEGER DEFAULT 1,
      incomeLevel TEXT,
      housingType TEXT,
      proxyMeansScore REAL,
      status TEXT DEFAULT 'active',
      version INTEGER DEFAULT 1,
      syncStatus TEXT DEFAULT 'pending',
      localUpdatedAt INTEGER,
      serverUpdatedAt INTEGER,
      createdAt INTEGER
    )`,
    `CREATE INDEX IF NOT EXISTS idx_households_sync ON households(syncStatus)`,

    `CREATE TABLE IF NOT EXISTS sync_queue (
      id TEXT PRIMARY KEY,
      entityType TEXT NOT NULL,
      entityId TEXT NOT NULL,
      operation TEXT NOT NULL,
      data TEXT,
      checksum TEXT,
      clientVersion INTEGER,
      serverVersion INTEGER,
      status TEXT DEFAULT 'pending',
      retryCount INTEGER DEFAULT 0,
      errorMessage TEXT,
      createdAt INTEGER,
      syncedAt INTEGER
    )`,
    `CREATE INDEX IF NOT EXISTS idx_sync_queue_status ON sync_queue(status)`,

    `CREATE TABLE IF NOT EXISTS pmt_surveys (
      id TEXT PRIMARY KEY,
      householdId TEXT NOT NULL,
      beneficiaryId TEXT,
      surveyData TEXT NOT NULL,
      pmtScore REAL,
      eligibilityCategory TEXT,
      calculatedAt INTEGER,
      syncStatus TEXT DEFAULT 'pending',
      createdAt INTEGER,
      updatedAt INTEGER
    )`,
    `CREATE INDEX IF NOT EXISTS idx_pmt_surveys_household ON pmt_surveys(householdId)`,

    `CREATE TABLE IF NOT EXISTS sync_log (
      id TEXT PRIMARY KEY,
      action TEXT NOT NULL,
      details TEXT,
      timestamp INTEGER
    )`,

    `CREATE TABLE IF NOT EXISTS conflicts (
      id TEXT PRIMARY KEY,
      entityType TEXT NOT NULL,
      entityId TEXT NOT NULL,
      clientData TEXT,
      serverData TEXT,
      resolution TEXT,
      resolvedData TEXT,
      status TEXT DEFAULT 'pending',
      createdAt INTEGER,
      resolvedAt INTEGER
    )`,

    `CREATE TABLE IF NOT EXISTS app_settings (
      key TEXT PRIMARY KEY,
      value TEXT
    )`,
  ];

  for (const query of queries) {
    await db.executeSql(query);
  }
}

export async function getDatabase(): Promise<SQLite.SQLiteDatabase> {
  if (!db) {
    await initDatabase();
  }
  return db!;
}

// Beneficiary operations
export async function saveBeneficiary(beneficiary: Omit<BeneficiaryRecord, 'createdAt' | 'localUpdatedAt'>): Promise<void> {
  const database = await getDatabase();
  const now = Date.now();

  await database.executeSql(
    `INSERT OR REPLACE INTO beneficiaries 
     (id, firstName, lastName, dateOfBirth, gender, nationalId, phoneNumber, email, 
      address, latitude, longitude, householdId, status, enrollmentDate, 
      biometricCaptured, documentsVerified, eligibilityScore, proxyMeansScore,
      version, syncStatus, localUpdatedAt, serverUpdatedAt, createdAt)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    [
      beneficiary.id,
      beneficiary.firstName,
      beneficiary.lastName,
      beneficiary.dateOfBirth,
      beneficiary.gender,
      beneficiary.nationalId,
      beneficiary.phoneNumber || null,
      beneficiary.email || null,
      beneficiary.address || null,
      beneficiary.latitude || null,
      beneficiary.longitude || null,
      beneficiary.householdId || null,
      beneficiary.status,
      beneficiary.enrollmentDate || null,
      beneficiary.biometricCaptured ? 1 : 0,
      beneficiary.documentsVerified ? 1 : 0,
      beneficiary.eligibilityScore || null,
      beneficiary.proxyMeansScore || null,
      beneficiary.version,
      'pending',
      now,
      beneficiary.serverUpdatedAt || null,
      now,
    ]
  );

  await addToSyncQueue('beneficiary', beneficiary.id, 'upsert', beneficiary);
}

export async function getBeneficiary(id: string): Promise<BeneficiaryRecord | null> {
  const database = await getDatabase();
  const [results] = await database.executeSql(
    'SELECT * FROM beneficiaries WHERE id = ?',
    [id]
  );

  if (results.rows.length === 0) return null;
  return rowToBeneficiary(results.rows.item(0));
}

export async function getAllBeneficiaries(): Promise<BeneficiaryRecord[]> {
  const database = await getDatabase();
  const [results] = await database.executeSql(
    'SELECT * FROM beneficiaries ORDER BY localUpdatedAt DESC'
  );

  const beneficiaries: BeneficiaryRecord[] = [];
  for (let i = 0; i < results.rows.length; i++) {
    beneficiaries.push(rowToBeneficiary(results.rows.item(i)));
  }
  return beneficiaries;
}

export async function searchBeneficiaries(query: string): Promise<BeneficiaryRecord[]> {
  const database = await getDatabase();
  const searchTerm = `%${query}%`;
  const [results] = await database.executeSql(
    `SELECT * FROM beneficiaries 
     WHERE firstName LIKE ? OR lastName LIKE ? OR nationalId LIKE ?
     ORDER BY localUpdatedAt DESC`,
    [searchTerm, searchTerm, searchTerm]
  );

  const beneficiaries: BeneficiaryRecord[] = [];
  for (let i = 0; i < results.rows.length; i++) {
    beneficiaries.push(rowToBeneficiary(results.rows.item(i)));
  }
  return beneficiaries;
}

export async function getPendingBeneficiaries(): Promise<BeneficiaryRecord[]> {
  const database = await getDatabase();
  const [results] = await database.executeSql(
    "SELECT * FROM beneficiaries WHERE syncStatus = 'pending'"
  );

  const beneficiaries: BeneficiaryRecord[] = [];
  for (let i = 0; i < results.rows.length; i++) {
    beneficiaries.push(rowToBeneficiary(results.rows.item(i)));
  }
  return beneficiaries;
}

function rowToBeneficiary(row: Record<string, unknown>): BeneficiaryRecord {
  return {
    id: row.id as string,
    firstName: row.firstName as string,
    lastName: row.lastName as string,
    dateOfBirth: row.dateOfBirth as string,
    gender: row.gender as string,
    nationalId: row.nationalId as string,
    phoneNumber: row.phoneNumber as string | undefined,
    email: row.email as string | undefined,
    address: row.address as string | undefined,
    latitude: row.latitude as number | undefined,
    longitude: row.longitude as number | undefined,
    householdId: row.householdId as string | undefined,
    status: row.status as string,
    enrollmentDate: row.enrollmentDate as string | undefined,
    biometricCaptured: Boolean(row.biometricCaptured),
    documentsVerified: Boolean(row.documentsVerified),
    eligibilityScore: row.eligibilityScore as number | undefined,
    proxyMeansScore: row.proxyMeansScore as number | undefined,
    version: row.version as number,
    syncStatus: row.syncStatus as 'synced' | 'pending' | 'conflict',
    localUpdatedAt: row.localUpdatedAt as number,
    serverUpdatedAt: row.serverUpdatedAt as number | undefined,
    createdAt: row.createdAt as number,
  };
}

// Household operations
export async function saveHousehold(household: Omit<HouseholdRecord, 'createdAt' | 'localUpdatedAt'>): Promise<void> {
  const database = await getDatabase();
  const now = Date.now();

  await database.executeSql(
    `INSERT OR REPLACE INTO households 
     (id, headId, name, address, latitude, longitude, memberCount, incomeLevel,
      housingType, proxyMeansScore, status, version, syncStatus, localUpdatedAt, 
      serverUpdatedAt, createdAt)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    [
      household.id,
      household.headId,
      household.name,
      household.address || null,
      household.latitude || null,
      household.longitude || null,
      household.memberCount,
      household.incomeLevel || null,
      household.housingType || null,
      household.proxyMeansScore || null,
      household.status,
      household.version,
      'pending',
      now,
      household.serverUpdatedAt || null,
      now,
    ]
  );

  await addToSyncQueue('household', household.id, 'upsert', household);
}

export async function getHousehold(id: string): Promise<HouseholdRecord | null> {
  const database = await getDatabase();
  const [results] = await database.executeSql(
    'SELECT * FROM households WHERE id = ?',
    [id]
  );

  if (results.rows.length === 0) return null;
  return rowToHousehold(results.rows.item(0));
}

export async function getAllHouseholds(): Promise<HouseholdRecord[]> {
  const database = await getDatabase();
  const [results] = await database.executeSql(
    'SELECT * FROM households ORDER BY localUpdatedAt DESC'
  );

  const households: HouseholdRecord[] = [];
  for (let i = 0; i < results.rows.length; i++) {
    households.push(rowToHousehold(results.rows.item(i)));
  }
  return households;
}

function rowToHousehold(row: Record<string, unknown>): HouseholdRecord {
  return {
    id: row.id as string,
    headId: row.headId as string,
    name: row.name as string,
    address: row.address as string | undefined,
    latitude: row.latitude as number | undefined,
    longitude: row.longitude as number | undefined,
    memberCount: row.memberCount as number,
    incomeLevel: row.incomeLevel as string | undefined,
    housingType: row.housingType as string | undefined,
    proxyMeansScore: row.proxyMeansScore as number | undefined,
    status: row.status as string,
    version: row.version as number,
    syncStatus: row.syncStatus as 'synced' | 'pending' | 'conflict',
    localUpdatedAt: row.localUpdatedAt as number,
    serverUpdatedAt: row.serverUpdatedAt as number | undefined,
    createdAt: row.createdAt as number,
  };
}

// PMT Survey operations
export async function savePMTSurvey(survey: Omit<PMTSurveyRecord, 'createdAt' | 'updatedAt'>): Promise<void> {
  const database = await getDatabase();
  const now = Date.now();

  await database.executeSql(
    `INSERT OR REPLACE INTO pmt_surveys 
     (id, householdId, beneficiaryId, surveyData, pmtScore, eligibilityCategory,
      calculatedAt, syncStatus, createdAt, updatedAt)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    [
      survey.id,
      survey.householdId,
      survey.beneficiaryId || null,
      survey.surveyData,
      survey.pmtScore || null,
      survey.eligibilityCategory || null,
      survey.calculatedAt || null,
      'pending',
      now,
      now,
    ]
  );

  await addToSyncQueue('pmt_survey', survey.id, 'create', survey);
}

export async function getPMTSurveysByHousehold(householdId: string): Promise<PMTSurveyRecord[]> {
  const database = await getDatabase();
  const [results] = await database.executeSql(
    'SELECT * FROM pmt_surveys WHERE householdId = ? ORDER BY createdAt DESC',
    [householdId]
  );

  const surveys: PMTSurveyRecord[] = [];
  for (let i = 0; i < results.rows.length; i++) {
    const row = results.rows.item(i);
    surveys.push({
      id: row.id,
      householdId: row.householdId,
      beneficiaryId: row.beneficiaryId,
      surveyData: row.surveyData,
      pmtScore: row.pmtScore,
      eligibilityCategory: row.eligibilityCategory,
      calculatedAt: row.calculatedAt,
      syncStatus: row.syncStatus,
      createdAt: row.createdAt,
      updatedAt: row.updatedAt,
    });
  }
  return surveys;
}

// Sync queue operations
async function addToSyncQueue(
  entityType: string,
  entityId: string,
  operation: string,
  data: unknown
): Promise<void> {
  const database = await getDatabase();
  const now = Date.now();
  const id = `${entityType}-${entityId}-${now}`;
  const dataStr = JSON.stringify(data);
  const checksum = generateChecksum(dataStr);

  await database.executeSql(
    `INSERT INTO sync_queue 
     (id, entityType, entityId, operation, data, checksum, clientVersion, serverVersion, status, retryCount, createdAt)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    [id, entityType, entityId, operation, dataStr, checksum, 1, 0, 'pending', 0, now]
  );
}

export async function getSyncQueue(): Promise<SyncQueueItem[]> {
  const database = await getDatabase();
  const [results] = await database.executeSql(
    "SELECT * FROM sync_queue WHERE status = 'pending' ORDER BY createdAt ASC"
  );

  const items: SyncQueueItem[] = [];
  for (let i = 0; i < results.rows.length; i++) {
    const row = results.rows.item(i);
    items.push({
      id: row.id,
      entityType: row.entityType,
      entityId: row.entityId,
      operation: row.operation,
      data: row.data,
      checksum: row.checksum,
      clientVersion: row.clientVersion,
      serverVersion: row.serverVersion,
      status: row.status,
      retryCount: row.retryCount,
      errorMessage: row.errorMessage,
      createdAt: row.createdAt,
      syncedAt: row.syncedAt,
    });
  }
  return items;
}

export async function updateSyncQueueItem(id: string, status: string, errorMessage?: string): Promise<void> {
  const database = await getDatabase();
  await database.executeSql(
    'UPDATE sync_queue SET status = ?, errorMessage = ?, syncedAt = ? WHERE id = ?',
    [status, errorMessage || null, status === 'synced' ? Date.now() : null, id]
  );
}

export async function clearSyncedItems(): Promise<void> {
  const database = await getDatabase();
  await database.executeSql("DELETE FROM sync_queue WHERE status = 'synced'");
}

// Stats
export async function getSyncStats(): Promise<{
  beneficiaries: { total: number; pending: number; synced: number };
  households: { total: number; pending: number; synced: number };
  queueSize: number;
}> {
  const database = await getDatabase();

  const [beneficiaryResults] = await database.executeSql(
    `SELECT 
       COUNT(*) as total,
       SUM(CASE WHEN syncStatus = 'pending' THEN 1 ELSE 0 END) as pending,
       SUM(CASE WHEN syncStatus = 'synced' THEN 1 ELSE 0 END) as synced
     FROM beneficiaries`
  );

  const [householdResults] = await database.executeSql(
    `SELECT 
       COUNT(*) as total,
       SUM(CASE WHEN syncStatus = 'pending' THEN 1 ELSE 0 END) as pending,
       SUM(CASE WHEN syncStatus = 'synced' THEN 1 ELSE 0 END) as synced
     FROM households`
  );

  const [queueResults] = await database.executeSql(
    "SELECT COUNT(*) as count FROM sync_queue WHERE status = 'pending'"
  );

  const bRow = beneficiaryResults.rows.item(0);
  const hRow = householdResults.rows.item(0);
  const qRow = queueResults.rows.item(0);

  return {
    beneficiaries: {
      total: bRow.total || 0,
      pending: bRow.pending || 0,
      synced: bRow.synced || 0,
    },
    households: {
      total: hRow.total || 0,
      pending: hRow.pending || 0,
      synced: hRow.synced || 0,
    },
    queueSize: qRow.count || 0,
  };
}

// Settings
export async function getSetting(key: string): Promise<string | null> {
  const database = await getDatabase();
  const [results] = await database.executeSql(
    'SELECT value FROM app_settings WHERE key = ?',
    [key]
  );

  if (results.rows.length === 0) return null;
  return results.rows.item(0).value;
}

export async function setSetting(key: string, value: string): Promise<void> {
  const database = await getDatabase();
  await database.executeSql(
    'INSERT OR REPLACE INTO app_settings (key, value) VALUES (?, ?)',
    [key, value]
  );
}

function generateChecksum(data: string): string {
  let hash = 0;
  for (let i = 0; i < data.length; i++) {
    const char = data.charCodeAt(i);
    hash = ((hash << 5) - hash) + char;
    hash = hash & hash;
  }
  return hash.toString(16);
}

// Conflict tracking operations
export interface ConflictRecord {
  id: string;
  entityType: string;
  entityId: string;
  clientData: string;
  serverData: string;
  resolution?: string;
  resolvedData?: string;
  status: 'pending' | 'resolved';
  createdAt: number;
  resolvedAt?: number;
}

export async function saveConflict(
  entityType: string,
  entityId: string,
  clientData: unknown,
  serverData: unknown
): Promise<string> {
  const database = await getDatabase();
  const now = Date.now();
  const id = `conflict-${entityType}-${entityId}-${now}`;

  await database.executeSql(
    `INSERT INTO conflicts 
     (id, entityType, entityId, clientData, serverData, status, createdAt)
     VALUES (?, ?, ?, ?, ?, ?, ?)`,
    [id, entityType, entityId, JSON.stringify(clientData), JSON.stringify(serverData), 'pending', now]
  );

  return id;
}

export async function getConflicts(): Promise<ConflictRecord[]> {
  const database = await getDatabase();
  const [results] = await database.executeSql(
    "SELECT * FROM conflicts WHERE status = 'pending' ORDER BY createdAt DESC"
  );

  const conflicts: ConflictRecord[] = [];
  for (let i = 0; i < results.rows.length; i++) {
    const row = results.rows.item(i);
    conflicts.push({
      id: row.id,
      entityType: row.entityType,
      entityId: row.entityId,
      clientData: row.clientData,
      serverData: row.serverData,
      resolution: row.resolution,
      resolvedData: row.resolvedData,
      status: row.status,
      createdAt: row.createdAt,
      resolvedAt: row.resolvedAt,
    });
  }
  return conflicts;
}

export async function getConflictCount(): Promise<number> {
  const database = await getDatabase();
  const [results] = await database.executeSql(
    "SELECT COUNT(*) as count FROM conflicts WHERE status = 'pending'"
  );
  return results.rows.item(0).count || 0;
}

export async function resolveConflict(
  conflictId: string,
  resolution: 'client' | 'server' | 'merge',
  resolvedData: unknown
): Promise<void> {
  const database = await getDatabase();
  await database.executeSql(
    `UPDATE conflicts SET status = 'resolved', resolution = ?, resolvedData = ?, resolvedAt = ? WHERE id = ?`,
    [resolution, JSON.stringify(resolvedData), Date.now(), conflictId]
  );
}

export async function closeDatabase(): Promise<void> {
  if (db) {
    await db.close();
    db = null;
  }
}
