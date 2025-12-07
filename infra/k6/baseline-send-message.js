import http from "k6/http";
import encoding from "k6/encoding";
import { check, sleep } from "k6";

export const options = {
  stages: [
    { duration: "1m", target: 10 }, // ramp up to 10 users
    { duration: "3m", target: 10 }, // stay at 10
    { duration: "1m", target: 0 }, // ramp down
  ],
};

// Configure via env when running k6:
// BASE_URL=http://localhost:8080 ACCESS_TOKEN=... FROM_DEVICE_ID=... TO_DEVICE_ID=... k6 run baseline-send-message.js
const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

const ACCESS_TOKEN =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzaWQiOiIwZDNkMmJmMy01NWQ5LTQ2OGUtYTZkOS02YTlmODc0MjU5ODUiLCJzY29wZSI6InVzZXIiLCJpc3MiOiJodHRwOi8vYXV0aDo4MDgxIiwic3ViIjoiNDc1OTJmNWItOTA2NS00NmNhLWExMzMtYTZlYzQ5ZTUzOTk1IiwiYXVkIjpbImNsaWVudCJdLCJleHAiOjE3NjUxMjk4NDQsImlhdCI6MTc2NTEyODk0NCwianRpIjoiZTJiNTAwNTEtN2Q2OS00ZDk5LTgxN2YtNTM3ZTE1OTcxZThhIn0.G2SpG5mRmEB7-N5Y7s8A7iRyzekJ4-fipsSPLhZ5YVY";
const CONV_ID = __ENV.CONV_ID || "00000000-0000-0000-0000-000000000003";
const FROM_DEVICE_ID = "36155f3c-0457-4dee-888c-26caabf21a8d";
const TO_DEVICE_ID = "f209d94a-1ab3-444d-b031-06b94413d5d0";

function b64(bytes) {
  return encoding.b64encode(bytes, "std");
}

export default function () {
  if (!ACCESS_TOKEN || !FROM_DEVICE_ID || !TO_DEVICE_ID) {
    throw new Error(
      "Set ACCESS_TOKEN, FROM_DEVICE_ID, and TO_DEVICE_ID env vars to hit /messages/send"
    );
  }

  const ciphertext = b64(`hello from k6 vu=${__VU} iter=${__ITER}`);
  const header = { ratchet: { nonce: `n-${__VU}-${__ITER}` } }; // opaque to the backend

  const payload = JSON.stringify({
    conv_id: CONV_ID,
    from_device_id: FROM_DEVICE_ID,
    to_device_id: TO_DEVICE_ID,
    ciphertext,
    header,
  });

  // Send to the gateway; it proxies to messages service.
  const params = {
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${ACCESS_TOKEN}`,
    },
  };

  const res = http.post(`${BASE_URL}/messages/send`, payload, params);
  if (res.status !== 201 && res.status !== 200) {
    console.log({ status: res.status, body: res.body });
  }
  check(res, {
    "status is 200 or 201": (r) => r.status === 200 || r.status === 201,
  });

  sleep(1);
}
