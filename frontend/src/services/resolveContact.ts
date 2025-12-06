import axios from "axios";
import config from "../config/config";

export type ResolveContactResponse = {
  userId: string;
  username: string;
  deviceId: string;
};

export async function resolveContact(
  username: string
): Promise<ResolveContactResponse> {
  const resp = await axios.post(`${config.apiBaseUrl}/auth/resolve`, {
    username,
  });
  return resp.data as ResolveContactResponse;
}
