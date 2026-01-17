export type ProfileFieldKey =
  | "auth.userId"
  | "auth.sessionId"
  | "auth.tokenDeviceId"
  | "auth.deviceAuthorized"
  | "messages.conversationCount";

export type AuthProfile = {
  userId: string;
  sessionId?: string;
  tokenDeviceId?: string;
  deviceAuthorized: boolean;
};

export type MessagesProfile = {
  conversationCount: number;
};

export type ProfileSectionState<T> =
  | { status: "loading" }
  | { status: "ready"; data: T }
  | { status: "error"; error: string };

export type ProfileSections = {
  auth: ProfileSectionState<AuthProfile>;
  messages: ProfileSectionState<MessagesProfile>;
};
