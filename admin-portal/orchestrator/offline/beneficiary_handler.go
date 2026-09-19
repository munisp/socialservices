package offline

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// BeneficiaryData represents beneficiary data for offline sync
type BeneficiaryData struct {
	ID                 string          `json:"id"`
	FirstName          string          `json:"firstName"`
	LastName           string          `json:"lastName"`
	DateOfBirth        string          `json:"dateOfBirth"`
	Gender             string          `json:"gender"`
	NationalID         string          `json:"nationalId"`
	PhoneNumber        string          `json:"phoneNumber,omitempty"`
	Email              string          `json:"email,omitempty"`
	Address            json.RawMessage `json:"address,omitempty"`
	Location           json.RawMessage `json:"location,omitempty"`
	HouseholdID        string          `json:"householdId,omitempty"`
	Status             string          `json:"status"`
	EnrollmentDate     string          `json:"enrollmentDate,omitempty"`
	BiometricCaptured  bool            `json:"biometricCaptured"`
	DocumentsVerified  bool            `json:"documentsVerified"`
	EligibilityScore   float64         `json:"eligibilityScore,omitempty"`
	ProxyMeansScore    float64         `json:"proxyMeansScore,omitempty"`
	Version            int64           `json:"version"`
	UpdatedAt          time.Time       `json:"updatedAt"`
}

// BeneficiarySyncHandler handles offline sync for beneficiaries
type BeneficiarySyncHandler struct {
	db *sql.DB
}

// NewBeneficiarySyncHandler creates a new BeneficiarySyncHandler
func NewBeneficiarySyncHandler(db *sql.DB) *BeneficiarySyncHandler {
	return &BeneficiarySyncHandler{db: db}
}

