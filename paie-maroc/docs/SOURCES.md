# Sources réglementaires et techniques

Sources vérifiées le 10 octobre 2026 :

- [Code général des impôts 2026 — DGI](https://www.finances.gov.ma/Publication/dgi/2025/CGI-2026-FR.pdf) : articles 59 et 73, frais professionnels et barème annuel de l’IR.
- [Décret n° 2-01-2723 — ACAPS](https://www.acaps.ma/sites/default/files/publication_documents/decret_ndeg_2-01-2723.pdf) : allocations familiales, prestations sociales et plafond CNSS de 6 000 MAD.
- [Couverture médicale de base — ACAPS](https://www.acaps.ma/fr/grand-public/sante/couverture-medicale-de-base-amo) : AMO privée de 4,52 % partagée à parts égales, majorée de 1,85 % à la charge de l’employeur.
- [Cahier des charges e-BDS CNSS v1.3](https://fr.scribd.com/document/456993223/CNSS-Regles-de-gestion-emission-et-preetabliV7-pdf) : structures A00–A03/B00–B06, longueur 260, contrôles, entrants et clés. Cette copie publique est ancienne; un fichier pilote doit être validé sur Damancom.
- [Virement de masse — BMCI](https://www.bmci.ma/entreprises/gestion-des-flux/optimiser-tresorerie/virement-de-masse/) : formats bancaires AMB160, SIMT500 et MT101 disponibles au Maroc.
- [Guide e-corpo — Crédit du Maroc](https://www.creditdumaroc.ma/sites/default/files/ecorpo/GuideUtilisateur.pdf) : prise en charge annoncée des fichiers SIMT, AMB160 et SICPA.
- [Code du travail — Ministère de la Justice](https://adala.justice.gov.ma/api/uploads/2024/04/30/code%20du%20travail-1714463246806.pdf) : articles 184 et 196–201 pour la durée normale et les heures supplémentaires; articles 217–227 pour les jours fériés payés. L’article 224 prévoit, lorsque le travail férié est autorisé, une indemnité supplémentaire égale au salaire correspondant, sauf repos compensateur convenu.
- [MB10-VL — ZKTeco](https://zkteco.com/en/Standalone_Device/MB10-VL) : capacités officielles, TCP/IP, ADMS, visage, empreinte, carte et 50 000 transactions (150 000 en option).
- [ZKBio Time — ZKTeco](https://www.zkteco.com/en/ZKBioTime/ZKBioTime) : logiciel serveur officiel pour les déploiements PUSH/ADMS; son intégration nécessite l’API et la version effectivement licenciées sur le site client.
- [Centre SDK — ZKTeco](https://www.zkteco.com/en/SDK) : SDK officiels par famille de matériel; il n’existe pas un pilote public unique couvrant tous les firmwares.
- [go-zkteco — Fariz Fadian](https://github.com/farizfadian/go-zkteco) : base MIT de l’adaptateur TCP Standalone en lecture seule, intégrée et durcie dans `internal/attendance/device.go`; attribution complète dans `THIRD_PARTY_NOTICES.md`.
- [ZK protocol — community reference](https://github.com/adrobinoga/zk-protocol/blob/master/protocol.md) : cadrage TCP, commandes et encodage temporel recoupés pour les tests du décodeur.

Les taux sociaux combinés utilisés sont versionnés dans `internal/payroll/payroll.go`. Le format exact d’une remise dépend du contrat de télétransmission de la banque : l’application exige donc une validation bancaire avant le premier envoi réel. Une validation réussie dans PaieMaroc ne remplace ni l’accusé de contrôle CNSS ni l’acceptation bancaire.

La compatibilité ZKTeco est sélectionnée par protocole et firmware. Les tests automatisés couvrent le cadrage TCP et les journaux de 8, 16 et 40 octets, mais ne remplacent pas une recette sur la pointeuse et le firmware réels. L’adaptateur Standalone ne lit que les pointages; il ne modifie ni utilisateurs ni gabarits biométriques. PUSH/ADMS et ZKBio Time nécessitent toujours leur adaptateur serveur.

Le calendrier embarqué matérialise automatiquement les fêtes nationales à date fixe. Les fêtes religieuses ne sont jamais extrapolées : l’administrateur les confirme après publication officielle. Les majorations produisent une proposition contrôlable et ne modifient pas automatiquement le brut mensuel.
