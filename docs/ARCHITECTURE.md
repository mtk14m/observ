# obsrv — Architecture V1

> Go, un binaire, un container. Logs + metrics + traces. Pas de PromQL.
> Le format de stockage est le vrai contrat : c'est lui qui rend possible un mode cluster, puis une réécriture partielle en Rust plus tard.

## 1. Vue d'ensemble : tout dans un container

```
 Apps (SDK OTel officiels) ─► [OTel Collector optionnel] ─► OTLP gRPC :4317 / HTTP :4318
                                                                   │
┌──────────────────────────── container obsrv (1 process Go) ─────┼──────────────────┐
│                                                                  ▼                  │
│  ingest   OTLP receiver → pdata → modèle canonique → WAL (fsync, group commit) → ack │
│                                      │                                              │
│                                      ▼                                              │
│  hot      buffers Arrow en mémoire (≈ dernières minutes ; 2 h pour les métriques)   │
│                                      │ flush (taille/temps)                         │
│                                      ▼                                              │
│  store    Parquet+ZSTD → ObjectStore (disque local | S3) → metadata (SQLite)       │
│  compact  fusion, tri, rollups métriques, rétention                                 │
│                                                                                     │
│  query    builder JSON / recherche logs / SQL → planner (pruning) → DuckDB          │
│           (Parquet élagués + buffers chauds Arrow) | chemins Go : trace-by-id, tail │
│  alert    évaluation périodique des règles → notify (webhook, Slack, email)         │
│  api      HTTP/JSON, MCP, UI web embarquée (go:embed) :8080                         │
└─────────────────────────────────────────────────────────────────────────────────────┘
                     │ volume /data (WAL, Parquet, SQLite, cache)
                     └─ optionnel : bucket S3 du client pour le Parquet
```

**Aucune dépendance externe.** SQLite et DuckDB sont embarqués, l'UI est servie par le binaire.
Le flag `-target=all|ingest|query|compact` existe dès le départ pour préparer le mode cluster (V2), mais la V1 n'est testée et supportée qu'en `all`.

## 2. Décisions clés (ADR dans `docs/adr/`)

| # | Décision | Pourquoi |
|---|---|---|
| 001 | Go pour tout le moteur | Maîtrise de l'équipe, écosystème OTel. Rust seulement pour un hot path mesuré (§9) |
| 002 | OTLP est la seule entrée | Vendor-neutral. Prometheus et les autres sources passent par le Collector OTel |
| 003 | WAL local + group commit avant l'ack | Ack en ~10–50 ms, perte nulle, et un replay au redémarrage reconstruit les buffers chauds |
| 004 | Parquet + ZSTD, avec un schéma public versionné | Zéro lock-in prouvable (exit test DuckDB) |
| 005 | **Un seul exécuteur : DuckDB embarqué.** Go fait le planning et le pruning, et génère le SQL | Un moteur au lieu de trois ; pas de DataFusion en Go ; on ne réécrit pas un moteur SQL |
| 006 | Pas de PromQL ni de langage maison : un builder (IR JSON) compilé en SQL | Simplicité pour l'utilisateur, et une seule surface à maintenir |
| 007 | SQLite pour la metadata, les dashboards, les alertes et les users | Embarqué ; Postgres derrière la même interface en V2 |
| 008 | UI web embarquée : **Vue 3** + TypeScript + Vite + uPlot | Plus léger et plus lisible que React ; uPlot tient très bien les grosses séries temporelles. Voir `docs/adr/0004` |

**Coût assumé de DuckDB :** cgo (binding officiel `duckdb-go`). La cross-compilation est plus pénible et l'image est plus grosse (~+40 Mo). C'est acceptable pour un container. On le garde derrière l'interface `QueryEngine`.

## 3. Ingestion

