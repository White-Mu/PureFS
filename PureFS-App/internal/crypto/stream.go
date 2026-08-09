package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// Header sizes
const (
	VersionSize       = 1                    // version byte
	KEKVersionSize    = 4                    // kek_version uint32
	NonceSize         = 12                   // AES-GCM standard nonce
	DEKSize           = 32                   // AES-256 key
	DEKCiphertextSize = NonceSize + DEKSize + 16 // nonce + encrypted DEK + GCM tag
	HeaderSize        = VersionSize + KEKVersionSize + DEKCiphertextSize
)

// EncryptDEK encrypts a plaintext DEK using AES-256-GCM with the given KEK.
// Returns nonce + ciphertext + tag.
func EncryptDEK(dek, kek []byte) ([]byte, error) {
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := aesgcm.Seal(nonce, nonce, dek, nil)
	return ciphertext, nil
}

// DecryptDEK decrypts an encrypted DEK using AES-256-GCM with the given KEK.
// The input is nonce + ciphertext + tag.
func DecryptDEK(ciphertext, kek []byte) ([]byte, error) {
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	if len(ciphertext) < NonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, encrypted := ciphertext[:NonceSize], ciphertext[NonceSize:]
	dek, err := aesgcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt DEK: %w", err)
	}
	return dek, nil
}

// GenerateDEK creates a new random 256-bit Data Encryption Key.
func GenerateDEK() ([]byte, error) {
	dek := make([]byte, DEKSize)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, fmt.Errorf("generate DEK: %w", err)
	}
	return dek, nil
}

// EncryptedWriter wraps an io.WriteCloser and transparently encrypts data
// using AES-256-GCM before writing to the underlying writer.
//
// Plaintext is accumulated in memory and encrypted as a single GCM stream
// on Close(). The on-disk format after the header is: nonce + ciphertext + tag.
type EncryptedWriter struct {
	inner  io.WriteCloser
	aesgcm cipher.AEAD
	buf    bytes.Buffer
	closed bool
}

// NewEncryptedWriter creates a new EncryptedWriter.
// The file header (version, kek_version, encrypted DEK) must already be
// written to the underlying writer before calling this constructor.
func NewEncryptedWriter(inner io.WriteCloser, dek []byte) (*EncryptedWriter, error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return &EncryptedWriter{
		inner:  inner,
		aesgcm: aesgcm,
	}, nil
}

// Write accumulates plaintext in an internal buffer.
func (w *EncryptedWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, fmt.Errorf("write on closed EncryptedWriter")
	}
	return w.buf.Write(p)
}

// Close encrypts all accumulated plaintext as a single GCM stream and writes
// it to the underlying writer, then closes the underlying writer.
func (w *EncryptedWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	plaintext := w.buf.Bytes()
	sealed := w.aesgcm.Seal(nonce, nonce, plaintext, nil)

	if _, err := w.inner.Write(sealed); err != nil {
		return fmt.Errorf("write encrypted data: %w", err)
	}

	return w.inner.Close()
}

// EncryptedReader wraps an io.ReadCloser and transparently decrypts data
// using AES-256-GCM from the underlying reader.
//
// The data is read as a single GCM stream: nonce + ciphertext + tag.
// Decryption is performed eagerly on construction and the plaintext is
// buffered for subsequent Read calls.
type EncryptedReader struct {
	inner  io.ReadCloser
	buf    []byte // decrypted plaintext buffer
	pos    int    // read position in buf
	closed bool
}

// NewEncryptedReader creates a new EncryptedReader.
// The file header must already be consumed from the underlying reader
// before calling this constructor. All remaining data from the inner
// reader is read and decrypted immediately.
func NewEncryptedReader(inner io.ReadCloser, dek []byte) (*EncryptedReader, error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	// Read all remaining encrypted data from the inner reader.
	encrypted, err := io.ReadAll(inner)
	if err != nil {
		return nil, fmt.Errorf("read encrypted data: %w", err)
	}

	if len(encrypted) < NonceSize+aesgcm.Overhead() {
		return nil, fmt.Errorf("encrypted data too short: %d bytes", len(encrypted))
	}

	nonce := encrypted[:NonceSize]
	ciphertext := encrypted[NonceSize:]

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return &EncryptedReader{
		inner: inner,
		buf:   plaintext,
	}, nil
}

// Read copies decrypted plaintext from the internal buffer.
func (r *EncryptedReader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, fmt.Errorf("read on closed EncryptedReader")
	}
	if r.pos >= len(r.buf) {
		return 0, io.EOF
	}
	n := copy(p, r.buf[r.pos:])
	r.pos += n
	return n, nil
}

// Close closes the underlying reader.
func (r *EncryptedReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.inner.Close()
}
