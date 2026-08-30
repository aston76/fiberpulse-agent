# FiberPulse — passation complète et reprise du projet

Dernière vérification opérationnelle : **31 août 2026, fuseau Asia/Manila**.

Ce fichier est la source de reprise principale. Une nouvelle conversation doit commencer par le lire, puis vérifier les états GitHub, DNS, o2switch et Store qui peuvent avoir changé depuis cette date.

## 1. Objectif du produit

FiberPulse est une application locale Windows 10/11 et macOS 13+ qui :

- mesure la connexion Internet avec M-Lab NDT7 ;
- compare les résultats avec l’offre souscrite auprès du fournisseur d’accès ;
- conserve un historique local ;
- contextualise les mesures (Wi-Fi/Ethernet, VPN, matériel, routeur supplémentaire, lieu approximatif) ;
- détecte les écarts persistants et incidents ;
- génère des rapports PDF/CSV et des brouillons de réclamation ;
- peut partager séparément des mesures techniques anonymisées avec l’observatoire public ;
- prévoit des mises à jour sécurisées Windows/macOS ;
- possède un site public, un dépôt méthodologique et une plateforme backend séparée.

Le trafic volumineux du speed test doit rester **direct entre le client et M-Lab**. Il ne doit jamais transiter par le backend FiberPulse.

## 2. Racine et dépôts

Racine locale :

```text
/Volumes/Dev-SSD/Projects/Mes Pojets IA/FiberPulse
```

Sous-projets :

| Dossier | Rôle | Dépôt GitHub | Visibilité au 30/08/2026 |
|---|---|---|---|
| `fiberpulse-agent` | Application desktop, dashboard local, mesure, rapports, updater, packaging | `https://github.com/aston76/fiberpulse-agent` | Public, nécessaire aux téléchargements et mises à jour |
| `fiberpulse-site` | Site produit `testspeednow.com` et déploiement o2switch | `https://github.com/aston76/fiberpulse-site` | Privé |
| `fiberpulse-platform` | API/plateforme et catalogues d’offres | `https://github.com/aston76/fiberpulse-platform` | Privé |
| `fiberpulse-methodology` | Méthodologie et contrats de mesure | `https://github.com/aston76/fiberpulse-methodology` | Privé |

Commits vérifiés au moment de cette passation :

```text
fiberpulse-agent       30f6cd0 Checkout source before provisional release publish
fiberpulse-site        b806504 Refresh static release hydration cache
fiberpulse-platform    84aa882 Expand major ISP plan coverage
fiberpulse-methodology ad89994 Document country-specific diagnostic guidance
```

Le dossier non suivi `fiberpulse-agent/output/` contient des PDF générés. Il ne faut pas le committer par défaut.

## 3. Architecture de l’application

Stack principale :

- Go pour le cœur, l’agent, la mesure, SQLite, le serveur local et l’updater ;
- React/TypeScript pour le dashboard local embarqué ;
- application macOS universelle `arm64 + x86_64` ;
- Windows x64 avec édition Inno Setup directe et édition MSIX Store ;
- stockage local-first ;
- serveur d’interface uniquement sur loopback ;
- aucun runtime Node, Python ou WebView requis dans l’application macOS distribuée.

Dossiers importants dans `fiberpulse-agent` :

```text
cmd/fiberpulse/              point d’entrée de l’application
cmd/fiberpulse-updater/      helper de remplacement et rollback
cmd/release-sign/            signature Ed25519 des manifests de release
dashboard/src/               interface locale React
internal/app/                orchestration, scheduler et updates
internal/measurement/        fournisseurs de mesure, dont M-Lab NDT7
internal/localapi/           API locale et assets embarqués
internal/reporting/          PDF, CSV et données de rapport
internal/complaint/          préparation des réclamations
internal/observatory/        partage public optionnel et minimal
packaging/macos/             construction de l’application universelle
packaging/windows/           EXE, Inno Setup et MSIX
docs/UPDATE-SECURITY.md      contrat de sécurité des mises à jour
docs/WINDOWS-DISTRIBUTION.md distribution Windows
docs/MACOS-DISTRIBUTION.md   distribution macOS
.github/workflows/ci.yml     CI générale
.github/workflows/release.yml signature, notarisation et releases
```

