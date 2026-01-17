import type { ProfileFieldKey } from "../types/profile";

type OwnershipInfo = {
  service: "auth" | "messages";
  description: string;
};

export const ProfileDataOwnership: Record<ProfileFieldKey, OwnershipInfo> = {
  "auth.userId": {
    service: "auth",
    description: "Subject identifier from auth token verification.",
  },
  "auth.sessionId": {
    service: "auth",
    description: "Session identifier issued by the auth service.",
  },
  "auth.tokenDeviceId": {
    service: "auth",
    description: "Device identifier embedded in the access token.",
  },
  "auth.deviceAuthorized": {
    service: "auth",
    description: "Authorization decision for this device from auth.",
  },
  "messages.conversationCount": {
    service: "messages",
    description: "Conversation list owned by the messages service.",
  },
};
