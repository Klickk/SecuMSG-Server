import axios from "axios";
import config from "../config/config";
import { VerifyResponse } from "../types/types";
import { requireAccessToken } from "../lib/authToken";

export async function verifyAccessToken(): Promise<VerifyResponse> {
  const token = await requireAccessToken();
  const resp = await axios.post(`${config.apiBaseUrl}/auth/verify`, { token });
  return resp.data as VerifyResponse;
}
