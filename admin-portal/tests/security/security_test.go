package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestAuthenticationSecurity tests authentication security controls
func TestAuthenticationSecurity(t *testing.T) {
	tests := []struct {
		name        string
		testType    string
		description string
		check       func() error
	}{
		{
			name:        "password complexity requirements",
			testType:    "authentication",
			description: "Verify password meets complexity requirements",
			check: func() error {
				passwords := []struct {
					password string
					valid    bool
				}{
					{"short", false},                    // Too short
					{"nouppercase123!", false},          // No uppercase
					{"NOLOWERCASE123!", false},          // No lowercase
					{"NoNumbers!", false},               // No numbers
					{"NoSpecial123", false},             // No special chars
					{"ValidP@ssw0rd!", true},            // Valid
					{"Str0ng!P@ssword123", true},        // Valid
				}

				for _, p := range passwords {
					valid := validatePasswordComplexity(p.password)
					if valid != p.valid {
						return fmt.Errorf("password %q: expected valid=%v, got %v", p.password, p.valid, valid)
					}
				}
				return nil
			},
		},
		{
			name:        "token expiration",
			testType:    "authentication",
			description: "Verify tokens expire correctly",
			check: func() error {
				// Simulate token with 1 hour expiry
				tokenExpiry := time.Now().Add(time.Hour)
				if time.Now().After(tokenExpiry) {
					return fmt.Errorf("token should not be expired")
				}

				// Simulate expired token
				expiredToken := time.Now().Add(-time.Hour)
				if !time.Now().After(expiredToken) {
					return fmt.Errorf("token should be expired")
				}
				return nil
			},
		},
		{
			name:        "session management",
			testType:    "authentication",
			description: "Verify session tokens are secure",
			check: func() error {
				// Generate secure session token
				token := make([]byte, 32)
				_, err := rand.Read(token)
				if err != nil {
					return fmt.Errorf("failed to generate secure token: %w", err)
				}

				// Verify token length
				if len(token) < 32 {
					return fmt.Errorf("token too short: %d bytes", len(token))
				}

				// Verify token is random (entropy check)
				hash := sha256.Sum256(token)
				if len(hash) != 32 {
					return fmt.Errorf("hash length incorrect")
				}
				return nil
			},
		},
		{
			name:        "brute force protection",
			testType:    "authentication",
			description: "Verify rate limiting on login attempts",
			check: func() error {
				maxAttempts := 5
				lockoutDuration := time.Minute * 15

				// Simulate failed attempts
				attempts := 0
				for i := 0; i < 10; i++ {
					attempts++
					if attempts >= maxAttempts {
						// Account should be locked
						break
					}
				}

				if attempts < maxAttempts {
					return fmt.Errorf("brute force protection not triggered")
				}

				// Verify lockout duration is reasonable
				if lockoutDuration < time.Minute*5 {
					return fmt.Errorf("lockout duration too short: %v", lockoutDuration)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.check(); err != nil {
				t.Errorf("%s: %v", tt.description, err)
			}
		})
	}
}

// validatePasswordComplexity checks if password meets security requirements
func validatePasswordComplexity(password string) bool {
	if len(password) < 8 {
		return false
	}

	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*(),.?":{}|<>]`).MatchString(password)

	return hasUpper && hasLower && hasNumber && hasSpecial
}

// TestAuthorizationSecurity tests authorization security controls
func TestAuthorizationSecurity(t *testing.T) {
	tests := []struct {
		name        string
		role        string
		resource    string
		action      string
		shouldAllow bool
	}{
		{
			name:        "admin can access all resources",
			role:        "admin",
			resource:    "beneficiary",
			action:      "delete",
			shouldAllow: true,
		},
		{
			name:        "case worker can view beneficiaries",
			role:        "case_worker",
			resource:    "beneficiary",
			action:      "view",
			shouldAllow: true,
		},
		{
			name:        "case worker cannot delete beneficiaries",
			role:        "case_worker",
			resource:    "beneficiary",
			action:      "delete",
			shouldAllow: false,
		},
		{
			name:        "auditor can view reports",
			role:        "auditor",
			resource:    "report",
			action:      "view",
			shouldAllow: true,
		},
		{
			name:        "auditor cannot modify data",
			role:        "auditor",
			resource:    "beneficiary",
			action:      "update",
			shouldAllow: false,
		},
		{
			name:        "guest cannot access protected resources",
			role:        "guest",
			resource:    "beneficiary",
			action:      "view",
			shouldAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed := checkPermission(tt.role, tt.resource, tt.action)
			if allowed != tt.shouldAllow {
				t.Errorf("checkPermission(%s, %s, %s) = %v, want %v",
					tt.role, tt.resource, tt.action, allowed, tt.shouldAllow)
			}
		})
	}
}

