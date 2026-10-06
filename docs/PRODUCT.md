# obsrv — Product Brief

> Nom de travail. Version 0.2, 2026-10-06.
> Changements par rapport à la 0.1 : les 3 signaux sont dans la V1, il n'y aura pas de PromQL, tout tient dans un seul container, et on se repositionne face à Tsuga.

## 1. Le paysage concurrentiel

| Produit | Modèle | Cible | Signaux | Requête | Licence |
|---|---|---|---|---|---|
| **Tsuga** (Paris) | BYOC managé sur EKS/AKS/GKE | Grands comptes régulés, contrats à 6 chiffres | L+M+T, pipelines, PII, MCP | non public | propriétaire |
| OpenObserve | Single binary Rust, Parquet/S3 | Large | L+M+T+RUM | SQL + PromQL | AGPL |
| Parseable | Single binary Rust, Parquet/S3 | Large | L+M+T | SQL + PromQL | AGPL |
| SigNoz | Go + ClickHouse | PME/ETI | L+M+T | builder + ClickHouse SQL + PromQL | MIT/EE |
| Grafana LGTM | 4 systèmes Go | Large | L+M+T | LogQL + PromQL + TraceQL | AGPL |
| Datadog | SaaS | Tout le monde | tout | builder maison | propriétaire |

### Ce que Tsuga change pour nous

Tsuga, c'est des ex-Datadog (ils avaient déjà vendu Madumbo à Datadog), 45 M$ levés en six mois, et des clients comme Le Monde, Camunda et Black Forest Labs. Leur pitch : BYOC, souveraineté européenne, OTel-first, formats ouverts, MCP, prix au Go sans « taxe de rétention ».
**C'est presque mot pour mot la thèse de la v0.1 de ce document.** Les attaquer de front sur le terrain « BYOC entreprise régulée » serait perdu d'avance face à une équipe financée, expérimentée, et qui a déjà une force de vente.

Ce qu'ils ne font pas (et qu'ils n'ont pas intérêt à faire avec leur modèle économique) :
- **Pas d'open source.** Leur moteur est fermé.
- **Pas de self-service pour les petites équipes.** Leurs contrats moyens sont à six chiffres, et le déploiement passe par Kubernetes managé dans le compte cloud du client.
- **Pas de « docker run » qui marche tout de suite.**

C'est là qu'est notre place.

## 2. Thèse

> **Datadog dans un seul container. Open source, OTel-native, vos données restent chez vous.**

```
docker run -p 4317:4317 -p 4318:4318 -p 8080:8080 -v obsrv:/data obsrv/obsrv
```

Avec cette seule commande on a l'ingestion OTLP, le stockage, les requêtes, l'UI, les dashboards et l'alerting. Logs, metrics et traces, corrélés. Pas de Postgres, pas de ClickHouse, pas de Kafka, pas de Grafana à brancher.

Les quatre piliers :

