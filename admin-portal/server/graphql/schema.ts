import gql from "graphql-tag";

/**
 * GraphQL Schema for Social Protection Platform
 * Provides flexible querying and real-time subscriptions
 */

export const typeDefs = gql`
  # User Types
  type User {
    id: ID!
    openId: String!
    name: String
    email: String
    role: UserRole!
    createdAt: String!
    lastSignedIn: String!
  }

  enum UserRole {
    admin
    user
  }

  # Beneficiary Types
  type Beneficiary {
    id: ID!
    firstName: String!
    lastName: String!
    dateOfBirth: String
    gender: String
    phoneNumber: String
    email: String
    nationalId: String
    address: String
    city: String
    state: String
    postalCode: String
    country: String
    status: BeneficiaryStatus!
    kycStatus: KYCStatus!
    enrolledAt: String!
  }

  enum BeneficiaryStatus {
    active
    inactive
    suspended
  }

  enum KYCStatus {
    pending
    verified
    rejected
  }

  # Program Types
  type Program {
    id: ID!
    name: String!
    description: String
    status: ProgramStatus!
    budget: Float
    startDate: String
    endDate: String
    createdAt: String!
  }

  enum ProgramStatus {
    draft
    active
    paused
    completed
  }

  # Transaction Types
  type Transaction {
    id: ID!
    beneficiaryId: ID!
    programId: ID
    amount: Float!
    currency: String!
    transactionType: TransactionType!
    status: TransactionStatus!
    merchantName: String
    merchantCategory: String
    description: String
    timestamp: String!
  }

  enum TransactionType {
    disbursement
    purchase
    refund
    adjustment
  }

  enum TransactionStatus {
    pending
    completed
    failed
    reversed
  }

  # Analytics Types
  type DashboardMetrics {
    totalUsers: Int!
    totalBeneficiaries: Int!
    activePrograms: Int!
    totalDisbursements: Float!
    pendingApprovals: Int!
  }

  type FraudAlert {
    id: ID!
    beneficiaryId: ID!
    transactionId: ID
    alertType: String!
    severity: String!
    description: String!
    status: String!
    createdAt: String!
  }

  # Middleware Monitoring Types
  type MiddlewareHealth {
    component: String!
    status: String!
    lastCheck: String!
    responseTime: Float
    errorRate: Float
  }

  type MiddlewareMetric {
    component: String!
    metricType: String!
    value: Float!
    timestamp: String!
  }

  # Tenant Types
  type Tenant {
    id: ID!
    tenantCode: String!
    tenantName: String!
    status: TenantStatus!
    createdAt: String!
  }

  enum TenantStatus {
    active
    suspended
    inactive
  }

  # Query Root
  type Query {
    # User queries
    me: User
    getUser(id: ID!): User
    getAllUsers: [User!]!

    # Beneficiary queries
    getBeneficiary(id: ID!): Beneficiary
    getAllBeneficiaries(limit: Int, offset: Int): [Beneficiary!]!
    searchBeneficiaries(query: String!): [Beneficiary!]!

    # Program queries
    getProgram(id: ID!): Program
    getAllPrograms: [Program!]!

    # Transaction queries
    getTransaction(id: ID!): Transaction
    getTransactionsByBeneficiary(beneficiaryId: ID!): [Transaction!]!
    getAllTransactions(limit: Int, offset: Int): [Transaction!]!

    # Analytics queries
    getDashboardMetrics: DashboardMetrics!
    getFraudAlerts(limit: Int): [FraudAlert!]!

    # Middleware queries
    getMiddlewareHealth: [MiddlewareHealth!]!
    getMiddlewareMetrics(component: String!, hours: Int): [MiddlewareMetric!]!

    # Tenant queries
    getTenant(id: ID!): Tenant
    getAllTenants: [Tenant!]!
    getMyTenants: [Tenant!]!
  }

  # Mutation Root
  type Mutation {
    # Beneficiary mutations
    createBeneficiary(input: CreateBeneficiaryInput!): Beneficiary!
    updateBeneficiary(id: ID!, input: UpdateBeneficiaryInput!): Beneficiary!
    deleteBeneficiary(id: ID!): Boolean!

    # Program mutations
    createProgram(input: CreateProgramInput!): Program!
    updateProgram(id: ID!, input: UpdateProgramInput!): Program!
    deleteProgram(id: ID!): Boolean!

    # Transaction mutations
    createDisbursement(beneficiaryId: ID!, amount: Float!, programId: ID): Transaction!

    # Tenant mutations
    createTenant(tenantCode: String!, tenantName: String!): Tenant!
    updateTenantStatus(id: ID!, status: TenantStatus!): Tenant!
  }

  # Subscription Root
  type Subscription {
    # Real-time dashboard updates
    dashboardUpdated: DashboardMetrics!

    # Transaction updates
    transactionCreated: Transaction!
    transactionUpdated(beneficiaryId: ID): Transaction!

    # Fraud alerts
    fraudAlertCreated: FraudAlert!

    # Middleware health updates
    middlewareHealthUpdated: MiddlewareHealth!
  }

  # Input Types
  input CreateBeneficiaryInput {
    firstName: String!
    lastName: String!
    dateOfBirth: String
    gender: String
    phoneNumber: String
    email: String
    nationalId: String
    address: String
    city: String
    state: String
    postalCode: String
    country: String
  }

  input UpdateBeneficiaryInput {
    firstName: String
    lastName: String
    phoneNumber: String
    email: String
    address: String
    city: String
    state: String
    postalCode: String
    status: BeneficiaryStatus
  }

  input CreateProgramInput {
    name: String!
    description: String
    budget: Float
    startDate: String
    endDate: String
  }

  input UpdateProgramInput {
    name: String
    description: String
    budget: Float
    startDate: String
    endDate: String
    status: ProgramStatus
  }
`;
