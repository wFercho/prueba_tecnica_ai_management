-- Events become importable, not just readable.
--
-- The seeder is run by hand, on boot in development, and by anyone following the
-- README, so importing the same event file twice must not double the events the
-- detector correlates against. A re-import that carries a fuller description keeps
-- the newer wording.
--
-- An event's identity is its meter, the hour it happened and its type: the same
-- event reported twice is one event, while two different events at the same hour
-- are two.

CREATE UNIQUE INDEX IF NOT EXISTS events_identity_idx
    ON events (meter_id, timestamp, type);
