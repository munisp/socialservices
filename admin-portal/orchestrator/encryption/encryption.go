// Package encryption provides data encryption at rest using AES-256-GCM
// with Vault Transit for key management
package encryption

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext format")
	ErrDecryptionFailed  = errors.New("decryption failed")
	ErrKeyNotFound       = errors.New("encryption key not found")
	ErrVaultUnavailable  = errors.New("vault service unavailable")
)

// EncryptedData represents encrypted data with metadata
type EncryptedData struct {
	Ciphertext string `json:"ciphertext"`
	Nonce      string `json:"nonce"`
	KeyID      string `json:"key_id"`
	KeyVersion int    `json:"key_version"`
	Algorithm  string `json:"algorithm"`
}

// VaultTransitClient handles encryption via HashiCorp Vault Transit
type VaultTransitClient struct {
	vaultAddr   string
	vaultToken  string
	transitPath string
	httpClient  *http.Client
	keyCache    map[string][]byte
	cacheMu     sync.RWMutex
}

// FieldEncryptor handles application-level field encryption
type FieldEncryptor struct {
	vaultClient *VaultTransitClient
	localKey    []byte // Fallback for local development
	keyID       string
	keyVersion  int
}

// NewVaultTransitClient creates a new Vault Transit client
func NewVaultTransitClient() (*VaultTransitClient, error) {
	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		vaultAddr = "http://vault:8200"
	}

	vaultToken := os.Getenv("VAULT_TOKEN")
	if vaultToken == "" {
		// Try to read from file (Kubernetes service account)
		tokenBytes, err := os.ReadFile("/var/run/secrets/vault/token")
		if err == nil {
			vaultToken = strings.TrimSpace(string(tokenBytes))
		}
	}

	transitPath := os.Getenv("VAULT_TRANSIT_PATH")
	if transitPath == "" {
		transitPath = "transit"
	}

	return &VaultTransitClient{
		vaultAddr:   vaultAddr,
		vaultToken:  vaultToken,
		transitPath: transitPath,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		keyCache: make(map[string][]byte),
	}, nil
}

// Encrypt encrypts plaintext using Vault Transit
func (v *VaultTransitClient) Encrypt(ctx context.Context, keyName string, plaintext []byte) (*EncryptedData, error) {
	if v.vaultToken == "" {
		return nil, ErrVaultUnavailable
	}

	url := fmt.Sprintf("%s/v1/%s/encrypt/%s", v.vaultAddr, v.transitPath, keyName)
	
	payload := map[string]interface{}{
		"plaintext": base64.StdEncoding.EncodeToString(plaintext),
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", v.vaultToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault returned status %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Ciphertext string `json:"ciphertext"`
			KeyVersion int    `json:"key_version"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &EncryptedData{
		Ciphertext: result.Data.Ciphertext,
		KeyID:      keyName,
		KeyVersion: result.Data.KeyVersion,
		Algorithm:  "vault-transit-aes256-gcm96",
	}, nil
}

// Decrypt decrypts ciphertext using Vault Transit
func (v *VaultTransitClient) Decrypt(ctx context.Context, keyName string, encrypted *EncryptedData) ([]byte, error) {
	if v.vaultToken == "" {
		return nil, ErrVaultUnavailable
	}

	url := fmt.Sprintf("%s/v1/%s/decrypt/%s", v.vaultAddr, v.transitPath, keyName)
	
	payload := map[string]interface{}{
		"ciphertext": encrypted.Ciphertext,
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", v.vaultToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault returned status %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Plaintext string `json:"plaintext"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return base64.StdEncoding.DecodeString(result.Data.Plaintext)
}

// RotateKey triggers key rotation in Vault
func (v *VaultTransitClient) RotateKey(ctx context.Context, keyName string) error {
	if v.vaultToken == "" {
		return ErrVaultUnavailable
	}

	url := fmt.Sprintf("%s/v1/%s/keys/%s/rotate", v.vaultAddr, v.transitPath, keyName)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", v.vaultToken)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("vault returned status %d", resp.StatusCode)
	}

	return nil
}

