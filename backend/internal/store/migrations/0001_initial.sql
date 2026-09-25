-- Schema for the MVP. Applied in order by store.Migrate.
--
-- Four tables carry the data the API reads, and one carries the history of what
-- the detector found. Anomalies are rows rather than views: a finding that was
-- examined and dismissed is a fact worth keeping (ADR-0002).

CREATE TABLE IF NOT EXISTS meters (
    id          BIGSERIAL PRIMARY KEY,
    meter_id    TEXT        NOT NULL UNIQUE,
    name        TEXT        NOT NULL,
    location    TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- status is the source's own claim about the reading, carried through verbatim.
-- It is not this system's verdict: the delivered dataset marks all 4,032 rows OK,
-- including the 16 corrupt ones, so a verdict computed from it would be a verdict
-- about the pipeline, not about the meter.
CREATE TABLE IF NOT EXISTS readings (
    id              BIGSERIAL PRIMARY KEY,
    meter_id        TEXT             NOT NULL REFERENCES meters(meter_id),
    timestamp       TIMESTAMPTZ      NOT NULL,
    consumption_kwh DOUBLE PRECISION NOT NULL,
    voltage_v       DOUBLE PRECISION NOT NULL,
    current_a       DOUBLE PRECISION NOT NULL,
    power_factor    DOUBLE PRECISION NOT NULL,
    status          TEXT             NOT NULL,
    UNIQUE (meter_id, timestamp)
);

CREATE INDEX IF NOT EXISTS readings_meter_timestamp_idx ON readings (meter_id, timestamp);

CREATE TABLE IF NOT EXISTS events (
    id          BIGSERIAL PRIMARY KEY,
    meter_id    TEXT        NOT NULL REFERENCES meters(meter_id),
    timestamp   TIMESTAMPTZ NOT NULL,
    type        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS events_meter_timestamp_idx ON events (meter_id, timestamp);

-- A run is COMPLETED when detection finished, not when narration did. Narration
-- progress is per anomaly, in anomalies.explanation_status (ADR-0008).
CREATE TABLE IF NOT EXISTS analysis_runs (
    id            BIGSERIAL PRIMARY KEY,
    started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at   TIMESTAMPTZ,
    state         TEXT        NOT NULL,
    anomaly_count INT         NOT NULL DEFAULT 0,
    window_start  TIMESTAMPTZ NOT NULL,
    window_end    TIMESTAMPTZ NOT NULL,
    error         TEXT        NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS anomalies (
    id                     BIGSERIAL PRIMARY KEY,
    run_id                 BIGINT      NOT NULL REFERENCES analysis_runs(id) ON DELETE CASCADE,
    meter_id               TEXT        NOT NULL REFERENCES meters(meter_id),
    window_start           TIMESTAMPTZ NOT NULL,
    window_end             TIMESTAMPTZ NOT NULL,
    affected_reading_count INT         NOT NULL,
    type                   TEXT        NOT NULL,
    severity               TEXT        NOT NULL,
    confidence             DOUBLE PRECISION NOT NULL,
    confidence_basis       JSONB       NOT NULL,
    deviation_percent      DOUBLE PRECISION NOT NULL,
    actual_kwh             DOUBLE PRECISION NOT NULL,
    baseline_kwh           DOUBLE PRECISION NOT NULL,
    corroborating          TEXT[]      NOT NULL DEFAULT '{}',
    findings               JSONB       NOT NULL DEFAULT '[]',
    -- The episode's own hourly readings against the baseline that judged them.
    -- Persisted rather than recomputed on read, so the investigation view shows
    -- the series the detector acted on.
    deviation_series       JSONB       NOT NULL DEFAULT '[]',
    correlated_event       JSONB,
    reason                 TEXT        NOT NULL,
    recommended_action     TEXT        NOT NULL,
    detected_by            TEXT        NOT NULL,
    -- Whether reason and recommended_action were written by the deterministic
    -- template or by the narrator. Always rules until narration succeeds, because
    -- a row is never persisted without an explanation (ADR-0006).
    explanation_source     TEXT        NOT NULL DEFAULT 'rules',
    explanation_status     TEXT        NOT NULL DEFAULT 'PENDING',
    status                 TEXT        NOT NULL DEFAULT 'OPEN',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- One row per episode per run. Re-running the detector over the same window
    -- writes a new run, and this keeps a retried episode from doubling up inside
    -- it.
    UNIQUE (run_id, meter_id, window_start)
);

CREATE INDEX IF NOT EXISTS anomalies_run_idx ON anomalies (run_id);
CREATE INDEX IF NOT EXISTS anomalies_meter_idx ON anomalies (meter_id, window_start DESC);
