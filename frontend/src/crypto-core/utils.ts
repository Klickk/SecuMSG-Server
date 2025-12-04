const textEncoder = new TextEncoder();

export function copyBytes(data: Uint8Array): Uint8Array {
  return new Uint8Array(data);
}

export function concatBytes(...arrays: Uint8Array[]): Uint8Array {
  const total = arrays.reduce((acc, arr) => acc + arr.length, 0);
  const out = new Uint8Array(total);
  let offset = 0;
  for (const arr of arrays) {
    out.set(arr, offset);
    offset += arr.length;
  }
  return out;
}

export function equalsBytes(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) {
    return false;
  }
  for (let i = 0; i < a.length; i += 1) {
    if (a[i] !== b[i]) {
      return false;
    }
  }
  return true;
}

export function zeroBytes(length: number): Uint8Array {
  return new Uint8Array(length);
}

export function toBase64(data: Uint8Array): string {
  if (typeof Buffer !== "undefined") {
    return Buffer.from(data).toString("base64");
  }
  let binary = "";
  data.forEach((b) => {
    binary += String.fromCharCode(b);
  });
  return btoa(binary);
}

export function fromBase64(input: string): Uint8Array {
  if (typeof Buffer !== "undefined") {
    return new Uint8Array(Buffer.from(input, "base64"));
  }
  const binary = atob(input);
  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    out[i] = binary.charCodeAt(i);
  }
  return out;
}

export function utf8(input: string): Uint8Array {
  return textEncoder.encode(input);
}

export function ensureLength(data: Uint8Array, size: number, name: string): Uint8Array {
  if (data.length !== size) {
    throw new Error(`unexpected length for ${name}: ${data.length}, want ${size}`);
  }
  return new Uint8Array(data);
}