## 4. Mesures, consentements et limites

- M-Lab NDT7 est le fournisseur de mesure réel par défaut.
- Le consentement M-Lab est obligatoire avant le premier test.
- Le partage vers l’observatoire FiberPulse est un second consentement, indépendant.
- Les données M-Lab peuvent inclure l’adresse IP publique selon la politique M-Lab.
- Pour le MVP, le client respecte au maximum quatre tests automatiques M-Lab par appareil et par période de 24 heures, avec planification aléatoire.
- Un test manuel peut être refusé quand le quota/cooldown est actif.
- Le mode de développement déterministe utilise `FIBERPULSE_DEV_FAKE=1`.
- Avant un test, l’interface doit avertir si un VPN est détecté et recommander Ethernet ; en Wi-Fi, elle recommande de se rapprocher du routeur.
- Une mesure isolée ne prouve pas la responsabilité du fournisseur. L’analyse doit tenir compte de répétitions, du support de connexion, du plan souscrit et du niveau de confiance.

## 5. Mécanisme réel de mise à jour

### 5.1 Ce qui existe dans le code

L’auto-update est déjà implémenté dans :

```text
fiberpulse-agent/internal/app/update.go
fiberpulse-agent/internal/update/
fiberpulse-agent/cmd/fiberpulse-updater/
fiberpulse-agent/cmd/fiberpulse/update_config.go
fiberpulse-agent/cmd/release-sign/
```

Comportement :

1. Une release publique embarque au build l’URL HTTPS du feed et la clé publique Ed25519.
2. L’application effectue un premier contrôle environ **45 à 105 secondes** après son lancement.
3. Elle recommence environ toutes les **6 heures**, avec une variation aléatoire de ±30 minutes.
4. Le bouton/menu `Update` permet aussi un contrôle manuel immédiat.
5. Si un test de débit est en cours, l’update automatique est différé de 30 minutes afin de ne pas fausser la mesure.
6. Le manifeste signé annonce version, canal, séquence monotone, SHA-256, taille, URL, version minimale et expiration.
7. L’application télécharge dans un dossier de staging privé.
8. Le helper vérifie le manifeste, le hash, la taille, la signature native de plateforme et l’anti-rollback.
9. L’application s’arrête proprement ; le helper remplace le binaire ou le bundle complet.
10. La nouvelle version doit produire un reçu de santé lié à son PID.
11. Si le démarrage échoue, l’ancienne version est restaurée et relancée.
12. Une garde de 24 heures empêche une release cassée de provoquer une boucle d’installation/rollback.

Les builds de développement sans URL de feed ni clé publique affichent les mises à jour comme désactivées. Ce comportement est volontaire.

### 5.2 Windows Microsoft Store

L’édition MSIX Store :

- utilise l’identité Partner Center `SEOWEBAPP.FiberPulse` ;
- est signée par Microsoft après certification ;
- est mise à jour par Microsoft Store ;
- ne contient volontairement pas le helper d’auto-update autonome.

Paquet Store préparé :

```text
FiberPulse-0.1.0.0-windows-x64.msix
```

Product ID précédemment enregistré dans Partner Center :

```text
9N3XLBSX2MPL
```

Cet identifiant et l’état de soumission doivent être relus dans Partner Center avant toute nouvelle affirmation publique.

### 5.3 Windows téléchargé depuis le site

L’édition `setup.exe` directe contient `fiberpulse-updater.exe`. L’auto-update sécurisé fonctionnera après publication d’une release complète avec :

- signature Authenticode SHA-256 ;
- timestamp RFC 3161 ;
- manifest Ed25519 signé ;
- feed et clé publique injectés au build.

Le pipeline actuel refuse correctement de présenter un build Windows direct non signé comme release publique.

### 5.4 macOS sans abonnement Apple — décision actuelle

