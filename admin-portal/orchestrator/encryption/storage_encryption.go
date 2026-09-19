// Package encryption provides storage-level encryption configuration
package encryption

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// StorageEncryptionConfig defines encryption settings for different storage backends
type StorageEncryptionConfig struct {
	PostgreSQL PostgreSQLEncryption `json:"postgresql"`
	Redis      RedisEncryption      `json:"redis"`
	S3         S3Encryption         `json:"s3"`
}

// PostgreSQLEncryption defines PostgreSQL encryption settings
type PostgreSQLEncryption struct {
	VolumeEncryption    bool   `json:"volume_encryption"`
	ColumnEncryption    bool   `json:"column_encryption"`
	WALEncryption       bool   `json:"wal_encryption"`
	BackupEncryption    bool   `json:"backup_encryption"`
	EncryptionAlgorithm string `json:"encryption_algorithm"`
	KeyManagement       string `json:"key_management"`
}

// RedisEncryption defines Redis encryption settings
type RedisEncryption struct {
	PersistenceEncryption bool   `json:"persistence_encryption"`
	AOFEncryption         bool   `json:"aof_encryption"`
	RDBEncryption         bool   `json:"rdb_encryption"`
	TLSEnabled            bool   `json:"tls_enabled"`
	EncryptionAlgorithm   string `json:"encryption_algorithm"`
}

// S3Encryption defines S3/RustFS encryption settings
type S3Encryption struct {
	ServerSideEncryption bool   `json:"server_side_encryption"`
	SSEAlgorithm         string `json:"sse_algorithm"`
	KMSKeyID             string `json:"kms_key_id"`
	ClientSideEncryption bool   `json:"client_side_encryption"`
	BucketKeyEnabled     bool   `json:"bucket_key_enabled"`
}

// S3EncryptionClient handles client-side encryption for S3/RustFS
type S3EncryptionClient struct {
	encryptor *FieldEncryptor
	kmsKeyID  string
}

// NewS3EncryptionClient creates a new S3 encryption client
func NewS3EncryptionClient() (*S3EncryptionClient, error) {
	encryptor, err := NewFieldEncryptor()
	if err != nil {
		return nil, err
	}

	kmsKeyID := os.Getenv("S3_KMS_KEY_ID")
	if kmsKeyID == "" {
		kmsKeyID = "social-protection-s3"
	}

	return &S3EncryptionClient{
		encryptor: encryptor,
		kmsKeyID:  kmsKeyID,
	}, nil
}

// EncryptObject encrypts an object before uploading to S3
func (s *S3EncryptionClient) EncryptObject(ctx context.Context, data []byte) ([]byte, map[string]string, error) {
	// Generate a random data encryption key (DEK)
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, nil, fmt.Errorf("failed to generate DEK: %w", err)
	}

	// Encrypt the DEK with the master key (KEK) via Vault or local
	encryptedDEK, err := s.encryptor.EncryptField(ctx, base64.StdEncoding.EncodeToString(dek))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt DEK: %w", err)
	}

	// Encrypt the data with the DEK using AES-256-GCM
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	// Return encrypted data and metadata headers
	metadata := map[string]string{
		"x-amz-meta-encrypted-dek": encryptedDEK,
		"x-amz-meta-encryption":    "AES256-GCM",
		"x-amz-meta-key-id":        s.kmsKeyID,
	}

	return ciphertext, metadata, nil
}

// DecryptObject decrypts an object downloaded from S3
func (s *S3EncryptionClient) DecryptObject(ctx context.Context, ciphertext []byte, metadata map[string]string) ([]byte, error) {
	encryptedDEK, ok := metadata["x-amz-meta-encrypted-dek"]
	if !ok {
		return nil, fmt.Errorf("missing encrypted DEK in metadata")
	}

	// Decrypt the DEK
	dekBase64, err := s.encryptor.DecryptField(ctx, encryptedDEK)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt DEK: %w", err)
	}

	dek, err := base64.StdEncoding.DecodeString(dekBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DEK: %w", err)
	}

	// Decrypt the data with the DEK
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return plaintext, nil
}

// GetDefaultConfig returns the default encryption configuration
func GetDefaultConfig() *StorageEncryptionConfig {
	return &StorageEncryptionConfig{
		PostgreSQL: PostgreSQLEncryption{
			VolumeEncryption:    true,
			ColumnEncryption:    true,
			WALEncryption:       true,
			BackupEncryption:    true,
			EncryptionAlgorithm: "AES-256-GCM",
			KeyManagement:       "vault-transit",
		},
		Redis: RedisEncryption{
			PersistenceEncryption: true,
			AOFEncryption:         true,
			RDBEncryption:         true,
			TLSEnabled:            true,
			EncryptionAlgorithm:   "AES-256-GCM",
		},
		S3: S3Encryption{
			ServerSideEncryption: true,
			SSEAlgorithm:         "aws:kms",
			KMSKeyID:             "alias/social-protection-s3",
			ClientSideEncryption: true,
			BucketKeyEnabled:     true,
		},
	}
}
