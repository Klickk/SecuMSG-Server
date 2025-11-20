import config from "../config/config";
import { MessagingClient } from "../lib/messagingClient";
import { DeviceRegisterResponse } from "../types/types";

export type DeviceRegistrationResult = {
  device: DeviceRegisterResponse;
  client: MessagingClient;
};

const registerDevice = async (
  deviceName: string
): Promise<DeviceRegistrationResult> => {
  const userId = localStorage.getItem("userId");
  if (!userId) {
    throw new Error("User ID missing. Please register or log in first.");
  }

  const { client, device } = await MessagingClient.registerDevice(
    userId,
    deviceName,
    config.apiBaseUrl
  );

  return { device: device as DeviceRegisterResponse, client };
};

export default registerDevice;
