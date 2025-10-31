# SecuMSG Frontend

This React application wraps the Go `msgctl` client in WebAssembly so you can register devices, send encrypted messages, and receive inbound messages from the browser.

## Prerequisites

- Node.js 18+
- Go 1.21+

## Setup

```bash
cd frontend
npm install
npm run prepare:wasm   # copies wasm_exec.js
npm run build:wasm      # compiles the Go client to WebAssembly
npm run dev
```

The development server runs on http://localhost:5173 by default. Ensure the gateway/messaging stack is reachable from the browser and that CORS permits the origin.

The application persists client state in `localStorage` under the key `secumsg-state-v1`. Use the **Reset state** button to clear stored credentials.
