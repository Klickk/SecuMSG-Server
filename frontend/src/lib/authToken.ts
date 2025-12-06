import { getItem } from "./storage";

export async function requireAccessToken(): Promise<string> {
  const token = await getItem("accessToken");
  if (!token) {
    throw new Error("Missing access token. Please sign in again.");
  }
  return token;
}

export async function authHeaders(): Promise<Record<string, string>> {
  const token = await requireAccessToken();
  return { Authorization: `Bearer ${token}` };
}
