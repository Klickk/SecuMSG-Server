import config from "../config/config";
import { MessagingClient } from "../lib/messagingClient";
import { DeviceRegisterResponse } from "../types/types";
import { getItem } from "../lib/storage";
import { requireAccessToken } from "../lib/authToken";
import { getKeyManager } from "../lib/keyManagerInstance";
import { SecureStore } from "../lib/secureStore";

export type DeviceRegistrationResult = {
  device: DeviceRegisterResponse;
  client: MessagingClient;
};

const registerDevice = async (
  deviceName: string
): Promise<DeviceRegistrationResult> => {
  const userId = await getItem("userId");
  if (!userId) {
    throw new Error("User ID missing. Please register or log in first.");
  }
  // Ensure we have an access token so downstream requests include Authorization.
  await requireAccessToken();

  const safeName = deviceName.trim() || navigator.userAgent.slice(0, 32) || "Device";

  const manager = getKeyManager(userId);
  if (!manager.isUnlocked()) {
    throw new Error("Device vault is locked.");
  }
  const secureStore = new SecureStore(manager, userId);

  const { client, device } = await MessagingClient.registerDevice(
    userId,
    safeName,
    config.apiBaseUrl,
    secureStore
  );

  return { device: device as DeviceRegisterResponse, client };
};

export default registerDevice;
