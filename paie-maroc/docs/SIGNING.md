# Publication et signature

Une version publique est créée uniquement par `.github/workflows/release.yml` lors de l’envoi d’un tag `vX.Y.Z`. Le workflow est volontairement bloquant : aucun exécutable Windows ou macOS non signé ne peut apparaître dans GitHub Releases.

## Certificats requis

Ajoutez ces secrets dans **Settings → Secrets and variables → Actions** du dépôt.

### Windows

- `WINDOWS_CERTIFICATE_BASE64` : certificat de signature de code Authenticode `.pfx`, encodé avec `base64 -i certificat.pfx | pbcopy` sur macOS.
- `WINDOWS_CERTIFICATE_PASSWORD` : mot de passe du `.pfx`.

Le workflow signe et horodate trois éléments : l’application, le désinstalleur NSIS et l’installateur. Il les vérifie ensuite avec la politique Authenticode Windows. Utilisez un certificat délivré à l’éditeur PaieMaroc ; un certificat EV aide aussi à établir plus rapidement la réputation Microsoft SmartScreen.

### macOS

- `APPLE_CERTIFICATE_BASE64` et `APPLE_CERTIFICATE_PASSWORD` : certificat **Developer ID Application** exporté en `.p12`.
- `APPLE_SIGNING_IDENTITY` : `Developer ID Application: Nom (TEAMID)`.
- `APPLE_ID`, `APPLE_APP_PASSWORD`, `APPLE_TEAM_ID` : compte Apple, mot de passe spécifique à l’app et Team ID.

L’application universelle Intel/Apple Silicon reçoit le runtime renforcé. Le DMG avec raccourci vers Applications est envoyé au service notarial Apple, agrafé, puis contrôlé par `codesign`, `notarytool`, `stapler` et Gatekeeper (`spctl`).

## Créer une version

Vérifiez d’abord la branche principale, puis poussez un tag signé :

```bash
git switch main
git pull --ff-only
git tag -s v1.0.0 -m "PaieMaroc 1.0.0"
git push origin v1.0.0
```

La release n’est publiée que si Windows, macOS et Linux réussissent tous. Elle contient l’installateur Windows, le DMG notarisé, le paquet Debian, `SHA256SUMS.txt` et les attestations de provenance GitHub. Ne recréez jamais manuellement une release officielle avec des binaires locaux.
