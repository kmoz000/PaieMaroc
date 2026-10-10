import { readFileSync, writeFileSync } from "node:fs";

const version = (process.env.GITHUB_REF_NAME || "").replace(/^v/, "");
if (!/^\d+\.\d+\.\d+$/.test(version)) {
  throw new Error(`Version de release invalide: ${version || "absente"}`);
}

const configPath = new URL("../wails.json", import.meta.url);
const config = JSON.parse(readFileSync(configPath, "utf8"));
config.info = {
  companyName: "PaieMaroc",
  productName: "PaieMaroc",
  productVersion: version,
  copyright: `Copyright ${new Date().getUTCFullYear()} PaieMaroc`,
  comments: "Paie et déclarations sociales pour les équipes RH au Maroc",
};
writeFileSync(configPath, `${JSON.stringify(config, null, 2)}\n`);
