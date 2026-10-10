# Publication et signature

Une version est créée par `.github/workflows/release.yml` lors de l’envoi d’un tag `vX.Y.Z`. Tant que les certificats commerciaux ne sont pas disponibles, les releases sont publiées comme **préversions auto-signées** avec sommes SHA-256 et attestations de provenance GitHub.

## Mode provisoire actuel

- Windows génère sur le runner un certificat Authenticode auto-signé éphémère. L’application, le désinstalleur NSIS et l’installateur sont signés puis vérifiés avant publication.
- macOS produit une application universelle Intel/Apple Silicon avec signature ad hoc, puis signe également le DMG.
- Linux produit un paquet Debian x64.

Ces signatures détectent une modification accidentelle du paquet, mais ne créent pas une chaîne de confiance publique. Microsoft SmartScreen et macOS Gatekeeper peuvent donc afficher un avertissement. Le workflow nomme explicitement les fichiers `SELF-SIGNED`, ajoute `SELF_SIGNED_NOTICE.txt` et marque la release comme préversion.

## Créer une préversion

Vérifiez la branche principale, puis poussez un tag annoté :

```bash
git switch main
git pull --ff-only
git tag -a v0.1.0 -m "PaieMaroc 0.1.0"
git push origin v0.1.0
```

La release n’est publiée que si Windows, macOS et Linux réussissent tous. Ne publiez jamais manuellement un binaire qui n’a pas traversé ce workflow.

## Passage à une signature officielle

Pour supprimer les avertissements système, remplacez le mode provisoire par :

- un certificat Authenticode public pour Windows, idéalement EV ;
- un certificat Apple **Developer ID Application**, le runtime renforcé et la notarisation Apple pour macOS.

La future migration devra rétablir les contrôles `signtool /pa`, `notarytool`, `stapler` et Gatekeeper avant de retirer la mention `SELF-SIGNED` ou le statut de préversion.
