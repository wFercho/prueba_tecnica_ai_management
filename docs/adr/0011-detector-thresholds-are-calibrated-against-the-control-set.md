# Detector thresholds live in one config, calibrated against the control set

Every tunable the detectors need — the MAD multiplier `k`, the level-shift deviation threshold, the energy-balance tolerance, the four confidence weights — lives in a single `DetectorConfig` struct. Each default is documented inline with the measured range from the delivered dataset that justifies it. No threshold lives anywhere else.

## Context

The *methods* are settled (ADR-0003 for the baseline, the robust-bounds-plus-corroboration approach for M-112), but nothing fixed where the numbers live or what guards them. These are the most dangerous constants in the repository: too tight and the eight normal meters begin firing, too loose and M-109 or M-112 slips past. Left scattered, they get adjusted in whichever file someone is editing that week.

## Consequences

The concrete values get set during implementation against the data, not guessed now — but they are set **once**, in one place, with a stated measured reason, and the negative-control test is the gate on all of them.

That test is the point. The obvious way to silence a false positive is to loosen the threshold that fired it, and that is precisely the change the control set exists to catch: it fails the moment any of the eight normal meters produces an anomaly. Those eight meters have a maximum day-over-day swing of 4.63%, which is what makes them a usable control. Counterfactual tests with changed meter IDs and timestamps, without the M-112 quality report, and with intermittent episodes separated by more than three hours check that the algorithm is not memorising this dataset.

The defaults are calibrated to one 14-day dataset, so their accuracy on different distributions is not guaranteed; the detector still accepts other compatible CSVs without special-case meter IDs. A meter with insufficient history for its own baseline is reported as such rather than classified `HEALTHY` or assigned an invented consumption anomaly.
