import axios from "axios";
import { GenerateIdentityKeypair } from "../crypto-core";
import config from "../config/config";
import { DeviceRegisterResponse } from "../types/types";

const registerDevice = async (
  deviceName: string
): Promise<DeviceRegisterResponse> => {
  const newDevice = GenerateIdentityKeypair();
  const publicBundle = newDevice.PublishPrekeyBundle(5);
  const userId: string = localStorage.getItem("userId")!;
  try {
    return axios
      .post(`${config.apiBaseUrl}/auth/devices/register`, {
        UserID: userId,
        Name: deviceName,
        Platform: navigator.userAgent,
        KeyBundle: {
          IdentityKeyPub: publicBundle.IdentityKey.toString(),
          SignedPrekeyPub: publicBundle.SignedPrekey.toString(),
          SignedPrekeySig: publicBundle.SignedPrekeySig.toString(),
          OneTimePrekeys: publicBundle.OneTimePrekeys.map((x) =>
            x.Public.toString()
          ),
        },
      })
      .then((response) => {
        return response.data as DeviceRegisterResponse;
      });
  } catch (error) {
    console.error("Error registering device:", error);
    throw error;
  }
};

export default registerDevice;
