# obsrv — Recherche UX et direction de conception

> Version 0.1, 2026-10-09. Document de travail : il sert à décider ensemble de l'UI qu'on construit.
> [DESIGN.md](DESIGN.md) reste la référence pour le langage visuel (tokens, typographie, densité).

## 1. Ce qu'on a étudié

| Source | Ce qu'on en retient |
|---|---|
| Datadog : le design system DRUIDS | La prévisibilité compte plus que la cohérence, et le contexte doit toujours rester à portée de main |
| Honeycomb : BubbleUp, et la refonte de ses visualisations | L'outil explique *ce qui diffère*, au lieu de seulement montrer. Et on corrige la friction avant la beauté |
| Grafana : Drilldown | On explore sans écrire de requête, on ventile par attribut et on bascule d'un signal à l'autre |
| Datadog : Watchdog Insights | L'explorateur fait remonter tout seul les `clé:valeur` sur-représentées parmi les erreurs |
| Sentry : le regroupement en issues | Des milliers d'erreurs deviennent une poignée de problèmes, avec leur première et leur dernière apparition |
| Dash0 | Tout est centré sur la ressource : un clic sur un span donne les logs et les métriques de la même ressource |
| incident.io : l'observabilité pour l'astreinte | On descend en trois niveaux (vue d'ensemble, puis système, puis logs et traces), et on évite la « soupe de dashboards » |
| Linear | La vitesse est une décision de design : clavier d'abord, Cmd+K, détails révélés seulement au besoin |

## 2. Les leçons, et ce qu'elles impliquent pour nous

1. **L'utilisateur arrive avec une question, pas avec un signal.** Personne ne se dit « je vais regarder des logs ». On se demande « qu'est-ce qui ne va pas ? » ou « pourquoi checkout est lent ? ». Les outils qui partent du signal (Logs / Traces / Metrics) obligent l'utilisateur à traduire sa question en navigation. *(incident.io, Grafana Drilldown)*
2. **Une vue d'ensemble « feu tricolore », puis on descend.** Le premier écran d'une astreinte doit dire en deux secondes où regarder. Il ne cherche pas à tout expliquer : il pointe la suite. *(incident.io)*
3. **Chaque valeur est une porte.** Ne jamais laisser l'utilisateur dans une impasse : le contexte suivant est toujours à un clic, que ce soit pour filtrer, exclure, ouvrir les logs ou les traces de cette valeur. *(DRUIDS : « minimizing dead ends », Dash0)*
4. **Expliquer plutôt que montrer.** La meilleure aide est de dire *ce qui distingue* les requêtes en erreur des autres, par exemple `service.version:1.4.2` sur-représenté à 90 % parmi les erreurs. C'est ce qui fait la valeur de BubbleUp et de Watchdog. *(Honeycomb, Datadog)*
5. **Regrouper avant de lister.** 2 000 lignes d'erreur, ce sont en réalité 3 problèmes. Il faut les montrer comme des problèmes (« issues »), avec leur volume, leur tendance, leur première apparition et un signal de régression. *(Sentry)*
6. **Une ressource, tous ses signaux.** Le service (ou le pod) est le pivot naturel : sa page doit donner ses traces, ses logs, ses métriques et ses erreurs sur la même période, sans rien ressaisir. *(Dash0)*
7. **La prévisibilité plutôt que la cohérence de façade.** Le même geste doit produire le même effet partout : un clic sur une valeur, un glisser sur un graphique, `Esc`. On apprend une fois, on l'utilise partout. *(DRUIDS)*
8. **La vitesse est une fonctionnalité.** Les transitions doivent prendre moins de 100 ms. Les résultats précédents restent affichés pendant le chargement, et on peut tout faire au clavier. *(Linear)*
9. **D'abord la friction, ensuite la beauté.** On priorise ce qui ralentit le diagnostic, et la beauté vient en soutien. *(Honeycomb)*

## 3. Audit honnête de l'UI actuelle

Notre [DESIGN.md](DESIGN.md) promet déjà plusieurs de ces principes, mais **l'UI ne les tient pas encore** :

| Promesse ou leçon | État actuel |
|---|---|
| « Everything is clickable » (DESIGN §2) | ❌ Les valeurs d'attributs sont du texte inerte dans le panneau de détail |
| « Dragging across a chart zooms » (DESIGN §5) | ❌ Il n'y a pas de zoom au glisser |
| « Keyboard first » (DESIGN §7) | ❌ Ni Cmd+K, ni `/`, ni `j`/`k` |
| Vue d'ensemble « feu tricolore » | ❌ On arrive sur un tableau de services, et rien ne dit si tout va bien |
| Expliquer ce qui diffère | ❌ Aucune aide : l'utilisateur doit deviner quels filtres essayer |
| Regrouper les erreurs | ❌ Les erreurs sont dispersées entre logs et traces |
| Une ressource, tous ses signaux | 🟡 La page service existe, mais ses traces, logs et métriques sont dans d'autres pages |
| Explorer sans écrire de requête | 🟡 Les facettes et le builder de métriques vont dans ce sens, mais la liste des métriques est brute (30 noms techniques) |
| Bascule entre signaux | 🟡 De la trace aux logs : oui. Du graphique aux traces de ce moment-là : non |

Le constat : **l'apparence est bonne, le parcours ne l'est pas encore.** On a une belle interface de consultation, pas encore une interface de diagnostic.

## 4. Notre thèse : une UI de diagnostic, pas un catalogue de données

> **obsrv répond à « qu'est-ce qui ne va pas, et pourquoi ? » en moins de 5 clics, sans écrire une requête.**

Six idées structurantes, dont deux qu'aucun concurrent open source ne fait bien :

### A. Démarrer par la question : un écran **Home** « Qu'est-ce qui ne va pas ? »
- Un feu tricolore par service (débit, erreurs, latence), comparé à la même durée juste avant.
- Les alertes en cours, les **nouveaux problèmes** (issues apparues ou revenues) et les **déploiements** récents, détectés par les changements de `service.version`.
- Chaque élément mène directement à la suite logique. On ne range rien dans des dashboards.

### B. « Chaque valeur est une porte » : un **menu de valeur** universel
Un clic sur n'importe quelle valeur (service, attribut, trace ID, niveau, version…), partout dans l'UI, propose toujours les mêmes actions :
`Filtrer` · `Exclure` · `Grouper par` · `Voir les logs` · `Voir les traces` · `Voir les métriques` · `Copier`.
C'est le geste unique qui rend l'UI prévisible (leçon 7), et c'est la base de toute la corrélation.

### C. « Expliquer ce qui diffère » : l'onglet **Compare**, notre fonctionnalité phare
On sélectionne un pic sur un graphique, ou « les requêtes en erreur ». obsrv classe alors les attributs **sur-représentés** par rapport au reste :

```
Ce qui distingue les 84 requêtes en erreur des 790 autres
  payment.issuer = acme-bank        92 %  vs  31 %   ████████████░
  service.version = 1.4.2           88 %  vs  50 %   ██████████░░░
  http.route = /pay                100 %  vs  60 %   ████████░░░░
```
Avec DuckDB, c'est une requête de distribution par attribut sur la sélection et sur le reste : le calcul est peu coûteux. Honeycomb en fait une offre payante, OpenObserve, SigNoz et Grafana ne l'ont pas sous cette forme. **C'est notre meilleur argument « wow ».**

### D. « Regrouper avant de lister » : la page **Issues**
- On regroupe les erreurs (logs ERROR et spans en erreur) par empreinte : service, puis `exception.type`, puis le message normalisé (nombres et IDs remplacés par des jokers).
- Chaque issue affiche son volume, une mini-courbe, sa première et sa dernière apparition, les services touchés et le statut **Nouveau** ou **Régressé** par rapport à la période précédente.
- Un clic montre un exemple, ses traces, et l'onglet Compare préfiltré.

### E. « Une ressource, tous ses signaux » : la page service devient un **hub à onglets**
`Vue d'ensemble · Traces · Logs · Métriques · Issues`. Tout est préfiltré sur le service et la période, avec le même explorateur que les pages globales. On ne ressaisit jamais rien.

### F. Clavier et graphiques actifs
- **Cmd+K** : aller à un service, une métrique, une page. Coller un trace ID ouvre la trace.
- `/` pour chercher, `j`/`k` pour naviguer, `Esc` pour fermer, `[` et `]` pour la période précédente ou suivante.
- **Glisser sur un graphique** zoome la période globale. **Cliquer un point** propose « logs à ce moment », « traces à ce moment » et « comparer ce pic ».

## 5. Architecture d'information proposée

```
◎  Home          Qu'est-ce qui ne va pas ?          ← nouvel écran d'arrivée
⬡  Services      liste + map → hub par service
⚠  Issues        erreurs regroupées                ← nouveau
───
≡  Traces  ┐
▤  Logs    ├─    même explorateur : recherche · chips · graphe · onglets [Liste | Compare | Patterns]
⌁  Metrics ┘
───
▦  Dashboards    (plus tard)
🔔 Alerts
```

### Le gabarit d'explorateur, commun aux trois signaux
```
┌ recherche ──────────────────────────────────────────────── [◷ 30 min ⌄] ┐
│ [level is error ✕] [service ⌄] [env ⌄] [+ filtre]                       │
├──────────────────────────────────────────────────────────────────────────┤
│ ▂▃▅▂▁▃▆█▅▃   ← glisser = zoom · clic = menu (logs/traces/compare ici)     │
├──────────────────────────────────────────────────────────────────────────┤
│ [Liste 2.2K] [Compare] [Patterns]                                        │
│  …résultats…                                    │ panneau de détail      │
│                                                 │ valeurs cliquables →   │
│                                                 │ menu de valeur         │
└─────────────────────────────────────────────────┴────────────────────────┘
```

### L'écran Home
```
┌ Tout va bien ? ─────────────────────────────────────────── [◷ 30 min ⌄] ┐
│ ● 2 services dégradés  ● 1 alerte  ● 1 nouveau problème  ● 1 déploiement  │
├────────────── Services ─────────────────────┬─────── À regarder ─────────┤
│ ● checkout   0.8/s   ⚠ 11% err ↑  p95 180ms │ ⚠ NOUVEAU  CardDeclined     │
│ ● payment    0.8/s     3% err     p95 840 ↑ │   payment · 84 en 30 min    │
│ ● frontend   2.3/s     4% err     p95 157   │ 🔔 Payment refusals (firing) │
│ ○ inventory  2.3/s     0%         p95 37    │ ⇡ checkout 1.4.2 il y a 12m  │
└─────────────────────────────────────────────┴────────────────────────────┘
```

## 6. Critère de réussite : un « game day » chronométré

Le scénario de la démo est le suivant : la banque de paiement ralentit et refuse des cartes. On mesure le nombre de clics et le temps entre l'arrivée dans obsrv et la phrase « c'est la banque, sur `/pay`, depuis 21h02 ».

| | Aujourd'hui (estimé) | Cible |
|---|---|---|
| Clics jusqu'à la cause | ~9 (Services → service → Traces → filtre erreur → trace → span → retour → Logs → recherche) | **≤ 4** (Home → issue → Compare → trace) |
| Requête à écrire | 1 (`service:payment level:error`) | **0** |

## 7. Plan de construction proposé

| Étape | Contenu | Pourquoi d'abord |
|---|---|---|
| **1. Fondations d'interaction** | Menu de valeur universel, valeurs cliquables partout, glisser pour zoomer, Cmd+K et raccourcis | Ce socle sert à tout le reste, et il respecte enfin DESIGN.md |
| **2. Home** | Feu tricolore par service avec comparaison à la période précédente, alertes, déploiements | C'est l'écran d'arrivée de l'astreinte |
| **3. Issues** | Empreinte des erreurs côté serveur, page Issues, Nouveau et Régressé | Il transforme le bruit en liste de problèmes |
| **4. Compare** | Distribution des attributs pour une sélection et pour le reste, côté serveur, plus l'onglet dans les explorateurs | La fonctionnalité phare, différenciante |
| **5. Hub service** | Onglets préfiltrés qui réutilisent l'explorateur | Une ressource, tous ses signaux |

Chaque étape est livrée en TDD et vérifiée en capture, et le game day est rejoué à la fin de chacune.

## Sources
- [DRUIDS, the design system that powers Datadog](https://www.datadoghq.com/blog/engineering/druids-the-design-system-that-powers-datadog/)
- [Honeycomb — Data visualization facelift](https://honeycomb.io/blog/data-facelift-honeycomb-data-visualization) · [BubbleUp](https://docs.honeycomb.io/investigate/analyze/identify-outliers.md)
- [Grafana Drilldown: the queryless experience](https://grafana.com/blog/grafana-drilldown-apps-the-improved-queryless-experience-formerly-known-as-the-explore-apps/)
- [Datadog Watchdog Insights for Logs](https://docs.datadoghq.com/logs/explorer/watchdog_insights/) · [Saved views](https://docs.datadoghq.com/logs/explorer/saved_views)
- [Sentry — Grouping and fingerprints](https://docs.sentry.io/product/sentry-basics/grouping-and-fingerprints/)
- [Dash0 — resource-centric observability](https://www.dash0.com/application-performance-management)
- [incident.io — Building On-call: our observability strategy](https://incident.io/blog/building-on-call-our-observability-strategy)
- [The Linear aesthetic: density and keyboard-first UX](https://www.buildmvpfast.com/blog/linear-aesthetic-tokens-density-keyboard-first-ux-2026)
