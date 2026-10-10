# Contrats de fichiers

## CNSS e-BDS

Le moteur transforme le fichier préétabli `A00`–`A03` en déclaration `B00`–`B06`. Chaque enregistrement contient exactement 260 caractères ASCII et se termine par LF. Tous les salariés `A02` doivent produire un `B02`; un salarié absent bloque la génération. Les nouveaux salariés sont émis en `B04` et une ligne `B04` vide reste obligatoire lorsqu’il n’y a aucun entrant.

Cas gérés : `SO`, `DE`, `IT`, `IL`, `AT`, `CS`, `MS`, `MP`, entrant sans immatriculation (`000000000`) et main-d’œuvre occasionnelle (`999999999`). Les contrôles portent sur l’ordre, les plafonds, les jours, les identifiants, les totaux verticaux et horizontaux et les sommes de contrôle.

Le fichier de saisie attendu est illustré par `examples/declarations-cnss.csv`. `entrant=oui` produit un `B04`; un numéro vide devient `000000000` et exige le nom complet et le CIN. `entrant=non` doit correspondre à un salarié du préétabli. Le champ `salaire_plafonne_mad` est facultatif et vaut par défaut le minimum entre le salaire réel et 6 000 MAD.

## Remises bancaires

- `CSV` : UTF-8 BOM, séparateur point-virgule et protection contre l’injection tableur.
- `SIMT500` : enregistrements fixes de 500 caractères et archive ZIP découpée à 99 999 opérations.
- `ISO 20022` : `pain.001.001.03`, proposé comme brouillon à faire confirmer par la banque.

Les variantes contractuelles diffèrent selon la banque. Aucun export bancaire n’est présenté comme universel : validez un fichier d’essai auprès de la banque avant production.
