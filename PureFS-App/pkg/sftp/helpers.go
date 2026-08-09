package sftp

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"
)

// generateED25519Key creates a new ED25519 key pair and returns the private
// key in OpenSSH PEM format and the raw bytes.
func generateED25519Key() (ssh.Signer, []byte, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	// Marshal to OpenSSH private key format (PKCS8 wrapped)
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return nil, nil, err
	}

	raw := pem.EncodeToMemory(block)

	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, nil, err
	}

	return signer, raw, nil
}

// checkSSHPassword validates a password against a bcrypt hash. It provides
// the same semantics as auth.CheckPassword but is in the sftp package to
// avoid a circular dependency.
func checkSSHPassword(password, hash string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return err
	}
	return nil
}
