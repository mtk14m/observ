# obsrv — Roadmap

Chaque phase a un **critère de sortie**. On ne passe pas à la suivante sans l'avoir atteint.
Les 3 signaux avancent **ensemble**, phase par phase : on ne fait pas « logs d'abord », pour ne jamais se retrouver avec un signal oublié.

## Phase 0 — Valider les choix risqués (3–4 semaines)

- Pipeline minimal pour les 3 signaux : OTLP → pdata → WAL → Arrow → Parquet → disque local.
- Banc de charge : `telemetrygen` + la **démo OpenTelemetry**, sur 4 vCPU / 8 Go.
- Mesures : évts/s, Mo/s, CPU, RSS, GC, ratio de compression par signal, latence d'ack.
- Comparatif : `arrow-go/parquet` contre `parquet-go`, l'effet du tri, les niveaux de ZSTD, et le schéma métriques (séries séparées ou inline).
- **DuckDB** : requêtes sur le Parquet + lecture des buffers Arrow en mémoire via `duckdb-go`, et requête de métrique p95/rate sur 7 jours.
- Baseline : le même dataset dans OpenObserve et dans SigNoz.

**Sortie :** `bench/RESULTS.md` montre au moins 50 Go/jour sur 4 vCPU, des ratios de compression proches des cibles, et un graphique de métrique sur 7 jours en moins de 1,5 s. Sinon, on corrige le schéma avant d'aller plus loin.

## Phase 1 — Moteur complet, sans UI (8–10 semaines)

- Ingestion des 3 signaux : WAL, buffers chauds, flush, backpressure.
- Metadata SQLite, planner avec pruning, IR + compilation SQL.
- API : recherche de logs, trace-by-id, recherche de traces, requête de métrique (IR), SQL.
- RED dérivées des spans, service map.
- Compaction, rollups, rétention. ObjectStore local et S3.
- Schéma public v1 + `SCHEMA.md` + **exit test en CI**.
- Self-telemetry.

**Sortie :** obsrv ingère la démo OTel et sa propre télémétrie pendant 7 jours dans un seul container, sans perte, en tenant les cibles de `PRODUCT.md` §5.

## Phase 2 — Le produit utilisable (8–10 semaines)

- UI : Explorer logs, Explorer traces (waterfall), builder de métriques, page Service (RED + map + erreurs + logs), dashboards, et navigation corrélée.
- Alerting + notifications (webhook, Slack, email).
- Auth admin local, puis OIDC.
- Serveur MCP.
- `docker run` documenté, docker-compose avec la démo OTel, un site et une doc.

**Sortie :** 10 équipes externes l'utilisent (GitHub, communautés FR/EU), le « du `docker run` à la première trace » est mesuré sous les 5 minutes, et on recueille des retours qualitatifs sur la corrélation.

## Phase 3 — Lancement open source (V1.0)

- Stabilisation, upgrade path, sauvegarde et restauration du volume.
- Lancement public (Hacker News, Reddit r/devops et r/selfhosted, CNCF Slack, meetups OTel Paris).
- Mesure de la traction : stars, installations (télémétrie d'usage opt-in), contributeurs.

## Plus tard, selon la traction

- **V2 :** mode cluster (rôles séparés, Postgres, S3 obligatoire), HA, SSO/RBAC. C'est le début de l'offre payante.
- OTel-Arrow en entrée, profiles OTel (dès que le signal est stable), dashboards importables.
- Rust pour un hot path, selon la règle d'`ARCHITECTURE.md` §9.

## Ce que je ferais cette semaine

1. `go mod init`, squelette `cmd/obsrv`, `internal/otlp`, `internal/model`, `internal/encoding/parquet`, `internal/objstore/fs`.
2. `deploy/docker-compose.yml` avec la démo OpenTelemetry pointée sur obsrv.
3. Écrire un premier fichier Parquet **de chaque signal** et l'interroger avec la CLI DuckDB : ce sera le premier exit test.
