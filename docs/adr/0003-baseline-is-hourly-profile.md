# Baseline is a per-meter, per-hour-of-day trailing profile

A meter's baseline is its **expected consumption at a given hour of day**, computed over a trailing window that **excludes the candidate episode**. A daily aggregation is derived on top of it for the dashboard.

## Context

The spec uses "baseline" in §7, §12 and §13 and never defines it. §6's variation figures are illustrative and not reproducible from the delivered data — M-104's +47.6% is close, but M-101's +2.5% is not derivable by any method (the measured value is −0.1%).

Three definitions were measured against the delivered CSVs:

- **Whole-period mean.** Trivial, and produces the nonsensical figures §6 shows. Also useless here: all 12 meters share one normalised 24-hour shape (pairwise r = 0.95–0.999) and differ only in absolute magnitude, so a single mean conflates the diurnal cycle with the level.
- **Change-point relative** (mean before vs mean after the episode). Reproduces M-104 and M-109 well, but is **circular**: it presupposes knowing where the change is, and the baseline is an *input* to finding it.
- **Per-hour-of-day trailing profile.** Hour-resolved, so it does not confuse the 06:00/08:00 ramp with a step.

Measured: a naive trailing-mean baseline produces 4–9 false hits per normal meter at the diurnal ramp, while a daily-sum threshold at 20% yields **zero** false positives across the 8 normal meters (max swing 4.63%) and still reproduces M-104's +46.5% step and M-109's +126.9% single-interval jump.

## Consequences

Hour-of-day resolution is not a refinement — it is what makes the "zero false positives on normal meters" claim true, and that claim is the substance of §18's 15 points for not treating M-106 as real.

Excluding the episode from its own baseline window is what stops a sustained change from redefining "normal" and going silent. It also means the baseline must be recomputed per candidate, not cached once per meter.
