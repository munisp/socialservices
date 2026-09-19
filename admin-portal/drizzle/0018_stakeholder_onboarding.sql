CREATE TABLE IF NOT EXISTS stakeholder_organizations (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  legal_name VARCHAR(255) NOT NULL,
  stakeholder_type VARCHAR(50) NOT NULL,
  registration_reference VARCHAR(255) NOT NULL UNIQUE,
  verification_status ENUM('pending','verified','rejected','suspended') NOT NULL DEFAULT 'pending',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS stakeholder_onboarding (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NULL,
  organization_id BIGINT NULL,
  stakeholder_type VARCHAR(50) NOT NULL,
  display_name VARCHAR(255) NOT NULL,
  state ENUM('invited','identity_pending','organization_pending','consent_pending','scope_pending','training_pending','mfa_pending','active','suspended','offboarded','rejected') NOT NULL DEFAULT 'invited',
  region_codes JSON NOT NULL,
  program_ids JSON NOT NULL,
  consent_version VARCHAR(100) NOT NULL,
  identity_evidence_refs JSON NOT NULL,
  requires_mfa BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_stakeholder_state (state),
  INDEX idx_stakeholder_type (stakeholder_type),
  CONSTRAINT fk_stakeholder_org FOREIGN KEY (organization_id) REFERENCES stakeholder_organizations(id)
);
CREATE TABLE IF NOT EXISTS stakeholder_onboarding_events (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  onboarding_id BIGINT NOT NULL,
  from_state VARCHAR(50),
  to_state VARCHAR(50) NOT NULL,
  actor_user_id BIGINT NULL,
  reason TEXT NOT NULL,
  evidence JSON,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_onboarding_events (onboarding_id, created_at),
  CONSTRAINT fk_onboarding_event FOREIGN KEY (onboarding_id) REFERENCES stakeholder_onboarding(id)
);