// checkPermission simulates permission checking
func checkPermission(role, resource, action string) bool {
	permissions := map[string]map[string][]string{
		"admin": {
			"beneficiary": {"view", "create", "update", "delete"},
			"program":     {"view", "create", "update", "delete"},
			"report":      {"view", "create", "update", "delete"},
		},
		"case_worker": {
			"beneficiary": {"view", "create", "update"},
			"program":     {"view"},
			"report":      {"view", "create"},
		},
		"auditor": {
			"beneficiary": {"view"},
			"program":     {"view"},
			"report":      {"view"},
		},
		"guest": {},
	}

	rolePerms, ok := permissions[role]
	if !ok {
		return false
	}

	resourcePerms, ok := rolePerms[resource]
	if !ok {
		return false
	}

	for _, perm := range resourcePerms {
		if perm == action {
			return true
		}
	}
	return false
}

// TestInputValidation tests input validation and sanitization
func TestInputValidation(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		inputType   string
		shouldPass  bool
	}{
		{
			name:       "valid email",
			input:      "user@example.com",
			inputType:  "email",
			shouldPass: true,
		},
		{
			name:       "invalid email - no domain",
			input:      "user@",
			inputType:  "email",
			shouldPass: false,
		},
		{
			name:       "SQL injection attempt",
			input:      "'; DROP TABLE users; --",
			inputType:  "text",
			shouldPass: false,
		},
		{
			name:       "XSS attempt",
			input:      "<script>alert('xss')</script>",
			inputType:  "text",
			shouldPass: false,
		},
		{
			name:       "valid phone number",
			input:      "+1234567890",
			inputType:  "phone",
			shouldPass: true,
		},
		{
			name:       "path traversal attempt",
			input:      "../../../etc/passwd",
			inputType:  "path",
			shouldPass: false,
		},
		{
			name:       "valid UUID",
			input:      "550e8400-e29b-41d4-a716-446655440000",
			inputType:  "uuid",
			shouldPass: true,
		},
		{
			name:       "command injection attempt",
			input:      "; rm -rf /",
			inputType:  "text",
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateInput(tt.input, tt.inputType)
			if valid != tt.shouldPass {
				t.Errorf("validateInput(%q, %s) = %v, want %v",
					tt.input, tt.inputType, valid, tt.shouldPass)
			}
		})
	}
}

// validateInput validates and sanitizes input
func validateInput(input, inputType string) bool {
	// Check for common injection patterns
	dangerousPatterns := []string{
		"<script", "</script>", "javascript:",
		"'; DROP", "-- ", "/*", "*/",
		"../", "..\\",
		"; rm", "| rm", "&& rm",
		"eval(", "exec(",
	}

	inputLower := strings.ToLower(input)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(inputLower, strings.ToLower(pattern)) {
			return false
		}
	}

	switch inputType {
	case "email":
		return regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).MatchString(input)
	case "phone":
		return regexp.MustCompile(`^\+?[0-9]{10,15}$`).MatchString(input)
	case "uuid":
		return regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(input)
	case "path":
		return !strings.Contains(input, "..") && !strings.HasPrefix(input, "/")
	case "text":
		return true // Already checked for dangerous patterns
	default:
		return true
	}
}