Décision produit au 30/08/2026 : **ne pas payer immédiatement l’Apple Developer Program**. Publier éventuellement le ZIP macOS directement sur le site avec une courte documentation d’ouverture, puis attendre avant le Mac App Store et la notarisation.

État technique vérifié de l’application locale :

```text
Bundle ID: dev.fiberpulse.agent
Architecture: universelle Apple Silicon + Intel
Signature: ad hoc
TeamIdentifier: absent
codesign: structure valide
Gatekeeper: rejetée comme développeur non identifié
```

Conséquences :

- l’utilisateur doit télécharger le ZIP, déplacer `FiberPulse.app` dans Applications, tenter de l’ouvrir puis choisir **Réglages Système > Confidentialité et sécurité > Ouvrir quand même** ;
- ne jamais demander de désactiver Gatekeeper globalement ;
- ne pas fournir une commande `xattr` globale comme parcours normal ;
- tant que le bundle n’est pas signé Developer ID et notarialisé, les mises à jour Mac doivent rester **manuelles par remplacement du bundle** ;
- l’auto-update Mac sécurisé existe déjà dans le code mais doit rester bloqué pour les releases publiques non notarialisées, car remplacer silencieusement un bundle ad hoc supprimerait une protection native importante.

Procédure manuelle Mac provisoire :

1. Quitter FiberPulse avec son bouton `Quit`.
2. Télécharger la nouvelle archive depuis `testspeednow.com` ou la release GitHub officielle.
3. Décompresser `FiberPulse.app`.
4. Remplacer l’ancienne application dans `/Applications`.
5. L’historique et le profil local restent dans le dossier de données utilisateur, séparé du bundle.
6. Relancer et vérifier la version dans l’interface.

Quand l’abonnement Apple sera acheté, renseigner les secrets de `release.yml`, signer avec Developer ID, notariser avec `notarytool`, stapler le ticket puis réactiver le feed Mac. Le pipeline est déjà conçu pour remplacer le bundle `.app` complet et effectuer un rollback.

### 5.5 État actuel des releases

La release publique `v0.1.0` est publiée :

```text
https://github.com/aston76/fiberpulse-agent/releases/tag/v0.1.0
```

Elle contient le ZIP macOS universel, la notice d’installation et `SHA256SUMS`. L’archive publique téléchargée depuis GitHub a été revérifiée : hashes valides, structure `codesign` valide, signature ad hoc sans TeamIdentifier, URL de partage de production présente, feed d’auto-update absent, démarrage et arrêt complet réussis. Gatekeeper la rejette normalement comme développeur non identifié ; le site et la notice le disent explicitement.

Le workflow propose désormais deux modes séparés :

- `macos-provisional`, pour publier uniquement le ZIP Mac manuel non notarialisé ;
- `full-signed`, réservé aux artefacts Windows Authenticode et macOS Developer ID/notarisés.

Windows n’est pas proposé en téléchargement direct non signé. La soumission MSIX du produit `9N3XLBSX2MPL` est réellement à l’étape **3/4 — en cours de certification** dans Partner Center. Attendre la décision Microsoft ; ne pas contourner ce contrôle avec un installateur public non signé.

## 6. Site public et déploiement

Domaine final :

```text
https://testspeednow.com
```

Hébergement préparé par Hermes/o2switch :

```text
cPanel host: https://polski.o2switch.net:2083
cPanel user: sc2fley6939
docroot: /home/sc2fley6939/testspeednow
serveur web attendu: 109.234.160.138
```

Ne jamais copier les tokens cPanel dans ce fichier. Le pointeur local actuel est :

```text
~/.hermes/local-secrets/vtc-marie-o2switch.txt
```

Le dépôt privé `fiberpulse-site` contient :

```text
.github/workflows/deploy-o2switch.yml
deploy/o2switch/.htaccess
deploy/o2switch/api/release/index.php
deploy/o2switch/api/observatory/index.php
scripts/prepare-o2switch-export.mjs
```

Secrets GitHub configurés dans le dépôt privé, sans valeur dans Git :

```text
CPANEL_HOST
CPANEL_USER
CPANEL_TOKEN
```

