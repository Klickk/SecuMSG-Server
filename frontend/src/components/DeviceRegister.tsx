import React, { useState } from "react";
import registerDevice from "../services/registerDevice";
import { DeviceRegisterResponse } from "../types/types";

export type DeviceRegisterFormValues = {
  name: string;
};

type DeviceRegisterFormProps = {
  onSubmit: (values: DeviceRegisterFormValues) => Promise<void> | void;
  isLoading?: boolean;
};

export const DeviceRegisterForm: React.FC<DeviceRegisterFormProps> = ({
  onSubmit,
  isLoading = false,
}) => {
  const [values, setValues] = useState<DeviceRegisterFormValues>({
    name: "",
  });

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>): void => {
    const { value } = e.target;
    setValues({ name: value });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!values.name.trim()) return;
    registerDevice(values.name).then((x) => {
      console.log("Device registered successfully");
      localStorage.setItem("deviceId", x.deviceId);
      localStorage.setItem("deviceName", x.name);
      localStorage.setItem("devicePlatform", x.platform);
    });
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex items-center justify-center px-4">
      <div className="w-full max-w-md">
        <div className="bg-slate-900/70 border border-slate-800 rounded-2xl shadow-xl p-8 backdrop-blur">
          <div className="mb-6">
            <h1 className="text-2xl font-semibold">Register a new device</h1>
            <p className="text-sm text-slate-400 mt-1">
              Give this device a name so you can recognize it in your account
              and manage its keys.
            </p>
          </div>

          <form className="space-y-4" onSubmit={handleSubmit}>
            <div className="space-y-1">
              <label
                htmlFor="deviceName"
                className="block text-sm font-medium text-slate-200"
              >
                Device name
              </label>
              <input
                id="deviceName"
                name="deviceName"
                type="text"
                required
                value={values.name}
                onChange={handleChange}
                className="w-full rounded-lg bg-slate-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500 focus:border-sky-500"
                placeholder="Ivan’s laptop, Pixel phone, etc."
              />
            </div>

            <button
              type="submit"
              disabled={isLoading || !values.name.trim()}
              className="w-full rounded-lg bg-sky-500 hover:bg-sky-400 disabled:opacity-60 disabled:cursor-not-allowed transition px-3 py-2 text-sm font-medium text-slate-900"
            >
              {isLoading ? "Registering device..." : "Register device"}
            </button>
          </form>
        </div>

        <p className="mt-4 text-center text-xs text-slate-500">
          A unique keypair will be generated locally for this device. 🔐
        </p>
      </div>
    </div>
  );
};
