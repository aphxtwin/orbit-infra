package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
)

const (
	// DefaultPasswordLength is the default length for generated passwords
	DefaultPasswordLength = 32
	// DefaultSecretLength is the default length for generated secrets (in bytes, before base64 encoding)
	DefaultSecretLength = 48
)

// PasswordCharset defines the characters allowed in generated passwords
// Includes uppercase, lowercase, digits, and safe special characters
const PasswordCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*-_=+"

// GeneratePassword generates a cryptographically secure random password
// Returns a string of the specified length using characters from PasswordCharset
func GeneratePassword(length int) (string, error) {
	if length <= 0 {
		length = DefaultPasswordLength
	}

	password := make([]byte, length)
	charsetLen := big.NewInt(int64(len(PasswordCharset)))

	for i := range password {
		randomIndex, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		password[i] = PasswordCharset[randomIndex.Int64()]
	}

	return string(password), nil
}

// GenerateSecret generates a cryptographically secure random secret
// Returns a base64-encoded string suitable for JWT secrets, webhook secrets, etc.
func GenerateSecret(byteLength int) (string, error) {
	if byteLength <= 0 {
		byteLength = DefaultSecretLength
	}

	secretBytes := make([]byte, byteLength)
	_, err := rand.Read(secretBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode to base64 for easy storage and transmission
	return base64.URLEncoding.EncodeToString(secretBytes), nil
}

// GenerateDatabaseName generates a database name from a subdomain
// Format: odoo_{subdomain}_{environment}
func GenerateDatabaseName(subdomain, environment string) string {
	return fmt.Sprintf("odoo_%s_%s", subdomain, environment)
}

// GenerateDatabaseUser generates a database username from a subdomain
// Format: odoo_{subdomain}
func GenerateDatabaseUser(subdomain string) string {
	return fmt.Sprintf("odoo_%s", subdomain)
}

// GenerateOdooHost generates the Odoo host URL from a subdomain
// Format: {subdomain}-odoo.bici-dev.com
func GenerateOdooHost(subdomain string) string {
	return fmt.Sprintf("%s.bici-dev.com", subdomain)
}
