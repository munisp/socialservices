-- Performance Indexes Migration
-- Adds indexes for common query patterns to improve performance

-- Beneficiaries table indexes
CREATE INDEX IF NOT EXISTS idx_beneficiaries_national_id ON beneficiaries(national_id);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_enrollment_status ON beneficiaries(enrollment_status);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_kyc_status ON beneficiaries(kyc_status);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_program_id ON beneficiaries(program_id);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_household_id ON beneficiaries(household_id);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_created_at ON beneficiaries(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_updated_at ON beneficiaries(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_deleted_at ON beneficiaries(deleted_at) WHERE deleted_at IS NOT NULL;

-- Composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_beneficiaries_status_program ON beneficiaries(enrollment_status, program_id);
CREATE INDEX IF NOT EXISTS idx_beneficiaries_search ON beneficiaries(first_name, last_name, national_id);

-- Households table indexes
CREATE INDEX IF NOT EXISTS idx_households_head_id ON households(head_id);
CREATE INDEX IF NOT EXISTS idx_households_location ON households(state, city);
CREATE INDEX IF NOT EXISTS idx_households_created_at ON households(created_at DESC);

-- Programs table indexes
CREATE INDEX IF NOT EXISTS idx_programs_status ON programs(status);
CREATE INDEX IF NOT EXISTS idx_programs_start_date ON programs(start_date);
CREATE INDEX IF NOT EXISTS idx_programs_end_date ON programs(end_date);

-- Disbursements table indexes
CREATE INDEX IF NOT EXISTS idx_disbursements_beneficiary_id ON disbursements(beneficiary_id);
CREATE INDEX IF NOT EXISTS idx_disbursements_program_id ON disbursements(program_id);
CREATE INDEX IF NOT EXISTS idx_disbursements_status ON disbursements(status);
CREATE INDEX IF NOT EXISTS idx_disbursements_created_at ON disbursements(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_disbursements_idempotency_key ON disbursements(idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Composite index for disbursement queries
CREATE INDEX IF NOT EXISTS idx_disbursements_beneficiary_status ON disbursements(beneficiary_id, status);
CREATE INDEX IF NOT EXISTS idx_disbursements_program_status ON disbursements(program_id, status);

-- Audit logs table indexes
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type ON audit_logs(resource_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_id ON audit_logs(resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_correlation_id ON audit_logs(correlation_id);

-- Composite index for audit queries
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_action ON audit_logs(user_id, action);

-- Workflows table indexes
CREATE INDEX IF NOT EXISTS idx_workflows_status ON workflows(status);
CREATE INDEX IF NOT EXISTS idx_workflows_type ON workflows(workflow_type);
CREATE INDEX IF NOT EXISTS idx_workflows_beneficiary_id ON workflows(beneficiary_id);
CREATE INDEX IF NOT EXISTS idx_workflows_created_at ON workflows(created_at DESC);

-- Approvals table indexes
CREATE INDEX IF NOT EXISTS idx_approvals_workflow_id ON approvals(workflow_id);
CREATE INDEX IF NOT EXISTS idx_approvals_approver_id ON approvals(approver_id);
CREATE INDEX IF NOT EXISTS idx_approvals_status ON approvals(status);
CREATE INDEX IF NOT EXISTS idx_approvals_created_at ON approvals(created_at DESC);

-- Grievances table indexes
CREATE INDEX IF NOT EXISTS idx_grievances_beneficiary_id ON grievances(beneficiary_id);
CREATE INDEX IF NOT EXISTS idx_grievances_status ON grievances(status);
CREATE INDEX IF NOT EXISTS idx_grievances_priority ON grievances(priority);
CREATE INDEX IF NOT EXISTS idx_grievances_created_at ON grievances(created_at DESC);

-- PMT surveys table indexes
CREATE INDEX IF NOT EXISTS idx_pmt_surveys_household_id ON pmt_surveys(household_id);
CREATE INDEX IF NOT EXISTS idx_pmt_surveys_score ON pmt_surveys(score);
CREATE INDEX IF NOT EXISTS idx_pmt_surveys_eligibility ON pmt_surveys(eligibility_category);
CREATE INDEX IF NOT EXISTS idx_pmt_surveys_created_at ON pmt_surveys(created_at DESC);

-- Identity verifications table indexes
CREATE INDEX IF NOT EXISTS idx_identity_verifications_beneficiary_id ON identity_verifications(beneficiary_id);
CREATE INDEX IF NOT EXISTS idx_identity_verifications_provider ON identity_verifications(provider_id);
CREATE INDEX IF NOT EXISTS idx_identity_verifications_status ON identity_verifications(status);
CREATE INDEX IF NOT EXISTS idx_identity_verifications_created_at ON identity_verifications(created_at DESC);

-- Consents table indexes
CREATE INDEX IF NOT EXISTS idx_consents_beneficiary_id ON consents(beneficiary_id);
CREATE INDEX IF NOT EXISTS idx_consents_sector ON consents(sector);
CREATE INDEX IF NOT EXISTS idx_consents_status ON consents(status);
CREATE INDEX IF NOT EXISTS idx_consents_expires_at ON consents(expires_at);

-- Sync conflicts table indexes
CREATE INDEX IF NOT EXISTS idx_sync_conflicts_device_id ON sync_conflicts(device_id);
CREATE INDEX IF NOT EXISTS idx_sync_conflicts_entity_type ON sync_conflicts(entity_type);
CREATE INDEX IF NOT EXISTS idx_sync_conflicts_resolved ON sync_conflicts(resolved);
CREATE INDEX IF NOT EXISTS idx_sync_conflicts_created_at ON sync_conflicts(created_at DESC);

-- Full-text search indexes (PostgreSQL specific)
CREATE INDEX IF NOT EXISTS idx_beneficiaries_fts ON beneficiaries 
  USING gin(to_tsvector('english', coalesce(first_name, '') || ' ' || coalesce(last_name, '') || ' ' || coalesce(national_id, '')));

-- GiST index for geospatial queries (if using PostGIS)
-- CREATE INDEX IF NOT EXISTS idx_households_location_geo ON households USING gist(location);

-- Partial indexes for common filtered queries
CREATE INDEX IF NOT EXISTS idx_beneficiaries_active ON beneficiaries(id) 
  WHERE enrollment_status = 'approved' AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_disbursements_pending ON disbursements(id, beneficiary_id, amount) 
  WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_workflows_active ON workflows(id, beneficiary_id) 
  WHERE status IN ('pending', 'in_progress');

-- Analyze tables after creating indexes
ANALYZE beneficiaries;
ANALYZE households;
ANALYZE programs;
ANALYZE disbursements;
ANALYZE audit_logs;
ANALYZE workflows;
ANALYZE approvals;
ANALYZE grievances;