Dernier déploiement GitHub Actions vérifié avant cette mise à jour documentaire :

```text
https://github.com/aston76/fiberpulse-site/actions/runs/33328210855
```

Chaque push sur `fiberpulse-site/main` lance lint, build, export o2switch, upload ciblé dans le docroot et vérification du virtual host.

Le remote `sites` reste configuré pour l’ancienne publication Sites :

```text
https://fiberpulse.aston76.chatgpt.site
```

Il ne doit plus être confondu avec le domaine final.

### État DNS/HTTPS à la dernière vérification

État vérifié le 31/08/2026 :

```text
NS publics: ns1.o2switch.net, ns2.o2switch.net
A/virtual host: 109.234.160.138
HTTPS public: valide sans -k
site FiberPulse public: servi par o2switch
```

Pour toute reprise future, revérifier cet état avec :

```sh
dig +short NS testspeednow.com
dig +short A testspeednow.com
dig +short A www.testspeednow.com
curl -I https://testspeednow.com/
```

L’A doit être `109.234.160.138` et `curl` doit réussir sans `-k`.

## 7. Téléchargements affichés sur le site

Le composant `fiberpulse-site/components/release-downloads.tsx` appelle :

```text
/api/release?v=3
```

Le proxy vérifie la dernière release `aston76/fiberpulse-agent` et sait exposer chaque plateforme indépendamment :

```text
FiberPulse-macos-universal.zip
FiberPulse-windows-x64-setup.exe (uniquement après signature complète)
```

Au 31/08/2026, l’API publie `v0.1.0` avec `macos` disponible et `windows: null`. Les cartes localisées distinguent clairement :

- Windows signé/Store ;
- Mac téléchargement provisoire non notarialisé avec documentation manuelle ;
- aucune promesse `Signed` ou `Notarized` non vraie.

## 8. Observatoire public et confidentialité

Le site contient l’interface active de l’observatoire avec pays, drapeau, date/heure, fournisseur, offre, débit, agrégats et recherche. Les endpoints o2switch renvoient des collections valides, y compris quand aucune mesure consentie n’est encore présente. Ne jamais insérer de fausses mesures pour donner une impression d’activité.

Ne jamais publier :

- nom, email ou téléphone ;
- numéro de compte client ;
- adresse postale exacte ;
- GPS exact ;
- SSID ou hostname ;
- profil matériel personnel ;
- adresse IP exacte dans l’observatoire FiberPulse.

Le partage public reste optionnel et séparé du consentement M-Lab.

## 9. Rapports et réclamations

L’application prévoit :

- rapports PDF professionnels avec logo, méthode, contexte et chronologie ;
- exports CSV ;
- synthèse après une semaine de mesures ;
- brouillon d’email de réclamation copiable ;
- ouverture dans la boîte mail locale de l’utilisateur, sans envoi automatique ;
- coordonnées support fournisseur et script d’appel technique ;
- profil local facultatif avec compte, installation, matériel et routeurs ;
- séparation stricte entre données personnelles locales et données publiques.

Les fichiers de démonstration actuellement non suivis sont dans :

```text
fiberpulse-agent/output/pdf/
```

## 10. Commandes de développement et vérification

Depuis `fiberpulse-agent` :

```sh
make test
make dashboard
make build
make windows
make macos
```

Versions attendues documentées : Go 1.26.7 et Node 24 LTS. Des builds conteneurisés existent pour limiter les dépendances hôte.

Tests ciblés updater :

```sh
go test ./internal/app ./internal/update ./cmd/release-sign
```

Site :

```sh
cd fiberpulse-site
npm ci
npm run lint
SITE_ORIGIN=https://testspeednow.com npm run build
```

État Git global :

```sh
for repo in fiberpulse-agent fiberpulse-site fiberpulse-platform fiberpulse-methodology; do
  git -C "$repo" status --short --branch
done
```

## 11. Secrets et règles de sécurité

Ne jamais afficher, committer ou copier dans une passation :

- token cPanel ;
- clés privées Ed25519 de release ;
- certificats Windows/macOS ;
- mots de passe de certificats ;
- clés App Store Connect ;
- identifiants de boîte mail ;
- tokens LWS/GitHub.

