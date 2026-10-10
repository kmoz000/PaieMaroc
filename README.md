# PaieMaroc

PaieMaroc est une application de bureau libre, en français, destinée aux responsables RH au Maroc. Elle calcule et contrôle la paie, conserve les données localement, prépare les déclarations CNSS e-BDS et génère des remises bancaires traçables.

[Télécharger la dernière version](https://kmoz000.github.io/PaieMaroc/) · [Voir les versions](https://github.com/kmoz000/PaieMaroc/releases) · [Signaler un problème](https://github.com/kmoz000/PaieMaroc/issues)

## Points forts

- Parcours guidé en français, avec validation avant chaque export.
- Montants calculés en centimes entiers, jamais en virgule flottante.
- Base SQLite locale, pagination par curseur et traitements en flux pour des volumes très importants.
- Premier compte administrateur local, rôles séparés et mots de passe Argon2id.
- Fiche salarié unique, historique mensuel, reprise du mois précédent et clôture réversible.
- Configuration multi-pointeuses ZKTeco, avec profil MB10-VL et liaison des identifiants salariés.
- Exports CSV, SIMT500 segmenté et ISO 20022, à confirmer avec la banque destinataire.
- Moteur CNSS e-BDS A00–A03 vers B00–B06, y compris entrants, sorties et cas particuliers.
- Tests unitaires des calculs, contrôles de longueur, totaux et sommes de contrôle.

## Développement

```bash
cd paie-maroc
make test
make dev
make build
```

Prérequis : Go 1.27.2+, Node.js 24+ et Wails 2.15. Les règles légales et formats bancaires sont versionnés ; faites toujours valider le premier fichier par la banque et le premier BDS par la CNSS avant un envoi réel.

## Licence

Distribué sous licence MIT. Voir [LICENSE](LICENSE).
