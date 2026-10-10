# Sources réglementaires et techniques

Sources vérifiées le 10 octobre 2026 :

- [Code général des impôts 2026 — DGI](https://www.finances.gov.ma/Publication/dgi/2025/CGI-2026-FR.pdf) : articles 59 et 73, frais professionnels et barème annuel de l’IR.
- [Décret n° 2-01-2723 — ACAPS](https://www.acaps.ma/sites/default/files/publication_documents/decret_ndeg_2-01-2723.pdf) : allocations familiales, prestations sociales et plafond CNSS de 6 000 MAD.
- [Couverture médicale de base — ACAPS](https://www.acaps.ma/fr/grand-public/sante/couverture-medicale-de-base-amo) : AMO privée de 4,52 % partagée à parts égales, majorée de 1,85 % à la charge de l’employeur.
- [Cahier des charges e-BDS CNSS v1.3](https://fr.scribd.com/document/456993223/CNSS-Regles-de-gestion-emission-et-preetabliV7-pdf) : structures A00–A03/B00–B06, longueur 260, contrôles, entrants et clés. Cette copie publique est ancienne; un fichier pilote doit être validé sur Damancom.
- [Virement de masse — BMCI](https://www.bmci.ma/entreprises/gestion-des-flux/optimiser-tresorerie/virement-de-masse/) : formats bancaires AMB160, SIMT500 et MT101 disponibles au Maroc.
- [Guide e-corpo — Crédit du Maroc](https://www.creditdumaroc.ma/sites/default/files/ecorpo/GuideUtilisateur.pdf) : prise en charge annoncée des fichiers SIMT, AMB160 et SICPA.
- [MB10-VL — ZKTeco](https://www.zkteco.com/en/HybridBiometric4ZM/MB10-VL) : capacités officielles, TCP/IP, ADMS, visage, empreinte, 50 000 transactions et compatibilité BioTime 8.0.
- [Centre SDK — ZKTeco](https://www.zkteco.com/en/SDK) : SDK officiels par famille de matériel; il n’existe pas un pilote public unique couvrant tous les firmwares.

Les taux sociaux combinés utilisés sont versionnés dans `internal/payroll/payroll.go`. Le format exact d’une remise dépend du contrat de télétransmission de la banque : l’application exige donc une validation bancaire avant le premier envoi réel. Une validation réussie dans PaieMaroc ne remplace ni l’accusé de contrôle CNSS ni l’acceptation bancaire.

La compatibilité ZKTeco est sélectionnée par protocole et firmware. Le test intégré prouve la joignabilité TCP, pas la synchronisation métier. L’activation complète d’un adaptateur Standalone, PUSH/ADMS ou ZKBio Time doit être validée sur le modèle et le firmware réels.
