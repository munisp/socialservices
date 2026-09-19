-- Payment Integration Schema for TigerBeetle and Mojaloop
-- This migration adds tables for payment intent tracking, account mappings, and callback correlation

-- TigerBeetle Account Mappings
-- Maps external IDs (program, beneficiary) to TigerBeetle account IDs
CREATE TABLE IF NOT EXISTS tigerbeetle_account_mappings (
    id SERIAL PRIMARY KEY,
    external_id VARCHAR(255) NOT NULL,
    external_type VARCHAR(50) NOT NULL, -- 'program', 'beneficiary', 'fee', 'treasury', 'escrow'
    tigerbeetle_account_id VARCHAR(64) NOT NULL, -- Uint128 as hex string
    ledger INTEGER NOT NULL DEFAULT 1,
    code INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(external_id, external_type)
);

CREATE INDEX idx_tb_account_external ON tigerbeetle_account_mappings(external_id, external_type);
CREATE INDEX idx_tb_account_id ON tigerbeetle_account_mappings(tigerbeetle_account_id);

-- Payment Intent Table
-- Tracks payment intents for idempotency and audit
CREATE TABLE IF NOT EXISTS payment_intents (
    id SERIAL PRIMARY KEY,
    intent_id VARCHAR(64) NOT NULL UNIQUE, -- Idempotency key
    disbursement_id VARCHAR(255),
    beneficiary_id VARCHAR(255) NOT NULL,
    program_id VARCHAR(255) NOT NULL,
    amount_cents BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'NGN',
    fee_cents BIGINT DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, reserved, committed, voided, failed
    tigerbeetle_pending_id VARCHAR(64), -- Pending transfer ID
    tigerbeetle_commit_id VARCHAR(64), -- Commit transfer ID
    mojaloop_quote_id VARCHAR(64),
    mojaloop_transfer_id VARCHAR(64),
    mojaloop_transfer_state VARCHAR(50),
    settlement_cycle_id VARCHAR(64),
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    reserved_at TIMESTAMP WITH TIME ZONE,
    committed_at TIMESTAMP WITH TIME ZONE,
    voided_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_payment_intent_id ON payment_intents(intent_id);
CREATE INDEX idx_payment_intent_beneficiary ON payment_intents(beneficiary_id);
CREATE INDEX idx_payment_intent_program ON payment_intents(program_id);
CREATE INDEX idx_payment_intent_status ON payment_intents(status);
CREATE INDEX idx_payment_intent_mojaloop ON payment_intents(mojaloop_transfer_id);
CREATE INDEX idx_payment_intent_tigerbeetle ON payment_intents(tigerbeetle_pending_id);

-- Mojaloop Callback Correlation Table
-- Stores pending callbacks for correlation across replicas
CREATE TABLE IF NOT EXISTS mojaloop_callbacks (
    id SERIAL PRIMARY KEY,
    callback_type VARCHAR(50) NOT NULL, -- 'party', 'quote', 'transfer', 'bulk_quote', 'bulk_transfer'
    correlation_id VARCHAR(64) NOT NULL, -- partyId, quoteId, transferId, etc.
    party_id_type VARCHAR(50),
    party_identifier VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, received, processed, expired
    request_payload JSONB,
    response_payload JSONB,
    error_payload JSONB,
    workflow_run_id VARCHAR(255), -- Temporal workflow run ID for correlation
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    received_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(callback_type, correlation_id)
);

CREATE INDEX idx_mojaloop_callback_correlation ON mojaloop_callbacks(callback_type, correlation_id);
CREATE INDEX idx_mojaloop_callback_status ON mojaloop_callbacks(status);
CREATE INDEX idx_mojaloop_callback_expires ON mojaloop_callbacks(expires_at);
CREATE INDEX idx_mojaloop_callback_workflow ON mojaloop_callbacks(workflow_run_id);

-- Payment Events Table
-- Stores payment lifecycle events for Kafka publishing and audit
CREATE TABLE IF NOT EXISTS payment_events (
    id SERIAL PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL UNIQUE,
    event_type VARCHAR(100) NOT NULL, -- 'payment.reserved', 'payment.committed', 'payment.voided', etc.
    payment_intent_id VARCHAR(64) NOT NULL,
    correlation_id VARCHAR(64), -- For tracing
    payload JSONB NOT NULL,
    published_to_kafka BOOLEAN DEFAULT FALSE,
    published_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_payment_event_type ON payment_events(event_type);
CREATE INDEX idx_payment_event_intent ON payment_events(payment_intent_id);
CREATE INDEX idx_payment_event_published ON payment_events(published_to_kafka);
CREATE INDEX idx_payment_event_correlation ON payment_events(correlation_id);

-- Settlement Cycles Table
-- Tracks settlement cycles for Treasury/CBN reporting
CREATE TABLE IF NOT EXISTS settlement_cycles (
    id SERIAL PRIMARY KEY,
    cycle_id VARCHAR(64) NOT NULL UNIQUE,
    currency VARCHAR(3) NOT NULL DEFAULT 'NGN',
    status VARCHAR(50) NOT NULL DEFAULT 'open', -- open, closed, pending_settlement, settled, aborted
    start_time TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    end_time TIMESTAMP WITH TIME ZONE,
    total_transfers INTEGER DEFAULT 0,
    total_amount_cents BIGINT DEFAULT 0,
    net_positions JSONB, -- { "dfsp_id": { "debit": 0, "credit": 0, "net": 0 } }
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_settlement_cycle_id ON settlement_cycles(cycle_id);
CREATE INDEX idx_settlement_cycle_status ON settlement_cycles(status);

-- Settlement Transfers Table
-- Links transfers to settlement cycles
CREATE TABLE IF NOT EXISTS settlement_transfers (
    id SERIAL PRIMARY KEY,
    settlement_cycle_id VARCHAR(64) NOT NULL,
    transfer_id VARCHAR(64) NOT NULL,
    payer_fsp VARCHAR(100) NOT NULL,
    payee_fsp VARCHAR(100) NOT NULL,
    amount_cents BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'NGN',
    transfer_state VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(settlement_cycle_id, transfer_id)
);

CREATE INDEX idx_settlement_transfer_cycle ON settlement_transfers(settlement_cycle_id);
CREATE INDEX idx_settlement_transfer_id ON settlement_transfers(transfer_id);

-- Activity Idempotency Table
-- Stores idempotency keys for Temporal activities
CREATE TABLE IF NOT EXISTS activity_idempotency (
    id SERIAL PRIMARY KEY,
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,
    activity_type VARCHAR(100) NOT NULL,
    workflow_id VARCHAR(255),
    run_id VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'processing', -- processing, completed, failed
    result JSONB,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_activity_idempotency_key ON activity_idempotency(idempotency_key);
CREATE INDEX idx_activity_idempotency_workflow ON activity_idempotency(workflow_id, run_id);
CREATE INDEX idx_activity_idempotency_expires ON activity_idempotency(expires_at);