// NewFieldEncryptor creates a new field encryptor
func NewFieldEncryptor() (*FieldEncryptor, error) {
	vaultClient, err := NewVaultTransitClient()
	if err != nil {
		return nil, err
	}

	encryptor := &FieldEncryptor{
		vaultClient: vaultClient,
		keyID:       "social-protection-pii",
		keyVersion:  1,
	}

	// Generate local fallback key for development
	localKeyEnv := os.Getenv("ENCRYPTION_KEY")
	if localKeyEnv != "" {
		key, err := base64.StdEncoding.DecodeString(localKeyEnv)
		if err == nil && len(key) == 32 {
			encryptor.localKey = key
		}
	}

	// Generate random key if none provided (development only)
	if encryptor.localKey == nil && os.Getenv("VAULT_TOKEN") == "" {
		encryptor.localKey = make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, encryptor.localKey); err != nil {
			return nil, fmt.Errorf("failed to generate local key: %w", err)
		}
	}

	return encryptor, nil
}

// EncryptField encrypts a single field value
func (f *FieldEncryptor) EncryptField(ctx context.Context, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	// Try Vault Transit first
	if f.vaultClient.vaultToken != "" {
		encrypted, err := f.vaultClient.Encrypt(ctx, f.keyID, []byte(plaintext))
		if err == nil {
			jsonBytes, _ := json.Marshal(encrypted)
			return base64.StdEncoding.EncodeToString(jsonBytes), nil
		}
		// Fall through to local encryption if Vault fails
	}

	// Local AES-256-GCM encryption
	if f.localKey == nil {
		return "", ErrKeyNotFound
	}

	block, err := aes.NewCipher(f.localKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	encrypted := &EncryptedData{
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		KeyID:      f.keyID,
		KeyVersion: f.keyVersion,
		Algorithm:  "aes-256-gcm",
	}

	jsonBytes, _ := json.Marshal(encrypted)
	return base64.StdEncoding.EncodeToString(jsonBytes), nil
}

// DecryptField decrypts a single field value
func (f *FieldEncryptor) DecryptField(ctx context.Context, encryptedValue string) (string, error) {
	if encryptedValue == "" {
		return "", nil
	}

	jsonBytes, err := base64.StdEncoding.DecodeString(encryptedValue)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	var encrypted EncryptedData
	if err := json.Unmarshal(jsonBytes, &encrypted); err != nil {
		return "", ErrInvalidCiphertext
	}

	// Try Vault Transit first for vault-encrypted data
	if strings.HasPrefix(encrypted.Algorithm, "vault-transit") {
		if f.vaultClient.vaultToken != "" {
			plaintext, err := f.vaultClient.Decrypt(ctx, encrypted.KeyID, &encrypted)
			if err == nil {
				return string(plaintext), nil
			}
		}
		return "", ErrVaultUnavailable
	}

	// Local AES-256-GCM decryption
	if f.localKey == nil {
		return "", ErrKeyNotFound
	}

	block, err := aes.NewCipher(f.localKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(encrypted.Nonce)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted.Ciphertext)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// PIIFields defines which fields should be encrypted
var PIIFields = map[string]bool{
	"first_name":        true,
	"last_name":         true,
	"full_name":         true,
	"national_id":       true,
	"phone_number":      true,
	"email":             true,
	"address":           true,
	"date_of_birth":     true,
	"bank_account":      true,
	"biometric_hash":    true,
	"mobile_money_id":   true,
	"passport_number":   true,
	"tax_id":            true,
	"social_security":   true,
}

// EncryptPIIFields encrypts all PII fields in a map
func (f *FieldEncryptor) EncryptPIIFields(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	for key, value := range data {
		if PIIFields[key] {
			if strValue, ok := value.(string); ok {
				encrypted, err := f.EncryptField(ctx, strValue)
				if err != nil {
					return nil, fmt.Errorf("failed to encrypt field %s: %w", key, err)
				}
				result[key] = encrypted
			} else {
				result[key] = value
			}
		} else {
			result[key] = value
		}
	}
	
	return result, nil
}

// DecryptPIIFields decrypts all PII fields in a map
func (f *FieldEncryptor) DecryptPIIFields(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	for key, value := range data {
		if PIIFields[key] {
			if strValue, ok := value.(string); ok {
				decrypted, err := f.DecryptField(ctx, strValue)
				if err != nil {
					return nil, fmt.Errorf("failed to decrypt field %s: %w", key, err)
				}
				result[key] = decrypted
			} else {
				result[key] = value
			}
		} else {
			result[key] = value
		}
	}
	
	return result, nil
}
