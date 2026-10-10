# Application PaieMaroc

Application Wails 2 (Go, React et TypeScript). Le domaine est isolé dans `internal/payroll`, `internal/cnss`, `internal/exporter` et `internal/store`; `app.go` expose uniquement la frontière bureau.

## Commandes

```bash
make dev       # application avec rechargement à chaud
make test      # tests Go, race detector et build TypeScript
make lint      # gofmt, go vet et vérification TypeScript
make build     # paquet natif dans build/bin/
```

Les fichiers CSV sont importés en lots de 5 000 lignes. La base SQLite utilise WAL et les listes sont paginées par curseur. Les exports massifs sont écrits en flux; SIMT500 est découpé à 99 999 opérations par fichier.
