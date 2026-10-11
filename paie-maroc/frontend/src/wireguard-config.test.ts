import { describe, expect, it } from "vitest";
import { parseWireGuardConfig } from "./wireguard-config";

describe("WireGuard configuration import", () => {
  it("maps Interface private key and Peer public key without reversing them", () => {
    const parsed = parseWireGuardConfig(`[Interface]
PrivateKey = interface-private
Address = 10.255.255.36/32
DNS = 10.255.255.33, 1.1.1.1

[Peer]
PublicKey = gateway-public
Endpoint = wg.wantastic.app:51820
AllowedIPs = 10.255.255.32/27
PersistentKeepalive = 25`);

    expect(parsed).toMatchObject({
      privateKey: "interface-private",
      peerPublicKey: "gateway-public",
      address: "10.255.255.36/32",
      endpoint: "wg.wantastic.app:51820",
      allowedIPs: ["10.255.255.32/27"],
      persistentKeepaliveSeconds: 25,
    });
  });

  it("rejects ambiguous multiple-peer configurations", () => {
    expect(() => parseWireGuardConfig(`[Interface]
PrivateKey=x
Address=10.0.0.2/32
[Peer]
PublicKey=y
Endpoint=one.example:51820
AllowedIPs=10.0.0.0/24
[Peer]
PublicKey=z`)).toThrow(/exactement une section \[Interface] et une section \[Peer]/);
  });
});