N’enregistrer que les noms de secrets et leurs pointeurs locaux. Les secrets GitHub attendus par le workflow de release incluent notamment :

```text
FIBERPULSE_UPDATE_SIGNING_KEY
WINDOWS_CERTIFICATE_BASE64 / mot de passe associé (selon workflow actuel)
MACOS_CERTIFICATE_BASE64
MACOS_CERTIFICATE_PASSWORD
MACOS_SIGNING_IDENTITY
APPLE_API_KEY_BASE64
APPLE_API_KEY_ID
APPLE_API_ISSUER_ID
```

Relire le workflow avant de créer les secrets : les noms exacts peuvent évoluer.

## 12. État vérifié de la CI

Dernière CI agent vérifiée comme réussie :

```text
https://github.com/aston76/fiberpulse-agent/actions/runs/33326921416
```

Release macOS provisoire vérifiée comme réussie :

```text
https://github.com/aston76/fiberpulse-agent/actions/runs/33326922499
```

Une réussite CI locale ou sur une ancienne archive ne suffit pas pour publier. Vérifier l’artefact exact produit par la dernière exécution réussie.

Les deux anciennes instabilités liées aux horodatages SQLite ont été corrigées : l’ordre des consentements utilise l’ordre d’insertion et la première tentative de partage reste exigible même en cas de recul de l’horloge. Les matrices macOS 14/15 et Windows sont vertes sur le run ci-dessus.

## 13. Prochaines actions, dans l’ordre

1. Attendre la fin de la certification Microsoft de la soumission Windows et relire son statut dans Partner Center.
2. Après validation Store, vérifier la fiche publique, l’installation, le bouton Quit et la mise à jour Microsoft Store sur un Windows 10/11 réel.
3. Ne publier l’installateur Windows direct qu’après signature Authenticode et essai réel ancienne version → mise à jour → reçu de santé → historique conservé.
4. Garder l’auto-update Mac public désactivé tant qu’Apple Developer ID et la notarisation ne sont pas disponibles.
5. Effectuer une revue juridique par pays avant toute promesse marketing fondée sur une règle réglementaire ou contractuelle.

## 14. Extension multi-pays (réalisée le 30/08/2026)

Décision : ADR 0001 (critères) + ADR 0002 (toutes les vagues fusionnées) dans `fiberpulse-methodology/adr/`.

- **11 pays en production de données** : PH, US, GB, DE, FR, AU, CA, CH, ES, BR, IN — 213 offres documentées dans `fiberpulse-platform/data/catalog/`.
- Contrat `plan-catalog-v1` : `fiberpulse-methodology/schemas/plan-catalog-v1.schema.json`.
- Agent : le catalogue Go est devenu un chargeur `go:embed` sur `internal/plan/data/*.json` (rafraîchi par `make catalog`) ; `PriceAmount` est passé en `float64` ; le sélecteur de pays avec drapeaux existe dans le dashboard ; le libellé de vitesse respecte la base légale du pays (« up to », « average » UK, « typical » 5G fixe).
- Internationalisation de l’app : interface embarquée disponible hors ligne en anglais, français, allemand, espagnol, portugais du Brésil, italien et hindi (`dashboard/src/i18n.ts`). La langue du système est détectée au premier lancement ; un sélecteur dans Réglages conserve ensuite le choix localement. La langue sélectionnée est aussi enregistrée dans les événements de consentement M-Lab/FiberPulse.
- Site : section « Coverage » avec drapeaux (`components/coverage-section.tsx`), données synchronisées par `npm run sync:catalog` depuis la plateforme. La page produit et la politique de confidentialité disposent de routes indexables en anglais, français, allemand, espagnol, portugais du Brésil, italien et hindi (`/`, `/fr`, `/de`, `/es`, `/pt-br`, `/it`, `/hi` et leurs routes `/privacy`), avec langue du document, URL canonique, `hreflang` et sitemap multilingue.
- Veille mensuelle automatisée (automatisation Codex `fiberpulse-catalog-watch`, le 1er de chaque mois) : revérifier les pages officielles, valider humainement, mettre à jour puis synchroniser agent et site.
- Les exports PDF, CSV, le brouillon de réclamation et l’e-mail `.eml` suivent désormais la langue active. Hindi utilise les polices embarquées Noto Sans Devanagari sous licence OFL. Les noms officiels des offres et les valeurs saisies par l’abonné ne sont jamais traduits.
- L’ADR 0003 documente les parcours pays et sépare strictement les bandes diagnostiques produit des seuils juridiques. Les bandes 70 %/40 % ne sont jamais présentées comme des règles légales ; l’ancienne mention non sourcée d’un seuil « ARCEP 60 % » est abandonnée.
- Reste à faire au fil de la veille : revue juridique avant communication marketing et traitement des gaps explicitement conservés dans `fiberpulse-platform/data/catalog/README.md`.