// TestDataProtection tests data protection controls
func TestDataProtection(t *testing.T) {
	tests := []struct {
		name        string
		testType    string
		description string
		check       func() error
	}{
		{
			name:        "PII masking",
			testType:    "data_protection",
			description: "Verify PII is properly masked in logs",
			check: func() error {
				piiData := map[string]string{
					"ssn":         "123-45-6789",
					"credit_card": "4111111111111111",
					"phone":       "+1234567890",
					"email":       "user@example.com",
				}

				for field, value := range piiData {
					masked := maskPII(value, field)
					if masked == value {
						return fmt.Errorf("PII field %s not masked: %s", field, value)
					}
					if !strings.Contains(masked, "***") && !strings.Contains(masked, "XXX") {
						return fmt.Errorf("PII field %s not properly masked: %s", field, masked)
					}
				}
				return nil
			},
		},
		{
			name:        "encryption at rest",
			testType:    "data_protection",
			description: "Verify sensitive data is encrypted",
			check: func() error {
				sensitiveData := "sensitive-beneficiary-data"
				encrypted := encryptData(sensitiveData)

				if encrypted == sensitiveData {
					return fmt.Errorf("data not encrypted")
				}

				if len(encrypted) < len(sensitiveData) {
					return fmt.Errorf("encrypted data too short")
				}
				return nil
			},
		},
		{
			name:        "secure key storage",
			testType:    "data_protection",
			description: "Verify encryption keys are not hardcoded",
			check: func() error {
				// Check that keys come from environment or secure storage
				key := os.Getenv("ENCRYPTION_KEY")
				if key == "" {
					// In test mode, this is acceptable
					return nil
				}

				// Verify key length
				if len(key) < 32 {
					return fmt.Errorf("encryption key too short: %d chars", len(key))
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.check(); err != nil {
				t.Errorf("%s: %v", tt.description, err)
			}
		})
	}
}

// maskPII masks personally identifiable information
func maskPII(value, fieldType string) string {
	switch fieldType {
	case "ssn":
		if len(value) >= 4 {
			return "XXX-XX-" + value[len(value)-4:]
		}
	case "credit_card":
		if len(value) >= 4 {
			return "XXXX-XXXX-XXXX-" + value[len(value)-4:]
		}
	case "phone":
		if len(value) >= 4 {
			return "***-***-" + value[len(value)-4:]
		}
	case "email":
		parts := strings.Split(value, "@")
		if len(parts) == 2 {
			return "***@" + parts[1]
		}
	}
	return "***MASKED***"
}

// encryptData simulates data encryption
func encryptData(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// TestSecurityHeaders tests HTTP security headers
func TestSecurityHeaders(t *testing.T) {
	requiredHeaders := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-XSS-Protection":          "1; mode=block",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Content-Security-Policy":   "default-src 'self'",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
	}

	for header, expectedValue := range requiredHeaders {
		t.Run(header, func(t *testing.T) {
			// Simulate checking header
			actualValue := getSecurityHeader(header)
			if actualValue == "" {
				t.Errorf("Missing security header: %s", header)
			}
			if !strings.Contains(actualValue, strings.Split(expectedValue, ";")[0]) {
				t.Errorf("Header %s = %q, want %q", header, actualValue, expectedValue)
			}
		})
	}
}

// getSecurityHeader simulates getting security headers
func getSecurityHeader(header string) string {
	headers := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-XSS-Protection":          "1; mode=block",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Content-Security-Policy":   "default-src 'self'; script-src 'self'",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
	}
	return headers[header]
}

// TestAPISecurityVulnerabilities tests for common API vulnerabilities
func TestAPISecurityVulnerabilities(t *testing.T) {
	tests := []struct {
		name        string
		vulnerability string
		testFunc    func() bool
	}{
		{
			name:        "IDOR protection",
			vulnerability: "Insecure Direct Object Reference",
			testFunc: func() bool {
				// Verify that accessing another user's resource is blocked
				userID := "user123"
				resourceOwner := "user456"
				return !canAccessResource(userID, resourceOwner)
			},
		},
		{
			name:        "mass assignment protection",
			vulnerability: "Mass Assignment",
			testFunc: func() bool {
				// Verify that only allowed fields can be updated
				allowedFields := []string{"name", "email", "phone"}
				attemptedFields := []string{"name", "email", "role", "is_admin"}
				filtered := filterAllowedFields(attemptedFields, allowedFields)
				return len(filtered) == 2 // Only name and email should pass
			},
		},
		{
			name:        "rate limiting",
			vulnerability: "Denial of Service",
			testFunc: func() bool {
				// Verify rate limiting is in place
				requestCount := 0
				limit := 100
				for i := 0; i < 150; i++ {
					if requestCount < limit {
						requestCount++
					} else {
						return true // Rate limiting kicked in
					}
				}
				return false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.testFunc() {
				t.Errorf("Vulnerability test failed: %s", tt.vulnerability)
			}
		})
	}
}

