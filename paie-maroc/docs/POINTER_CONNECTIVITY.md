# Connectivité des pointeuses

PaieMaroc réserve cette couche réseau aux pointeuses configurées. Elle ne fournit ni proxy général ni accès au réseau de l’ordinateur.

## Choisir le bon chemin

- **Standalone TCP/IP** : PaieMaroc ouvre une connexion sortante vers la pointeuse, généralement sur le port 4370. Aucune écoute entrante ou règle pare-feu n’est nécessaire.
- **PUSH / ADMS** : la pointeuse appelle le collecteur PaieMaroc sur le port 8088 par défaut. Activez le collecteur uniquement après avoir confirmé l’autorisation du pare-feu. Configurez aussi le numéro de série exact de chaque terminal.
- **WireGuard privé** : une passerelle du site distant doit être un pair WireGuard et router le sous-réseau des pointeuses. La pointeuse elle-même n’exécute pas WireGuard.

## Sécurité du collecteur

Le collecteur refuse les sources hors des CIDR autorisés, les numéros de série inconnus, les terminaux désactivés et les corps supérieurs à 32 MiB. Les en-têtes et durées HTTP sont bornés. Les événements sont importés par lots de 1 000 afin que la mémoire reste stable.

Sur Windows, autorisez PaieMaroc uniquement sur les réseaux privés lorsque le système le demande. Sur macOS, acceptez l’accès au réseau local et les connexions entrantes pour PaieMaroc. Sur Linux, ouvrez le port TCP choisi uniquement depuis le sous-réseau des pointeuses. Ne désactivez jamais le pare-feu global.

## WireGuard en espace utilisateur

Le tunnel utilise `wireguard-go` et une pile TCP/IP interne. Il ne crée pas d’interface noyau, ne modifie pas les routes ou le DNS du système et ne nécessite pas les outils `wg`/`wg-quick`.

1. Importez le fichier `.conf`, ou placez `PrivateKey` de `[Interface]` dans le champ privé et `PublicKey` de `[Peer]` dans le champ public de la passerelle. Ne les inversez pas.
2. Si PaieMaroc génère une nouvelle identité, copiez uniquement sa clé publique sur la passerelle distante.
3. Déclarez cette clé côté serveur avec l’adresse PaieMaroc en `/32`, puis saisissez l’adresse publique de la passerelle.
4. Limitez `AllowedIPs` aux réseaux privés des pointeuses. Les routes `/0` sont refusées.
5. Utilisez un maintien NAT de 25 secondes lorsque PaieMaroc se trouve derrière NAT.

La clé privée reste dans le dossier de configuration propre à l’utilisateur et n’est jamais renvoyée à l’interface après sauvegarde. Le fichier reçoit les permissions `0600` sur macOS/Linux et hérite des ACL du profil utilisateur sous Windows.

## Diagnostic du tunnel

Le statut « initialisé » confirme seulement que le moteur userspace a démarré. « Connecté » exige un handshake WireGuard réel. Utilisez **Tester le ping** avec l’adresse IP d’une pointeuse : PaieMaroc envoie l’ICMP dans son netstack interne et affiche l’endpoint résolu, le dernier handshake ainsi que les compteurs RX/TX.

- Aucun handshake et `RX 0` : vérifiez les deux clés, le pair `/32` côté serveur, l’écoute UDP et la redirection du port 51820.
- Handshake établi mais ping sans réponse : vérifiez le routage de la passerelle vers la pointeuse, son pare-feu et son adresse.
- Le champ `DNS` d’un fichier WireGuard est ignoré : PaieMaroc utilise le DNS du système uniquement pour résoudre l’endpoint public.
