import axios from "axios";
import config from "../config/config";
import { authHeaders } from "../lib/authToken";
import { getItem } from "../lib/storage";
import { verifyAccessToken } from "./verify";
import type {
  AuthProfile,
  MessagesProfile,
  ProfileSectionState,
  ProfileSections,
} from "../types/profile";

type ConversationsResponse = {
  conversations: string[];
};

const normalizeError = (err: unknown, fallback: string): string => {
  if (err instanceof Error && err.message) return err.message;
  return fallback;
};

const fetchAuthProfile = async (): Promise<AuthProfile> => {
  const verification = await verifyAccessToken();
  if (!verification.valid || !verification.userId) {
    throw new Error("Access token verification failed.");
  }
  return {
    userId: verification.userId,
    sessionId: verification.sessionId,
    tokenDeviceId: verification.tokenDeviceId,
    deviceAuthorized: verification.deviceAuthorized,
  };
};

const fetchMessagesProfile = async (): Promise<MessagesProfile> => {
  const deviceId = await getItem("deviceId");
  if (!deviceId) {
    throw new Error("Missing device ID. Register a device first.");
  }
  const headers = await authHeaders();
  const resp = await axios.get<ConversationsResponse>(
    `${config.apiBaseUrl}/messages/conversations`,
    {
      headers,
      params: { device_id: deviceId },
    }
  );
  return {
    conversationCount: resp.data.conversations.length,
  };
};

const settle = <T>(
  result: PromiseSettledResult<T>,
  fallback: string
): ProfileSectionState<T> => {
  if (result.status === "fulfilled") {
    return { status: "ready", data: result.value };
  }
  return {
    status: "error",
    error: normalizeError(result.reason, fallback),
  };
};

export const loadProfileSections = async (): Promise<ProfileSections> => {
  const [authResult, messagesResult] = await Promise.allSettled([
    fetchAuthProfile(),
    fetchMessagesProfile(),
  ]);

  return {
    auth: settle(authResult, "Failed to load auth profile."),
    messages: settle(messagesResult, "Failed to load messages profile."),
  };
};