// GetServerVersion returns the current server version for a beneficiary
func (h *BeneficiarySyncHandler) GetServerVersion(ctx context.Context, entityID string) (int64, error) {
	var version int64
	err := h.db.QueryRowContext(ctx, `
		SELECT version FROM beneficiaries WHERE id = ?
	`, entityID).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

// GetServerData returns the current server data for a beneficiary
func (h *BeneficiarySyncHandler) GetServerData(ctx context.Context, entityID string) (json.RawMessage, error) {
	var data BeneficiaryData
	var address, location sql.NullString
	
	err := h.db.QueryRowContext(ctx, `
		SELECT id, first_name, last_name, date_of_birth, gender, national_id,
			   phone_number, email, address, location, household_id, status,
			   enrollment_date, biometric_captured, documents_verified,
			   eligibility_score, proxy_means_score, version, updated_at
		FROM beneficiaries WHERE id = ?
	`, entityID).Scan(
		&data.ID, &data.FirstName, &data.LastName, &data.DateOfBirth, &data.Gender,
		&data.NationalID, &data.PhoneNumber, &data.Email, &address, &location,
		&data.HouseholdID, &data.Status, &data.EnrollmentDate, &data.BiometricCaptured,
		&data.DocumentsVerified, &data.EligibilityScore, &data.ProxyMeansScore,
		&data.Version, &data.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if address.Valid {
		data.Address = json.RawMessage(address.String)
	}
	if location.Valid {
		data.Location = json.RawMessage(location.String)
	}

	return json.Marshal(data)
}

// ApplyCreate creates a new beneficiary from offline data
func (h *BeneficiarySyncHandler) ApplyCreate(ctx context.Context, entityID string, data json.RawMessage, userID int64) error {
	var beneficiary BeneficiaryData
	if err := json.Unmarshal(data, &beneficiary); err != nil {
		return fmt.Errorf("invalid beneficiary data: %w", err)
	}

	_, err := h.db.ExecContext(ctx, `
		INSERT INTO beneficiaries (
			id, first_name, last_name, date_of_birth, gender, national_id,
			phone_number, email, address, location, household_id, status,
			enrollment_date, biometric_captured, documents_verified,
			eligibility_score, proxy_means_score, version, created_by, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, NOW(), NOW())
	`,
		entityID, beneficiary.FirstName, beneficiary.LastName, beneficiary.DateOfBirth,
		beneficiary.Gender, beneficiary.NationalID, beneficiary.PhoneNumber, beneficiary.Email,
		string(beneficiary.Address), string(beneficiary.Location), beneficiary.HouseholdID,
		beneficiary.Status, beneficiary.EnrollmentDate, beneficiary.BiometricCaptured,
		beneficiary.DocumentsVerified, beneficiary.EligibilityScore, beneficiary.ProxyMeansScore,
		userID,
	)
	return err
}

// ApplyUpdate updates an existing beneficiary from offline data
func (h *BeneficiarySyncHandler) ApplyUpdate(ctx context.Context, entityID string, data json.RawMessage, version int64, userID int64) error {
	var beneficiary BeneficiaryData
	if err := json.Unmarshal(data, &beneficiary); err != nil {
		return fmt.Errorf("invalid beneficiary data: %w", err)
	}

	result, err := h.db.ExecContext(ctx, `
		UPDATE beneficiaries SET
			first_name = ?, last_name = ?, date_of_birth = ?, gender = ?,
			national_id = ?, phone_number = ?, email = ?, address = ?,
			location = ?, household_id = ?, status = ?, enrollment_date = ?,
			biometric_captured = ?, documents_verified = ?, eligibility_score = ?,
			proxy_means_score = ?, version = version + 1, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND (version = ? OR ? = 0)
	`,
		beneficiary.FirstName, beneficiary.LastName, beneficiary.DateOfBirth, beneficiary.Gender,
		beneficiary.NationalID, beneficiary.PhoneNumber, beneficiary.Email, string(beneficiary.Address),
		string(beneficiary.Location), beneficiary.HouseholdID, beneficiary.Status, beneficiary.EnrollmentDate,
		beneficiary.BiometricCaptured, beneficiary.DocumentsVerified, beneficiary.EligibilityScore,
		beneficiary.ProxyMeansScore, userID, entityID, version, version,
	)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("optimistic lock failed: beneficiary %s has been modified", entityID)
	}
	return nil
}

// ApplyDelete soft-deletes a beneficiary
func (h *BeneficiarySyncHandler) ApplyDelete(ctx context.Context, entityID string, userID int64) error {
	_, err := h.db.ExecContext(ctx, `
		UPDATE beneficiaries SET 
			status = 'deleted', 
			deleted_at = NOW(), 
			deleted_by = ?,
			version = version + 1
		WHERE id = ?
	`, userID, entityID)
	return err
}

// MergeData merges client and server data for a beneficiary
func (h *BeneficiarySyncHandler) MergeData(ctx context.Context, clientData, serverData json.RawMessage) (json.RawMessage, error) {
	var client, server BeneficiaryData
	if err := json.Unmarshal(clientData, &client); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(serverData, &server); err != nil {
		return nil, err
	}

	// Merge strategy: prefer client for user-editable fields, server for system fields
	merged := server

	// User-editable fields - prefer client if changed
	if client.FirstName != "" {
		merged.FirstName = client.FirstName
	}
	if client.LastName != "" {
		merged.LastName = client.LastName
	}
	if client.PhoneNumber != "" {
		merged.PhoneNumber = client.PhoneNumber
	}
	if client.Email != "" {
		merged.Email = client.Email
	}
	if len(client.Address) > 0 {
		merged.Address = client.Address
	}
	if len(client.Location) > 0 {
		merged.Location = client.Location
	}

	// Biometric and document status - prefer true (once captured, always captured)
	if client.BiometricCaptured {
		merged.BiometricCaptured = true
	}
	if client.DocumentsVerified {
		merged.DocumentsVerified = true
	}

	// System fields - always use server values
	// (eligibility score, proxy means score, status changes require workflow)

	return json.Marshal(merged)
}

// GetChangesSince returns beneficiaries changed since a timestamp
func (h *BeneficiarySyncHandler) GetChangesSince(ctx context.Context, since time.Time, limit int) ([]OfflineRecord, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT id, first_name, last_name, date_of_birth, gender, national_id,
			   phone_number, email, address, location, household_id, status,
			   enrollment_date, biometric_captured, documents_verified,
			   eligibility_score, proxy_means_score, version, updated_at
		FROM beneficiaries 
		WHERE updated_at > ?
		ORDER BY updated_at ASC
		LIMIT ?
	`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []OfflineRecord
	for rows.Next() {
		var data BeneficiaryData
		var address, location sql.NullString

		err := rows.Scan(
			&data.ID, &data.FirstName, &data.LastName, &data.DateOfBirth, &data.Gender,
			&data.NationalID, &data.PhoneNumber, &data.Email, &address, &location,
			&data.HouseholdID, &data.Status, &data.EnrollmentDate, &data.BiometricCaptured,
			&data.DocumentsVerified, &data.EligibilityScore, &data.ProxyMeansScore,
			&data.Version, &data.UpdatedAt,
		)
		if err != nil {
			continue
		}

		if address.Valid {
			data.Address = json.RawMessage(address.String)
		}
		if location.Valid {
			data.Location = json.RawMessage(location.String)
		}

		jsonData, _ := json.Marshal(data)
		checksum := GenerateChecksum(jsonData)

		operation := SyncOpUpdate
		if data.Status == "deleted" {
			operation = SyncOpDelete
		}

		records = append(records, OfflineRecord{
			ID:            fmt.Sprintf("server-%s-%d", data.ID, data.Version),
			EntityType:    "beneficiary",
			EntityID:      data.ID,
			Operation:     operation,
			Data:          jsonData,
			Checksum:      checksum,
			ServerVersion: data.Version,
			Status:        SyncStatusSynced,
			CreatedAt:     data.UpdatedAt,
		})
	}

	return records, nil
}

// HouseholdSyncHandler handles offline sync for households
type HouseholdSyncHandler struct {
	db *sql.DB
}

// HouseholdData represents household data for offline sync
type HouseholdData struct {
	ID              string          `json:"id"`
	HeadID          string          `json:"headId"`
	Name            string          `json:"name"`
	Address         json.RawMessage `json:"address,omitempty"`
	Location        json.RawMessage `json:"location,omitempty"`
	MemberCount     int             `json:"memberCount"`
	IncomeLevel     string          `json:"incomeLevel,omitempty"`
	HousingType     string          `json:"housingType,omitempty"`
	Assets          json.RawMessage `json:"assets,omitempty"`
	ProxyMeansScore float64         `json:"proxyMeansScore,omitempty"`
	Status          string          `json:"status"`
	Version         int64           `json:"version"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// NewHouseholdSyncHandler creates a new HouseholdSyncHandler
func NewHouseholdSyncHandler(db *sql.DB) *HouseholdSyncHandler {
	return &HouseholdSyncHandler{db: db}
}

// GetServerVersion returns the current server version for a household
func (h *HouseholdSyncHandler) GetServerVersion(ctx context.Context, entityID string) (int64, error) {
	var version int64
	err := h.db.QueryRowContext(ctx, `
		SELECT version FROM households WHERE id = ?
	`, entityID).Scan(&version)
	return version, err
}

// GetServerData returns the current server data for a household
func (h *HouseholdSyncHandler) GetServerData(ctx context.Context, entityID string) (json.RawMessage, error) {
	var data HouseholdData
	var address, location, assets sql.NullString

	err := h.db.QueryRowContext(ctx, `
		SELECT id, head_id, name, address, location, member_count,
			   income_level, housing_type, assets, proxy_means_score,
			   status, version, updated_at
		FROM households WHERE id = ?
	`, entityID).Scan(
		&data.ID, &data.HeadID, &data.Name, &address, &location,
		&data.MemberCount, &data.IncomeLevel, &data.HousingType, &assets,
		&data.ProxyMeansScore, &data.Status, &data.Version, &data.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if address.Valid {
		data.Address = json.RawMessage(address.String)
	}
	if location.Valid {
		data.Location = json.RawMessage(location.String)
	}
	if assets.Valid {
		data.Assets = json.RawMessage(assets.String)
	}

	return json.Marshal(data)
}

// ApplyCreate creates a new household from offline data
func (h *HouseholdSyncHandler) ApplyCreate(ctx context.Context, entityID string, data json.RawMessage, userID int64) error {
	var household HouseholdData
	if err := json.Unmarshal(data, &household); err != nil {
		return fmt.Errorf("invalid household data: %w", err)
	}

	_, err := h.db.ExecContext(ctx, `
		INSERT INTO households (
			id, head_id, name, address, location, member_count,
			income_level, housing_type, assets, proxy_means_score,
			status, version, created_by, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, NOW(), NOW())
	`,
		entityID, household.HeadID, household.Name, string(household.Address),
		string(household.Location), household.MemberCount, household.IncomeLevel,
		household.HousingType, string(household.Assets), household.ProxyMeansScore,
		household.Status, userID,
	)
	return err
}

// ApplyUpdate updates an existing household from offline data
func (h *HouseholdSyncHandler) ApplyUpdate(ctx context.Context, entityID string, data json.RawMessage, version int64, userID int64) error {
	var household HouseholdData
	if err := json.Unmarshal(data, &household); err != nil {
		return fmt.Errorf("invalid household data: %w", err)
	}

	result, err := h.db.ExecContext(ctx, `
		UPDATE households SET
			head_id = ?, name = ?, address = ?, location = ?,
			member_count = ?, income_level = ?, housing_type = ?,
			assets = ?, proxy_means_score = ?, status = ?,
			version = version + 1, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND (version = ? OR ? = 0)
	`,
		household.HeadID, household.Name, string(household.Address),
		string(household.Location), household.MemberCount, household.IncomeLevel,
		household.HousingType, string(household.Assets), household.ProxyMeansScore,
		household.Status, userID, entityID, version, version,
	)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("optimistic lock failed: household %s has been modified", entityID)
	}
	return nil
}

// ApplyDelete soft-deletes a household
func (h *HouseholdSyncHandler) ApplyDelete(ctx context.Context, entityID string, userID int64) error {
	_, err := h.db.ExecContext(ctx, `
		UPDATE households SET 
			status = 'deleted', 
			deleted_at = NOW(), 
			deleted_by = ?,
			version = version + 1
		WHERE id = ?
	`, userID, entityID)
	return err
}

// MergeData merges client and server data for a household
func (h *HouseholdSyncHandler) MergeData(ctx context.Context, clientData, serverData json.RawMessage) (json.RawMessage, error) {
	var client, server HouseholdData
	if err := json.Unmarshal(clientData, &client); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(serverData, &server); err != nil {
		return nil, err
	}

	// Merge strategy
	merged := server

	if client.Name != "" {
		merged.Name = client.Name
	}
	if len(client.Address) > 0 {
		merged.Address = client.Address
	}
	if len(client.Location) > 0 {
		merged.Location = client.Location
	}
	if client.MemberCount > 0 {
		merged.MemberCount = client.MemberCount
	}
	if client.IncomeLevel != "" {
		merged.IncomeLevel = client.IncomeLevel
	}
	if client.HousingType != "" {
		merged.HousingType = client.HousingType
	}
	if len(client.Assets) > 0 {
		merged.Assets = client.Assets
	}

	return json.Marshal(merged)
}

// GetChangesSince returns households changed since a timestamp
func (h *HouseholdSyncHandler) GetChangesSince(ctx context.Context, since time.Time, limit int) ([]OfflineRecord, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT id, head_id, name, address, location, member_count,
			   income_level, housing_type, assets, proxy_means_score,
			   status, version, updated_at
		FROM households 
		WHERE updated_at > ?
		ORDER BY updated_at ASC
		LIMIT ?
	`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []OfflineRecord
	for rows.Next() {
		var data HouseholdData
		var address, location, assets sql.NullString

		err := rows.Scan(
			&data.ID, &data.HeadID, &data.Name, &address, &location,
			&data.MemberCount, &data.IncomeLevel, &data.HousingType, &assets,
			&data.ProxyMeansScore, &data.Status, &data.Version, &data.UpdatedAt,
		)
		if err != nil {
			continue
		}

		if address.Valid {
			data.Address = json.RawMessage(address.String)
		}
		if location.Valid {
			data.Location = json.RawMessage(location.String)
		}
		if assets.Valid {
			data.Assets = json.RawMessage(assets.String)
		}

		jsonData, _ := json.Marshal(data)
		checksum := GenerateChecksum(jsonData)

		operation := SyncOpUpdate
		if data.Status == "deleted" {
			operation = SyncOpDelete
		}

		records = append(records, OfflineRecord{
			ID:            fmt.Sprintf("server-%s-%d", data.ID, data.Version),
			EntityType:    "household",
			EntityID:      data.ID,
			Operation:     operation,
			Data:          jsonData,
			Checksum:      checksum,
			ServerVersion: data.Version,
			Status:        SyncStatusSynced,
			CreatedAt:     data.UpdatedAt,
		})
	}

	return records, nil
}
