# SecuMSG Frontend

This React application provides a browser UI for the SecuMSG messaging stack. The browser loads the Go `msgclient` library as a WebAssembly module so all key generation, encryption, and decryption happens locally in the client.

## Prerequisites

- Node.js 18+
- A running deployment of the key and messaging services that the browser can reach

## Setup

```bash
cd frontend
npm install
npm run dev
```

The `predev` and `prebuild` scripts automatically compile the WebAssembly module and copy the matching `wasm_exec.js` shim into `frontend/public`. If you need to refresh the module manually (for example after editing the Go sources) run:

```bash
npm run prepare:wasm
```

The development server runs on http://localhost:5173 by default. Ensure the key and messaging services are reachable from the browser and that CORS permits the origin. The application persists client state in IndexedDB under the key `secumsg-state-v1`. Use the **Reset state** button to clear stored credentials.

## PIN-based key wrapping

The browser uses `frontend/src/crypto-core/keyManager.ts` to derive a key-encryption key (KEK) from a 4-digit PIN, wrap a randomly generated data-encryption key (DEK), and store only the wrapped DEK in IndexedDB. We use PBKDF2-SHA256 with 250k iterations and a per-profile random salt. Argon2id would be stronger but requires a heavier WASM dependency; PBKDF2 is a pragmatic fallback and is explicitly weaker against offline guessing.

Security notes:
- A 4-digit PIN is low entropy; PBKDF2 slows online and local attempts but does not prevent offline brute force if the database is copied.
- The DEK never leaves memory in plaintext; the stored record includes only `{wrappedDEK, wrapIv, wrapAad, kdfParams, salt, version}`.
- Unlock throttling is best-effort UX protection and does not stop offline attacks.

## SecureStore coverage

The `SecureStore` wrapper encrypts sensitive IndexedDB records using the DEK. It encrypts the entire JSON payload per record (AES-GCM with per-record IV) and binds AAD to `{storeName, key, userId, version, origin}`.

| Data | Storage | Encrypted? | Notes |
| --- | --- | --- | --- |
| Messaging state (device keys + sessions) | secure/messaging-state | Yes | Encrypted blob per device state |
| Message history cache | secure/messages | Yes | Encrypted blob per conversation |
| Contacts list | kv/CONTACTS_KEY | No | Display metadata |
| Username, access/refresh tokens, userId | kv/* | No | Needed before unlock |
