import axios from "axios";
import config from "../config/config";
import { RegisterResponse } from "../types/types";

export const Register = async (
  name: string,
  email: string,
  password: string
): Promise<RegisterResponse> => {
  return axios
    .post(`${config.apiBaseUrl}/auth/register`, {
      email: email,
      username: name,
      password: password,
    })
    .then((response) => {
      return response.data as RegisterResponse;
    })
    .catch((error) => {
      console.error("Registration error:", error);
      throw error;
    });
};
