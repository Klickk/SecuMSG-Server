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
const BASE_URL = __ENV.BASE_URL || "http://localhost:8081";

const ACCESS_TOKEN =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzaWQiOiI0ZDJjZTY3YS1lNDNkLTQ1NzYtOTVmMS1iMGZlZDJlNDU4ODEiLCJzY29wZSI6InVzZXIiLCJpc3MiOiJodHRwOi8vYXV0aDo4MDgxIiwic3ViIjoiMTJjNTVkZGMtNWVkYS00OWQ2LThkYTAtODIxZWI3YjU1MDI1IiwiYXVkIjpbImNsaWVudCJdLCJleHAiOjE3NjUxMjc2MjcsImlhdCI6MTc2NTEyNjcyNywianRpIjoiYjg5YjQwOGQtYWU2YS00YmZmLWIzYmUtMWU5MDYxMDMzMTI2In0.kXpBjFxtn8hUZ1WS1AmrZ4vIFZ8Pr6_MyErxbwiphLs";
const CONV_ID = __ENV.CONV_ID || "00000000-0000-0000-0000-000000000002";
const FROM_DEVICE_ID = "2d6dd2ed-f26b-42a4-9a6f-378a88cc58aa";
const TO_DEVICE_ID = "19da5129-f99b-43e4-bd96-f74e53b660e5";

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
