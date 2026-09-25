package analysis

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// explain is the deterministic explanation every anomaly is persisted with.
//
// It is written before anything touches the network, and it is the floor of what
// the product promises: a reader never sees an anomaly without prose, even if the
// narrator is slow, rate-limited or absent (ADR-0006). The narrator's job is to
// say this better, not to supply it.
//
// The rule it follows is cite or stay silent. Every claim — how far, how long,
// what else moved, what was reported — is a number or a quoted event that the
// reader can check against the detail view. Nothing is asserted that the
// evidence does not carry, and where a cause is unknown the template says so
// instead of reaching for the likeliest-sounding one.
func explain(a Anomaly) (reason, action string) {
	switch a.Type {
	case AnomalyDataQuality:
		return explainDataQuality(a), escalate(a)
	case AnomalyFalsePositive:
		return explainExplained(a, false), escalate(a)
	case AnomalyExplainable:
		return explainExplained(a, true), escalate(a)
	default:
		return explainReal(a), escalate(a)
	}
}

// window renders the episode's period for prose.
func window(a Anomaly) string {
	return fmt.Sprintf("%s to %s",
		a.WindowStart.Format("2006-01-02 15:04 MST"),
		a.WindowEnd.Format("2006-01-02 15:04 MST"))
}

func explainReal(a Anomaly) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Between %s, %s consumed %.2f kWh against an expected %.2f kWh for the same hours, %.1f%% %s its own baseline, across %d hourly readings.",
		window(a), a.MeterCode, a.ActualKWh, a.BaselineKWh, abs(a.DeviationPercent), direction(a.DeviationPercent), a.AffectedReadings)

	b.WriteString(" " + corroborationClause(a))
	b.WriteString(" " + eventClause(a))
	b.WriteString(" " + confidenceClause(a))
	return b.String()
}

// explainExplained covers both EXPLAINABLE and FALSE_POSITIVE. The difference
// between them is not the size of the change but what follows from it: a
// confirmed change is recorded and confirmed, a fully accounted-for one is
// dismissed.
func explainExplained(a Anomaly, real bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Between %s, %s changed %.1f%% %s its own baseline (%.2f kWh against %.2f kWh expected, %d hourly readings).",
		window(a), a.MeterCode, abs(a.DeviationPercent), direction(a.DeviationPercent),
		a.ActualKWh, a.BaselineKWh, a.AffectedReadings)

	if event := a.CorrelatedEvent; event != nil {
		fmt.Fprintf(&b, " The reported event at %s — %q (%s) — falls in this window.",
			event.Timestamp.Format("2006-01-02 15:04 MST"), event.Description, event.Type)
	} else {
		b.WriteString(" A reported event falls in this window.")
	}

	if real {
		b.WriteString(" The change is real and the reported event accounts for it, so it needs confirming rather than escalating.")
		b.WriteString(" " + corroborationClause(a))
	} else {
		// No corroboration clause here, deliberately. Its job everywhere else is
		// to support "this is a real change in load rather than a reporting
		// fault", and during an outage the current falling *is* that fault-free
		// drop — citing it would argue for a real change the classification has
		// just said there is no case for.
		b.WriteString(" A known outage accounts for the whole change, so there is nothing to investigate: the meter was reporting correctly at a lower load, and its current fell with the load rather than contradicting it.")
	}
	b.WriteString(" " + confidenceClause(a))
	return b.String()
}

func explainDataQuality(a Anomaly) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Between %s, %s reported %.1f%% change in consumption — essentially flat — while %d of its hourly readings were not physically consistent.",
		window(a), a.MeterCode, a.DeviationPercent, a.AffectedReadings)

	if offenders := offendersByVariable(a.Findings); len(offenders) > 0 {
		b.WriteString(" The offending values were:")
		for _, offender := range offenders {
			fmt.Fprintf(&b, " %s read %.2f against an expected %.2f (%d readings);",
				spellOut(string(offender.variable)), offender.value, offender.expected, offender.count)
		}
		// The trailing semicolon reads as a list, so close it into a sentence.
		text := strings.TrimSuffix(b.String(), ";")
		b.Reset()
		b.WriteString(text + ".")
	}

	if event := a.CorrelatedEvent; event != nil {
		fmt.Fprintf(&b, " This matches the reported event at %s — %q.",
			event.Timestamp.Format("2006-01-02 15:04 MST"), event.Description)
	}
	b.WriteString(" The consumption it reports over this window cannot be trusted, so the meter and its reporting need checking before the figures are used.")
	b.WriteString(" " + confidenceClause(a))
	return b.String()
}

// offender is one variable's worst finding, summarised for prose.
type offender struct {
	variable Variable
	value    float64
	expected float64
	count    int
}