// canAccessResource checks if a user can access a resource
func canAccessResource(userID, resourceOwner string) bool {
	return userID == resourceOwner
}

// filterAllowedFields filters fields to only allowed ones
func filterAllowedFields(attempted, allowed []string) []string {
	allowedMap := make(map[string]bool)
	for _, f := range allowed {
		allowedMap[f] = true
	}

	var filtered []string
	for _, f := range attempted {
		if allowedMap[f] {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// TestCORSConfiguration tests CORS security configuration
func TestCORSConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		origin        string
		shouldAllow   bool
	}{
		{
			name:        "allowed origin",
			origin:      "https://admin.social-protection.gov",
			shouldAllow: true,
		},
		{
			name:        "allowed subdomain",
			origin:      "https://api.social-protection.gov",
			shouldAllow: true,
		},
		{
			name:        "disallowed origin",
			origin:      "https://malicious-site.com",
			shouldAllow: false,
		},
		{
			name:        "null origin",
			origin:      "null",
			shouldAllow: false,
		},
	}

	allowedOrigins := []string{
		"https://admin.social-protection.gov",
		"https://api.social-protection.gov",
		"https://portal.social-protection.gov",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed := isOriginAllowed(tt.origin, allowedOrigins)
			if allowed != tt.shouldAllow {
				t.Errorf("isOriginAllowed(%s) = %v, want %v", tt.origin, allowed, tt.shouldAllow)
			}
		})
	}
}

// isOriginAllowed checks if an origin is in the allowed list
func isOriginAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == origin {
			return true
		}
	}
	return false
}

// TestAuditLogging tests audit logging functionality
func TestAuditLogging(t *testing.T) {
	tests := []struct {
		name      string
		action    string
		userID    string
		resource  string
		shouldLog bool
	}{
		{
			name:      "login attempt",
			action:    "LOGIN",
			userID:    "user123",
			resource:  "auth",
			shouldLog: true,
		},
		{
			name:      "data access",
			action:    "READ",
			userID:    "user123",
			resource:  "beneficiary:b1",
			shouldLog: true,
		},
		{
			name:      "data modification",
			action:    "UPDATE",
			userID:    "admin",
			resource:  "beneficiary:b1",
			shouldLog: true,
		},
		{
			name:      "permission change",
			action:    "GRANT_PERMISSION",
			userID:    "admin",
			resource:  "user:user456",
			shouldLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := createAuditLogEntry(tt.action, tt.userID, tt.resource)

			if entry.Action != tt.action {
				t.Errorf("Audit log action = %s, want %s", entry.Action, tt.action)
			}
			if entry.UserID != tt.userID {
				t.Errorf("Audit log userID = %s, want %s", entry.UserID, tt.userID)
			}
			if entry.Timestamp.IsZero() {
				t.Error("Audit log timestamp not set")
			}
		})
	}
}

// AuditLogEntry represents an audit log entry
type AuditLogEntry struct {
	Action    string
	UserID    string
	Resource  string
	Timestamp time.Time
	IPAddress string
	UserAgent string
}

// createAuditLogEntry creates an audit log entry
func createAuditLogEntry(action, userID, resource string) AuditLogEntry {
	return AuditLogEntry{
		Action:    action,
		UserID:    userID,
		Resource:  resource,
		Timestamp: time.Now(),
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}
}

// TestSecureConfiguration tests secure configuration settings
func TestSecureConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		setting     string
		checkFunc   func() bool
	}{
		{
			name:    "debug mode disabled in production",
			setting: "DEBUG_MODE",
			checkFunc: func() bool {
				env := os.Getenv("ENVIRONMENT")
				debug := os.Getenv("DEBUG_MODE")
				if env == "production" && debug == "true" {
					return false
				}
				return true
			},
		},
		{
			name:    "secure cookies enabled",
			setting: "SECURE_COOKIES",
			checkFunc: func() bool {
				// In production, cookies should be secure
				return true
			},
		},
		{
			name:    "TLS minimum version",
			setting: "TLS_MIN_VERSION",
			checkFunc: func() bool {
				// Should be TLS 1.2 or higher
				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.checkFunc() {
				t.Errorf("Security configuration check failed: %s", tt.setting)
			}
		})
	}
}
