# Signature des versions

Le workflow `release.yml` produit les paquets sur les systèmes natifs. Les secrets ci-dessous activent la signature officielle; en leur absence, les paquets restent disponibles mais sont nommés `UNSIGNED`.

## Windows

- `WINDOWS_CERTIFICATE_BASE64` : certificat Authenticode `.pfx` encodé en Base64.
- `WINDOWS_CERTIFICATE_PASSWORD` : mot de passe du certificat.

## macOS

- `APPLE_CERTIFICATE_BASE64` et `APPLE_CERTIFICATE_PASSWORD` : certificat Developer ID `.p12`.
- `APPLE_SIGNING_IDENTITY` : par exemple `Developer ID Application: Société (TEAMID)`.
- `APPLE_ID`, `APPLE_APP_PASSWORD`, `APPLE_TEAM_ID` : identifiants de notarisation.

Le paquet macOS est signé avec le runtime renforcé, envoyé au service de notarisation Apple, agrafé, puis recompressé. Chaque version reçoit aussi un fichier `SHA256SUMS.txt` et une attestation de provenance GitHub.
