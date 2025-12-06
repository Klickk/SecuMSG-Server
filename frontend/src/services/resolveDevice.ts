import axios from "axios";
import config from "../config/config";
import { authHeaders } from "../lib/authToken";

export type ResolveDeviceResponse = {
  userId: string;
  username: string;
  deviceId: string;
};

export async function resolveContactByDevice(
  deviceId: string
): Promise<ResolveDeviceResponse> {
  const resp = await axios.post(
    `${config.apiBaseUrl}/auth/resolve-device`,
    { deviceId },
    { headers: await authHeaders() }
  );
  return resp.data as ResolveDeviceResponse;
}
