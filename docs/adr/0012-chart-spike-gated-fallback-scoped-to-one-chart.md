# Recharts is the first choice for the meter chart, gated by an early spike

The project chooses Recharts as its first charting library, while the PDF requires only effective graphs, not a named package. Prototype the M-109 detail chart with Recharts **before** migrating the rest of the dashboard. If Recharts fails the pre-agreed visual criteria, keep or adapt hand-rolled SVG **only for this chart** and document why; the rest of the chosen frontend stack remains unchanged.

## Context

The meter detail screen is the visual argument of the demo and sits on the critical path for the 20 Frontend/UX points. The concrete requirement is a consumption series, baseline overlay and shaded episode window across a 14-day time axis, with the episode band appearing only **after** analysis. The current frontend uses SVG, so adopting Recharts is a deliberate project choice (ADR-0015), not a claim that the PDF disallows SVG.

Recharts supports multiple lines, tooltips and bounded reference areas over an X axis; the uncertainty is whether that combination stays readable for this exact 58-hour episode and usable with a keyboard. The cost to control is unreadable evidence, not divergence from a prescribed library.

## Consequences

The spike is bounded to the detail chart. A fallback is allowed only if a criterion fails, not because SVG is fashionable; do not build the same chart twice just in case. Other charts can stay on Recharts, since a single difficult episode chart does not justify two complete charting systems.

The four gates are: the anomaly band is distinguishable from the baseline without occluding it; the **Spanish** tooltip shows hour / actual / baseline / deviation; the day-grouped X axis is legible across 14 days; and M-109's 58-hour window reads as a continuous block rather than noise. Check keyboard access and a textual/table alternative to the visual evidence regardless of library.

The spike lands before the rest of the dashboard migration, so a chart-library limitation is found while a local fallback is still cheap.