1. **Réception :** `go.opentelemetry.io/collector/pdata` + serveurs OTLP gRPC/HTTP. On n'embarque pas tout le Collector.
2. **Normalisation :** pdata → modèle canonique. Les resources sont dédupliquées par fingerprint ; les séries de métriques sont identifiées par `series_id = hash(metric, attributes, resource)`.
3. **WAL :** les segments sont en append-only, avec un fsync groupé toutes les ~10–50 ms, puis l'ack. Le WAL est tronqué une fois le Parquet persisté.
4. **Buffers chauds :** un builder Arrow par signal. Les requêtes lisent **à la fois** les buffers et le Parquet, ce qui donne une fraîcheur inférieure à la seconde.
5. **Flush :** un fichier Parquet par signal, à ~64–128 Mo ou toutes les ~30 s–2 min (moins de petits fichiers qu'en mode sans état, puisque le WAL assure la durabilité).
6. **Backpressure :** au-dessus de la limite RAM ou disque, on renvoie `RESOURCE_EXHAUSTED` / HTTP 429.
7. **Hygiène Go :** `sync.Pool`, `GOMEMLIMIT` réglé sur la limite du container, pprof en continu, et obsrv s'observe lui-même en OTLP.

## 4. Stockage : le contrat public

```
<root>/v1/
  logs/            date=YYYY-MM-DD/hour=HH/<ulid>.parquet
  spans/           date=YYYY-MM-DD/hour=HH/<ulid>.parquet
  metric_points/   date=YYYY-MM-DD/hour=HH/<ulid>.parquet
  metric_rollups/  res=1m|1h/date=YYYY-MM-DD/<ulid>.parquet
  metric_series/   <ulid>.parquet                ← dictionnaire des séries
  resources/       <ulid>.parquet                ← resources dédupliquées
  SCHEMA.md
```
Le `<root>` est `/data/store` ou `s3://bucket/prefix`, derrière l'interface `ObjectStore`. Le mode mono-tenant est le défaut ; le champ `tenant` existe déjà dans le schéma pour plus tard.

### Logs et spans
- Les noms suivent les **conventions sémantiques OTel** et on ne renomme rien.
- Les attributs chauds ont leurs colonnes dédiées (`service.name`, `k8s.namespace.name`, `http.route`, `http.response.status_code`, `db.system`, `error.type`…), le reste va dans une `map`. La liste des colonnes promues est versionnée.
- `trace_id`/`span_id` sont en `FIXED_LEN_BYTE_ARRAY` dans **tous** les signaux, ce qui fait la corrélation.
- Tri : `(service, timestamp)` pour les logs, `(trace_id, start_time)` pour les spans.
- Index : stats min/max et bloom filters sur `trace_id` et `service.name`. Pour la recherche texte, on fait un scan après pruning en V1 (largement suffisant à l'échelle d'un container). Des blooms de tokens viendront si les mesures l'exigent.

### Metrics (le point dur sans PromQL)
- **`metric_series`** : `series_id, metric_name, type, unit, temporality, is_monotonic, attributes, resource_id`.
- **`metric_points`** : `series_id, time, start_time, value_double|value_int`, plus pour les histogrammes `count, sum, min, max, bounds[], bucket_counts[]` (et les champs exponentiels), plus `exemplars[{trace_id, span_id, value, time}]`.
- Tri `(metric_name, series_id, time)` ; encodage `DELTA_BINARY_PACKED` pour les temps, `BYTE_STREAM_SPLIT` pour les doubles. C'est la source principale du ratio de compression.
- **Temporalité :** on stocke ce qu'on reçoit (cumulative ou delta). Le `rate`/`delta` est calculé à la requête, avec détection des resets.
- **Rollups** (calculés par le compactor) : par série et par intervalle de 1 min et de 1 h, on garde `min, max, sum, count, last` ; pour les histogrammes, on fusionne les buckets. Le planner choisit la résolution selon la plage demandée.
- **Fenêtre chaude** : les 2 dernières heures restent en mémoire, ce qui sert l'alerting et les dashboards « live ».
- **Garde-fou de cardinalité :** une limite de séries actives par métrique, au-delà de laquelle on agrège dans `__overflow__` (même idée que les cardinality limits du SDK OTel) et on affiche un avertissement.

## 5. Requêtes : une seule IR, un seul exécuteur

```
UI builder ─┐
recherche   ├─► Query IR (JSON) ─► planner Go ─► SQL DuckDB ─► résultat Arrow ─► JSON
logs        │                      │ pruning : temps, stats, blooms, rollup choisi
SQL avancé ─┘                      │ + buffers chauds injectés comme tables Arrow
MCP ────────┘
```

IR métriques (ce que génère le builder) :
```json
{ "signal": "metrics", "metric": "http.server.request.duration",
  "filter": {"service.name": "checkout", "http.response.status_code": {"gte": 500}},
  "agg": "p95", "group_by": ["http.route"], "fn": "rate", "step": "1m",
  "range": {"from": "now-6h", "to": "now"} }
```

- **Recherche logs** : `service:api level:error "timeout" -health`. Le parseur est simple et compile vers la même IR.
- **SQL** : en lecture seule, sur le schéma public, avec un timeout, une limite mémoire et des macros prêtes (`rate()`, `histogram_quantile()`).
- **Chemins Go spécialisés** (hors DuckDB) : trace-by-id (blooms puis lecture des row groups ciblés) et live tail (abonnement aux buffers chauds).
- **Point à valider en phase 0 :** est-ce que DuckDB lit bien les buffers Arrow en mémoire via `duckdb-go` ? En mode S3, faut-il passer par `httpfs` linké statiquement, ou par un cache disque géré en Go ?

## 6. Corrélation (le cœur du produit)

- **Métriques RED dérivées des spans** à l'ingestion : rate, erreurs et histogramme de durée par `(service, opération)`. Elles sont écrites comme des métriques normales, et chaque service a son dashboard sans aucune instrumentation de métriques.
- **Service map :** les arêtes parent→enfant entre services sont agrégées pendant l'ingestion des spans.
- **Liens de navigation :**
  - d'une métrique à ses exemplars, puis à la trace ;
  - d'un pic au même service et à la même fenêtre de temps, puis aux traces lentes et aux logs d'erreur ;
  - d'une trace aux logs filtrés par `trace_id` ;
  - d'un service au déploiement (`service.version`).

## 7. Compaction, rétention, alerting

- **Compaction :** fusion des petits fichiers de chaque heure (cible 256 Mo à 1 Go), retri, reconstruction des blooms, calcul des rollups, puis swap atomique dans la metadata.
- **Rétention :** par signal (ex. logs 30 j, spans 15 j, points bruts 15 j, rollups 1 h pendant 13 mois). On supprime des partitions entières.
- **Alerting :** les règles sont des IR avec une condition (`> seuil pendant N min`). Elles sont évaluées toutes les 30 à 60 s sur la fenêtre chaude, avec les états OK/PENDING/FIRING/RESOLVED dans SQLite. Les notifications partent par webhook, Slack et email (SMTP).

## 8. Structure du code

```
cmd/obsrv/                 main, flags, -target
internal/
  otlp/                    receivers gRPC/HTTP, pdata
  model/                   modèle canonique (logs, spans, metrics, resources, series)
  wal/                     segments, group commit, replay
  hot/                     buffers Arrow en mémoire, live tail
  ingest/                  pipeline, backpressure, RED dérivées, service map
  encoding/parquet/        schéma public, writers, blooms
  objstore/                interface + fs/ + s3/
  metadata/                interface + sqlite/
  query/                   ir/, logsearch/ (parseur), planner/, duckdb/, tracebyid/
  compact/                 compaction, rollups, rétention
  alert/                   règles, évaluation, états
  notify/                  webhook, slack, smtp
  api/                     HTTP/JSON, MCP, auth
  selftelemetry/
web/                       UI (Vue 3 + TS + Vite + uPlot), compilée puis go:embed
pkg/schema/                schéma public (source de SCHEMA.md)
bench/                     harness de benchmark
test/exit/                 exit test DuckDB
deploy/                    Dockerfile, docker-compose (démo OTel), Helm plus tard
```

Interfaces pivot : `ObjectStore`, `TelemetrySink`, `MetadataStore`, `QueryEngine`.

## 9. Règle pour Rust

Un composant ne passe en Rust que si ces trois conditions sont réunies :
1. un profil montre qu'il pèse plus de 30 % du CPU sur une vraie charge ;
2. l'optimisation Go raisonnable est épuisée ;
3. un prototype Rust gagne plus de 2×.

Comme le format sur disque est public, un compactor ou un encodeur en Rust peut cohabiter avec le reste en Go sans migration.