1. **Tout-en-un, vraiment.** Les 3 signaux, l'UI, l'alerting et les notifications sont dans le binaire. On le distingue de la stack Grafana (4 systèmes), de SigNoz (qui demande ClickHouse) et de Tsuga (qui demande Kubernetes et un contrat).
2. **Simple à requêter : pas de PromQL, pas de langage maison à apprendre.** Les requêtes se font avec un query builder visuel, comme chez Datadog, qui couvre 95 % des besoins. Pour les cas avancés il y a du SQL standard, que tout le monde connaît. Pour la recherche de logs, une syntaxe de filtre simple : `service:api level:error "timeout"`.
3. **Zéro lock-in.** OTLP uniquement en entrée (SDK officiels). En sortie, du Parquet avec un schéma public. Le bucket reste lisible avec DuckDB sans obsrv (la CI le vérifie avec un « exit test »). Licence Apache 2.0.
4. **Les données restent chez le client** (disque local ou n'importe quel S3), par construction et non comme une option payante. On obtient la même souveraineté que Tsuga, sans le contrat entreprise.

## 3. Pour qui (ICP)

**Cible principale :** les équipes de 5 à 200 ingénieurs qui ont au moins un des problèmes suivants.
- Elles paient Datadog/New Relic trop cher pour leur taille, ou elles n'ont rien.
- Elles ont essayé la stack Grafana et elles ont trouvé ça trop lourd à opérer.
- Elles veulent s'auto-héberger (souveraineté, coût, ou tout simplement on-prem) **sans équipe plateforme dédiée**.

**Persona :** le développeur ou le « DevOps de l'équipe », qui installe l'outil un vendredi après-midi.
**Adoption :** bottom-up, via l'open source et GitHub. L'inverse exact du top-down entreprise de Tsuga.

**Plus tard :** les ETI, avec le mode cluster et une offre payante.

## 4. Périmètre V1 : les 3 signaux, chacun en version « l'essentiel bien fait »

Le risque d'un V1 avec 3 signaux, c'est de faire 3 produits médiocres. Pour l'éviter, chaque signal a un périmètre strict.

### Traces
- Recherche par service, opération, durée, statut, attributs.
- Vue trace (waterfall) et trace-by-id.
- Métriques RED (Rate, Errors, Duration) **dérivées automatiquement des spans** : pas besoin d'instrumenter des métriques à part.
- Service map dérivée des relations parent/enfant.

### Logs
- Recherche plein texte + filtres par attributs, en live tail.
- Histogramme du volume dans le temps, et facettes (service, niveau, namespace…).
- Saut direct log → trace via `trace_id`.

### Metrics
- Les 5 types OTLP : Sum, Gauge, Histogram, ExponentialHistogram, Summary.
- Un query builder : **métrique → filtres → agrégation (avg/sum/min/max/count/p50/p95/p99) → group by → fonction (rate, delta)**.
- Des dashboards (graphiques temps, valeurs, tableaux) et des rollups automatiques pour les longues périodes.
- Garde-fou de cardinalité par métrique, avec un avertissement dans l'UI.
- **Pas de PromQL. Pas de remote-write Prometheus en V1.** Les clients Prometheus passent par le Collector OTel (`prometheusreceiver`), qui fait le pont.

### Transverse
- **La corrélation est le cœur du produit :** de la métrique au pic, puis aux traces lentes de ce pic, puis aux logs de cette trace. Le lien se fait par `service.name`, l'intervalle de temps, et `trace_id`/`span_id` (y compris les exemplars OTel sur les métriques).
- Alerting : seuils sur les métriques et sur les comptages de logs/erreurs. Notifications par webhook, Slack et email.
- Auth : un admin local en V1, OIDC en V1.1.
- Serveur MCP : `search_logs`, `get_trace`, `query_metric`, `list_services`, `run_sql`.

## 5. Ce qu'on promet (et qu'on mesure)

| Promesse | Indicateur | Cible V1 (hypothèse) |
|---|---|---|
| Installation | du `docker run` à la première trace dans l'UI | < 5 min |
| Petit footprint | RAM au repos, en mode single container | < 512 Mo |
| Capacité d'un container | ingestion soutenue sur 4 vCPU / 8 Go | ≥ 50 Go/jour, à valider en phase 0 |
| Compression | octets OTLP / octets stockés | ≥ 10× logs, ≥ 8× traces, ≥ 15× metrics |
| Fraîcheur | délai entre l'ack et la donnée requêtable | < 5 s (et < 1 s pour les métriques récentes) |
| Durabilité | perte après ack | 0 |
| Dashboards | graphique de métrique sur 1 h / 7 j | p95 < 300 ms / < 1,5 s |
| Ouverture | exit test DuckDB | 100 % des signaux |

## 6. Non-objectifs

- Jamais de SDK ni d'agent propriétaire. Jamais d'attributs `obsrv.*` dans les données client.
- Pas de PromQL, ni LogQL, ni TraceQL, ni langage de requête maison. Le builder et SQL suffisent.
- V1 : pas de RUM, de session replay, de profiling, de SIEM ni de LLM observability.
- V1 : pas de mode cluster. Le single container doit d'abord être excellent.

## 7. Modèle économique

- **Le moteur complet, les 3 signaux, l'UI et l'alerting sont open source sous Apache 2.0**, sans aucune limite artificielle. C'est le moteur de l'adoption.
- **Ce qui sera payant (après le product-market fit) :**
  - le mode cluster / haute disponibilité ;
  - le SSO/SAML, le RBAC fin et l'audit ;
  - le support ;
  - éventuellement un cloud managé ou du BYOC pour les clients qui grandissent. C'est le terrain de Tsuga, on n'y va qu'avec une base installée.
- Le modèle de référence est celui de Grafana, Plausible ou Sentry : l'adoption open source d'abord, puis la monétisation de la taille et de la conformité.

## 8. Risques principaux

| Risque | Mitigation |
|---|---|
| Trois signaux, c'est trois fois le travail | Périmètre strict par signal (§4) ; un seul modèle de stockage et un seul moteur pour les trois |
| OpenObserve est déjà « single binary, 3 signaux » | Se différencier par la simplicité (pas de PromQL, builder d'abord), la corrélation, l'Apache 2.0 et le contrat de données ouvert |
| Requêtes de métriques lentes sur Parquet | Fenêtre chaude en mémoire, rollups, tri `(metric, series, time)` |
| Les limites d'un single container | Assumées en V1 ; le design par rôles (`-target`) prépare le mode cluster |
| Tsuga descend en gamme | Peu probable avec leur modèle économique ; notre avance serait l'open source et la communauté |
| Solo founder, périmètre large | Roadmap avec critères de sortie ; on dit non à tout ce qui sort du §4 |
