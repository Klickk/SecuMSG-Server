import axios from "axios";
import config from "../config/config";
import { TokenResponse } from "../types/types";

export const Login = async (
  email: string,
  password: string
): Promise<TokenResponse> => {
  return axios
    .post(`${config.apiBaseUrl}/auth/login`, {
      emailOrUsername: email,
      password: password,
    })
    .then((response) => {
      return response.data as TokenResponse;
    })
    .catch((error) => {
      console.error("Login error:", error);
      throw error;
    });
};
