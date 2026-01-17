import axios from "axios";
import config from "../config/config";
import { authHeaders } from "../lib/authToken";

export type DeleteServicePayload = {
  status: string;
  deletedResources: Record<string, number>;
  timestamp: string;
};

export type DeleteProfilePayload = {
  status: string;
  deletedResources?: Record<string, Record<string, number>>;
  errors?: Record<string, string>;
  timestamp: string;
};

export async function deleteAuthData(): Promise<DeleteServicePayload> {
  const headers = await authHeaders();
  const resp = await axios.delete(`${config.apiBaseUrl}/auth/me`, { headers });
  return resp.data as DeleteServicePayload;
}

export async function deleteKeysData(): Promise<DeleteServicePayload> {
  const headers = await authHeaders();
  const resp = await axios.delete(`${config.apiBaseUrl}/keys/me`, { headers });
  return resp.data as DeleteServicePayload;
}

export async function deleteMessagesData(
  deviceId?: string
): Promise<DeleteServicePayload> {
  const headers = await authHeaders();
  const resp = await axios.delete(`${config.apiBaseUrl}/messages/me`, {
    headers,
    params: deviceId ? { device_id: deviceId } : undefined,
  });
  return resp.data as DeleteServicePayload;
}

export async function deleteAllProfileData(
  deviceId?: string
): Promise<DeleteProfilePayload> {
  const headers = await authHeaders();
  const resp = await axios.delete(`${config.apiBaseUrl}/profile`, {
    headers,
    params: deviceId ? { device_id: deviceId } : undefined,
  });
  return resp.data as DeleteProfilePayload;
}
