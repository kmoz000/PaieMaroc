# Application PaieMaroc

Application Wails 2 (Go, React et TypeScript). Le domaine est isolé dans `internal/payroll`, `internal/cnss`, `internal/exporter`, `internal/auth`, `internal/attendance` et `internal/store`; `app.go` expose uniquement la frontière bureau.

## Commandes

```bash
make dev       # application avec rechargement à chaud
make test      # tests Go/React, race detector et build TypeScript
make lint      # gofmt, go vet, audit npm, tests et build TypeScript
make build     # paquet natif dans build/bin/
```

Les fichiers CSV sont importés en lots de 5 000 lignes. La base SQLite utilise WAL et les listes sont paginées par curseur. Les exports massifs sont écrits en flux; SIMT500 est découpé à 99 999 opérations par fichier.

La génération e-BDS demande uniquement le fichier préétabli CNSS du mois. PaieMaroc rapproche ensuite ce fichier avec la période active enregistrée, détecte les nouveaux entrants et produit le BDS contrôlé sans second fichier de saisie.

Au premier lancement, l’application exige la création d’un administrateur local. Les salariés sont uniques, tandis que les jours, salaires et avances sont enregistrés par mois. Une nouvelle période peut reprendre la précédente; une clôture peut être rouverte ou supprimée par un administrateur.

Le module de présence configure plusieurs pointeuses ZKTeco selon leur famille (`standalone`, `adms` ou `biotime`). Le MB10-VL est le profil prioritaire. PaieMaroc conserve les identifiants de liaison et les événements utiles, jamais les gabarits de visage ou d’empreinte.
