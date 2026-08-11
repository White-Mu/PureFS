// End-to-end encryption primitives built on WebCrypto.
//
// Model (envelope encryption, standard E2EE):
//   - A random 32-byte MASTER KEY is generated on the client and never leaves it
//     except in wrapped form.
//   - A per-user SALT plus the user's E2EE passphrase derive a Key-Encryption-Key
//     (PBKDF2-SHA256, 150k iterations).
//   - The master key is wrapped (AES-256-GCM) with that KEK and stored on the
//     server (users.e2ee_wrapped_key). The server cannot unwrap it.
//   - Each file gets a random per-file DEK (Data Encryption Key). File bytes are
//     encrypted with the DEK (AES-256-GCM). The DEK is then wrapped with the
//     master key and stored server-side (files.dek_ciphertext, kek_version=0).
//   - The server only ever stores and serves ciphertext. Forgetting the
//     passphrase makes the wrapped master key permanently unrecoverable.
//
// On-disk / on-wire formats (both are nonce(12) || AES-GCM-ciphertext+tag):
//   - wrapped_key / wrapped DEK
//   - file ciphertext

const encoder = new TextEncoder();

// Wire format helpers: Uint8Array <-> base64.
function bufToB64(buf: Uint8Array): string {
  let bin = '';
  const chunk = 0x8000;
  for (let i = 0; i < buf.length; i += chunk) {
    bin += String.fromCharCode(...buf.subarray(i, i + chunk));
  }
  return btoa(bin);
}

function b64ToBuf(b64: string): Uint8Array {
  const bin = atob(b64);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return bytes;
}

// Derive the AES-256-GCM KEK from the passphrase + per-user salt.
export async function deriveKEK(passphrase: string, saltB64: string): Promise<CryptoKey> {
  const salt = b64ToBuf(saltB64);
  const baseKey = await crypto.subtle.importKey(
    'raw', encoder.encode(passphrase), 'PBKDF2', false, ['deriveKey'],
  );
  return crypto.subtle.deriveKey(
    { name: 'PBKDF2', salt, iterations: 150000, hash: 'SHA-256' },
    baseKey,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt'],
  );
}

// Generate a random 32-byte master key.
export function generateMasterKey(): Uint8Array {
  return crypto.getRandomValues(new Uint8Array(32));
}

// Generate a random salt for PBKDF2.
export function generateSalt(): string {
  return bufToB64(crypto.getRandomValues(new Uint8Array(16)));
}

// Wrap a raw key (master key or DEK) with an AES-GCM key. Returns base64.
async function sealKey(rawKey: Uint8Array, key: CryptoKey): Promise<string> {
  const nonce = crypto.getRandomValues(new Uint8Array(12));
  const sealed = await crypto.subtle.encrypt({ name: 'AES-GCM', iv: nonce }, key, rawKey);
  const out = new Uint8Array(nonce.length + sealed.byteLength);
  out.set(nonce);
  out.set(new Uint8Array(sealed), nonce.length);
  return bufToB64(out);
}

// Unwrap a base64-wrapped key into an AES-GCM CryptoKey.
async function openKey(wrappedB64: string, key: CryptoKey): Promise<CryptoKey> {
  const data = b64ToBuf(wrappedB64);
  const nonce = data.slice(0, 12);
  const ct = data.slice(12);
  const raw = await crypto.subtle.decrypt({ name: 'AES-GCM', iv: nonce }, key, ct);
  return crypto.subtle.importKey('raw', raw, 'AES-GCM', false, ['encrypt', 'decrypt']);
}

// Wrap the master key with the passphrase-derived KEK (for server storage).
export function wrapMasterKey(masterKey: Uint8Array, kek: CryptoKey): Promise<string> {
  return sealKey(masterKey, kek);
}

// Unwrap the master key stored on the server into a usable AES-GCM CryptoKey.
export function unwrapMasterKey(wrappedKeyB64: string, kek: CryptoKey): Promise<CryptoKey> {
  return openKey(wrappedKeyB64, kek);
}

// Encrypt file data with a fresh per-file DEK. Returns the ciphertext blob and
// the base64 master-key-wrapped DEK to store server-side.
export async function encryptFile(
  data: ArrayBuffer,
  masterKey: CryptoKey,
): Promise<{ ciphertext: Blob; wrappedDEK: string }> {
  const dek = crypto.getRandomValues(new Uint8Array(32));
  const dekKey = await crypto.subtle.importKey('raw', dek, 'AES-GCM', false, ['encrypt']);

  const fileNonce = crypto.getRandomValues(new Uint8Array(12));
  const ct = await crypto.subtle.encrypt({ name: 'AES-GCM', iv: fileNonce }, dekKey, data);

  const ciphertext = new Uint8Array(fileNonce.length + ct.byteLength);
  ciphertext.set(fileNonce);
  ciphertext.set(new Uint8Array(ct), fileNonce.length);

  const wrappedDEK = await sealKey(dek, masterKey);
  return { ciphertext: new Blob([ciphertext]), wrappedDEK };
}

// Decrypt file ciphertext using the server-stored wrapped DEK and the unlocked
// master key. Returns the plaintext bytes.
export async function decryptFile(
  ciphertext: ArrayBuffer,
  wrappedDEKB64: string,
  masterKey: CryptoKey,
): Promise<ArrayBuffer> {
  const dekKey = await openKey(wrappedDEKB64, masterKey);

  const data = new Uint8Array(ciphertext);
  const fileNonce = data.slice(0, 12);
  const ct = data.slice(12);
  return crypto.subtle.decrypt({ name: 'AES-GCM', iv: fileNonce }, dekKey, ct);
}
