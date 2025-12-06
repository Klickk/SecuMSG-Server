export type TokenResponse = {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
};

export type RegisterResponse = {
  userId: string;
  requiresEmailVerification: boolean;
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
};

export type VerifyResponse = {
  valid: boolean;
  userId?: string;
  sessionId?: string;
  tokenDeviceId?: string;
  deviceAuthorized: boolean;
};

export type DeviceRegisterResponse = {
  deviceId: string;
  userId: string;
  name: string;
  platform: string;
};
