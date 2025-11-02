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

The development server runs on http://localhost:5173 by default. Ensure the key and messaging services are reachable from the browser and that CORS permits the origin. The application persists client state in `localStorage` under the key `secumsg-state-v1`. Use the **Reset state** button to clear stored credentials.
