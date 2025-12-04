export type Bytes32 = Uint8Array;
export type Bytes12 = Uint8Array;

export enum SessionRole {
  RoleInitiator = 0,
  RoleResponder = 1,
}

export interface KeyPair {
  Private: Bytes32;
  Public: Bytes32;
}

export interface IdentityKeyPair {
  signingPublic: Uint8Array;
  signingPrivate: Uint8Array;
  dhPrivate: Bytes32;
  dhPublic: Bytes32;
}

export interface OneTimeEntry {
  key: KeyPair;
}

export interface OneTimePrekey {
  ID: string;
  Public: Bytes32;
}

export interface PrekeyBundle {
  IdentityKey: Bytes32;
  IdentitySignatureKey: Uint8Array;
  SignedPrekey: Bytes32;
  SignedPrekeySig: Uint8Array;
  OneTimePrekeys: OneTimePrekey[];
}

export interface HandshakeMessage {
  IdentityKey: Bytes32;
  IdentitySignatureKey: Uint8Array;
  EphemeralKey: Bytes32;
  OneTimePrekeyID?: string;
}

export interface ChainState {
  Key: Bytes32;
  Index: number;
}

export interface SessionState {
  RootKey: Bytes32;
  SendChain: ChainState;
  RecvChain: ChainState;
  RatchetPrivate: Bytes32;
  RatchetPublic: Bytes32;
  RemoteRatchet: Bytes32;
  RemoteIdentity: Bytes32;
  RemoteSignature: Uint8Array;
  PN: number;
  Role: SessionRole;
  PendingPrekey?: string;
  skipped: Map<string, Bytes32>;
}

export class MessageHeader {
  constructor(
    public DHPublic: Bytes32,
    public PN: number,
    public N: number,
    public Nonce: Bytes12
  ) {}

  associatedData(): Uint8Array {
    const buf = new Uint8Array(40);
    buf.set(this.DHPublic, 0);
    const view = new DataView(buf.buffer, buf.byteOffset + 32, 8);
    view.setUint32(0, this.PN, false);
    view.setUint32(4, this.N, false);
    return buf;
  }
}

export interface DeviceState {
  SigningPrivate: string;
  SigningPublic: string;
  DHPrivate: string;
  DHPublic: string;
  SignedPrekey: X25519KeyPairState;
  SignedPrekeySig: string;
  OneTime?: Record<string, X25519KeyPairState>;
}

export interface X25519KeyPairState {
  Private: string;
  Public: string;
}

export interface ChainStateSnapshot {
  Key: string;
  Index: number;
}

export interface SessionStateSnapshot {
  RootKey: string;
  SendChain: ChainStateSnapshot;
  RecvChain: ChainStateSnapshot;
  RatchetPrivate: string;
  RatchetPublic: string;
  RemoteRatchet: string;
  RemoteIdentity: string;
  RemoteSignature: string;
  PN: number;
  Role: SessionRole;
  PendingPrekey?: string;
  Skipped?: Record<string, string>;
}
