# PostgreSQL without TimescaleDB

The MVP uses **plain PostgreSQL** (`postgres:17`) with indexes on `(meter_id, timestamp)`. No TimescaleDB extension, no hypertable.

## Context

The PDF leaves persistence architecture open. An earlier plan proposed TimescaleDB, but no `consideraciones_persistencia.md` is present in this repository; this ADR records the actual choice rather than citing a nonexistent document.

TimescaleDB is justified by write throughput and query volume this project does not have: 4,032 rows total, 336 per meter, 12 meters, imported for analysis and only replaced on an explicit reseed. Analysis writes a small number of result rows, and neither the dashboard nor the detector needs hypertables.

## Consequences

- TimescaleDB is not in a stock `postgres` image; using it would add build and setup complexity for the evaluator without a measurable gain on this dataset.
- Hypertables buy nothing at 336 rows per meter. Revisit this decision if ingest volume or query demands change substantially.
- The source readings and the run/anomaly results share one relational database without an extension.
