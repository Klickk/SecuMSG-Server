export type TokenResponse = {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
};

export type RegisterResponse = {
  userId: string;
  requiresEmailVerification: boolean;
};

export type DeviceRegisterResponse = {
  deviceId: string;
  userId: string;
  name: string;
  platform: string;
};