// offendersByVariable summarises the findings by variable, worst departure first,
// so the prose names the most wrong measurement rather than the first one
// encountered.
func offendersByVariable(findings []DataQualityFinding) []offender {
	byVariable := map[Variable]*offender{}
	for _, finding := range findings {
		existing, ok := byVariable[finding.Variable]
		if !ok {
			byVariable[finding.Variable] = &offender{
				variable: finding.Variable,
				value:    finding.Value,
				expected: finding.Expected,
				count:    1,
			}
			continue
		}
		existing.count++
		if finding.RelativeDeviation > abs(existing.value-existing.expected)/abs(existing.expected) {
			existing.value = finding.Value
			existing.expected = finding.Expected
		}
	}

	out := make([]offender, 0, len(byVariable))
	for _, o := range byVariable {
		out = append(out, *o)
	}
	sort.Slice(out, func(i, j int) bool {
		left := abs(out[i].value-out[i].expected) / abs(out[i].expected)
		right := abs(out[j].value-out[j].expected) / abs(out[j].expected)
		if left != right {
			return left > right
		}
		return out[i].variable < out[j].variable
	})
	return out
}

// corroborationClause names what else moved. It is the clause that distinguishes
// a real load change from a metering artefact, so it is never omitted when there
// is something to say.
func corroborationClause(a Anomaly) string {
	// The corroborating list carries the detector's variable names, which are
	// identifiers and belong in the API. Prose gets words.
	others := make([]string, 0, len(a.Corroborating))
	for _, name := range a.Corroborating {
		if name == "consumption" {
			continue
		}
		others = append(others, spellOut(name))
	}
	if len(a.Corroborating) == 0 {
		return "Nothing else moved with it, so the change is visible in consumption alone."
	}
	if len(others) == 0 {
		return "Only consumption moved; the meter's electrical values did not follow it."
	}
	return capitalise(fmt.Sprintf("%s moved with it, which is what a real change in load looks like rather than a reporting fault.",
		joinWithAnd(others)))
}

// spellOut turns a detector variable name into the word a reader expects.
func spellOut(name string) string {
	switch name {
	case "power_factor":
		return "power factor"
	case "energy_balance":
		return "the energy balance"
	case "current":
		return "current"
	case "voltage":
		return "voltage"
	default:
		return strings.ReplaceAll(name, "_", " ")
	}
}

func eventClause(a Anomaly) string {
	switch {
	case a.CorrelatedEvent == nil:
		return "No operational event was reported for this meter in this window, so nothing on record accounts for the change."
	case a.CorrelatedEvent.Explains:
		return fmt.Sprintf("The reported event %q is consistent with the change, though it does not by itself establish the cause.",
			a.CorrelatedEvent.Description)
	case a.CorrelatedEvent.Type == string(catalog.EventTypeUnknown):
		return "Somebody looked and reported no operational event, which is positive evidence that nothing on record accounts for the change."
	default:
		return fmt.Sprintf("The reported event %q is not the cause.", a.CorrelatedEvent.Description)
	}
}

func confidenceClause(a Anomaly) string {
	return fmt.Sprintf("Confidence %.2f, from deviation %.2f, event match %.2f, corroboration %.2f and persistence %.2f — an evidence score, not a probability.",
		a.Confidence, a.ConfidenceBasis.Deviation, a.ConfidenceBasis.EventMatch,
		a.ConfidenceBasis.Corroboration, a.ConfidenceBasis.Persistence)
}

// escalate is the recommended action for each classification. Each one names who
// should do what, because "investigate" is not an instruction.
func escalate(a Anomaly) string {
	switch a.Type {
	case AnomalyDataQuality:
		return "Check the meter and its reporting before using these consumption figures: ask the metering team to verify the installation and pull the raw register for this window."
	case AnomalyFalsePositive:
		return "No escalar: nothing needs to be done. The scheduled outage accounts for this window, and it is recorded so that the drop is visibly examined rather than silently missing."
	case AnomalyExplainable:
		return "Confirm with the operations team that the reported change is the whole story, then acknowledge and close it. Escalate only if the confirmed change does not match the reported one."
	default:
		return "Investigate: confirm with the operations team whether a load change was intended, and if none was, check for unmetered or faulty equipment drawing the additional load."
	}
}

// capitalise starts a clause as a sentence, because these clauses are appended to
// prose rather than formatted into a template.
func capitalise(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

func direction(deviation float64) string {
	if deviation >= 0 {
		return "above"
	}
	return "below"
}

func joinWithAnd(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	case 2:
		return values[0] + " and " + values[1]
	default:
		return strings.Join(values[:len(values)-1], ", ") + " and " + values[len(values)-1]
	}
}
