# SecuMSG Frontend

This React application provides a browser UI for the SecuMSG messaging stack. It talks to the backend through the `/client/*` HTT
P helpers that wrap the existing Go `msgctl` logic on the server, so no WebAssembly build steps are required in the browser.

## Prerequisites

- Node.js 18+
- A running deployment of the key and messaging services that exposes the `/client/*` endpoints (part of the messages service).

## Setup

```bash
cd frontend
npm install
npm run dev
```

Set the `VITE_CLIENT_API_URL` environment variable if the helper endpoints are not available on `http://localhost:8080`:

```bash
VITE_CLIENT_API_URL="http://runner-host:8080" npm run dev
```

The development server runs on http://localhost:5173 by default. Ensure the key and messaging services are reachable from the bro
wser and that CORS permits the origin.

The application persists client state in `localStorage` under the key `secumsg-state-v1`. Use the **Reset state** button to clear
stored credentials.