## 15. Observatoire o2switch et flux public (30/08/2026)

- L’application conserve le trajet du test directement entre le client et M-Lab. Après consentement séparé au partage public, elle transmet uniquement l’événement public minimisé à `https://testspeednow.com/api/v1/measurements`.
- Les requêtes sont liées à une clé locale Ed25519 et protégées par signature du corps, horodatage de cinq minutes, nonce UUID, séquence monotone, idempotence et limite horaire. L’identifiant technique d’installation n’est jamais exposé par l’API publique.
- Le récepteur o2switch est dans `fiberpulse-site/deploy/o2switch/api/v1/index.php`. Il utilise PHP Sodium et PDO SQLite. La base `observatory.sqlite` est créée dans `fiberpulse-private`, au-dessus du docroot, avec permissions de répertoire privées.
- Aucun nom, e-mail, téléphone, numéro de compte, adresse exacte, IP exacte, GPS, SSID, hostname ou profil d’appareil n’est accepté ni stocké dans le schéma public.
- `fiberpulse-site/deploy/o2switch/api/observatory/index.php` fournit mesures paginées, facettes pays/fournisseurs et agrégats par pays. Le site interroge ce flux toutes les 15 secondes, annonce le nouveau test anonymement et met la nouvelle ligne en lumière. `prefers-reduced-motion` désactive l’animation.
- L’interface de l’observatoire est disponible dans les sept langues du site, y compris les états de chargement, d’erreur et de données vides. Les anciennes fausses lignes de démonstration ont été retirées.
- Test local prouvé : inscription → signature Ed25519 → insertion SQLite → lecture publique. Les capacités `pdo_sqlite` et `sodium` doivent aussi être vérifiées sur l’hébergement réel via `/api/v1/health` après chaque déploiement.
- Une maintenance GitHub Actions quotidienne à 01:17 UTC crée une sauvegarde SQLite transactionnelle dans le répertoire privé, vérifie son intégrité, conserve 35 jours de sauvegardes et purge uniquement les nonces expirés et installations techniques inactives depuis plus de 366 jours. Les mesures historiques ne sont pas purgées par cette maintenance.
- Preuve de production : run `33327951193`, sauvegarde de 40 960 octets, puis `/api/v1/health` avec `status: ok`, `storage: true`, `signatures: true` et `backup_fresh: true`.

## 16. Critères de fin pour la prochaine reprise

Ne considérer la distribution comme terminée que si :

- `testspeednow.com` pointe publiquement sur `109.234.160.138` ;
- le certificat TLS est valide sans contournement ;
- les pages `/`, `/privacy/`, `robots.txt`, `sitemap.xml` et les endpoints API attendus répondent ;
- Windows Store affiche réellement la version publique ;
- la release GitHub contient les bons fichiers et hashes ;
- le téléchargement du site correspond exactement à l’artefact vérifié ;
- l’update Windows est prouvé depuis une installation antérieure ;
- le parcours Mac décrit honnêtement l’avertissement Gatekeeper ;
- aucune donnée personnelle n’apparaît dans l’observatoire public ;
- le bouton Quit ferme l’interface, l’agent, le serveur loopback et les tâches en arrière-plan.
