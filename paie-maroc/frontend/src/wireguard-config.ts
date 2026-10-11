export type ImportedWireGuardConfig = {
  address: string;
  privateKey: string;
  peerPublicKey: string;
  presharedKey: string;
  endpoint: string;
  allowedIPs: string[];
  persistentKeepaliveSeconds: number;
  mtu?: number;
};

export function parseWireGuardConfig(input: string): ImportedWireGuardConfig {
  if (!input.trim()) throw new Error("Collez d’abord la configuration WireGuard.");
  if (input.length > 32_768) throw new Error("La configuration WireGuard est trop volumineuse.");

  let section = "";
  let interfaceSections = 0;
  let peerSections = 0;
  const values: Partial<ImportedWireGuardConfig> = {};

  for (const rawLine of input.replace(/^\uFEFF/, "").split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#") || line.startsWith(";")) continue;
    const sectionMatch = /^\[([^\]]+)]$/.exec(line);
    if (sectionMatch) {
      section = sectionMatch[1].trim().toLowerCase();
      if (section === "interface") interfaceSections += 1;
      if (section === "peer") peerSections += 1;
      continue;
    }
    const separator = line.indexOf("=");
    if (separator < 1) throw new Error("Une ligne de la configuration WireGuard est invalide.");
    const key = line.slice(0, separator).trim().toLowerCase();
    const value = line.slice(separator + 1).trim();
    if (section === "interface") {
      if (key === "privatekey") values.privateKey = value;
      if (key === "address") {
        const addresses = splitList(value);
        if (addresses.length !== 1) throw new Error("PaieMaroc attend une seule adresse dans [Interface] Address.");
        values.address = addresses[0];
      }
      if (key === "mtu") values.mtu = parseInteger(value, "MTU");
      continue;
    }
    if (section === "peer") {
      if (key === "publickey") values.peerPublicKey = value;
      if (key === "presharedkey") values.presharedKey = value;
      if (key === "endpoint") values.endpoint = value;
      if (key === "allowedips") values.allowedIPs = splitList(value);
      if (key === "persistentkeepalive") {
        values.persistentKeepaliveSeconds = parseInteger(value, "PersistentKeepalive");
      }
    }
  }

  if (interfaceSections !== 1 || peerSections !== 1) {
    throw new Error("La configuration doit contenir exactement une section [Interface] et une section [Peer].");
  }
  const required: Array<[keyof ImportedWireGuardConfig, string]> = [
    ["privateKey", "[Interface] PrivateKey"],
    ["address", "[Interface] Address"],
    ["peerPublicKey", "[Peer] PublicKey"],
    ["endpoint", "[Peer] Endpoint"],
    ["allowedIPs", "[Peer] AllowedIPs"],
  ];
  for (const [key, label] of required) {
    if (!values[key] || (Array.isArray(values[key]) && values[key].length === 0)) {
      throw new Error(`${label} manque dans la configuration.`);
    }
  }
  return {
    address: values.address!,
    privateKey: values.privateKey!,
    peerPublicKey: values.peerPublicKey!,
    presharedKey: values.presharedKey || "",
    endpoint: values.endpoint!,
    allowedIPs: values.allowedIPs!,
    persistentKeepaliveSeconds: values.persistentKeepaliveSeconds ?? 25,
    mtu: values.mtu,
  };
}

function splitList(value: string): string[] {
  return value.split(",").map((item) => item.trim()).filter(Boolean);
}

function parseInteger(value: string, label: string): number {
  if (!/^\d+$/.test(value)) throw new Error(`${label} doit être un nombre entier.`);
  return Number(value);
}
