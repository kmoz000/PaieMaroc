# Contrats de fichiers

## CNSS e-BDS

Le moteur transforme le fichier préétabli `A00`–`A03` en déclaration `B00`–`B06`. Chaque enregistrement contient exactement 260 caractères ASCII et se termine par LF. Tous les salariés `A02` doivent produire un `B02`; un salarié absent bloque la génération. Les nouveaux salariés sont émis en `B04` et une ligne `B04` vide reste obligatoire lorsqu’il n’y a aucun entrant.

Cas gérés : `SO`, `DE`, `IT`, `IL`, `AT`, `CS`, `MS`, `MP`, entrant sans immatriculation (`000000000`) et main-d’œuvre occasionnelle (`999999999`). Les contrôles portent sur l’ordre, les plafonds, les jours, les identifiants, les totaux verticaux et horizontaux et les sommes de contrôle.

L’utilisateur choisit un seul fichier source : le préétabli CNSS du mois actif. Les jours, salaires, noms et CIN proviennent directement de la période enregistrée dans SQLite. Un salarié présent dans le préétabli produit un `B02`; un salarié de la période absent du préétabli devient automatiquement un entrant `B04`. Un numéro CNSS vide est émis comme `000000000` et exige donc un nom complet et un CIN valides. La période `AAAAMM` du préétabli doit correspondre exactement au mois actif.

L’index de rapprochement est temporaire et stocké sur disque : le traitement ne charge pas toute la paie en mémoire, même pour un effectif important. Le fichier CSV d’exemple reste un fixture technique du moteur historique; il n’est plus demandé dans le parcours utilisateur.

## Remises bancaires

- `CSV` : UTF-8 BOM, séparateur point-virgule et protection contre l’injection tableur.
- `SIMT500` : enregistrements fixes de 500 caractères et archive ZIP découpée à 99 999 opérations.
- `ISO 20022` : `pain.001.001.03`, proposé comme brouillon à faire confirmer par la banque.

Les variantes contractuelles diffèrent selon la banque. Aucun export bancaire n’est présenté comme universel : validez un fichier d’essai auprès de la banque avant production.
