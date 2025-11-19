import axios from "axios";
import config from "../config/config";

export const Register = async (
  name: string,
  email: string,
  password: string
): Promise<boolean> => {
  return axios
    .post(`${config.apiBaseUrl}/auth/register`, {
      email: email,
      username: name,
      password: password,
    })
    .then((response) => {
      return response.status === 200;
    })
    .catch((error) => {
      console.error("Registration error:", error);
      return false;
    });
};
