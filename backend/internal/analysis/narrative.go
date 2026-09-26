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
	return fmt.Sprintf("del %s al %s",
		a.WindowStart.Format("2006-01-02 15:04 MST"),
		a.WindowEnd.Format("2006-01-02 15:04 MST"))
}

func explainReal(a Anomaly) string {
	var b strings.Builder
	fmt.Fprintf(&b, "En el periodo %s, %s consumió %.2f kWh frente a %.2f kWh esperados para las mismas horas: %.1f%% %s su línea base propia durante %d lecturas horarias.",
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
	fmt.Fprintf(&b, "En el periodo %s, %s cambió %.1f%% %s su línea base propia (%.2f kWh frente a %.2f kWh esperados, %d lecturas horarias).",
		window(a), a.MeterCode, abs(a.DeviationPercent), direction(a.DeviationPercent),
		a.ActualKWh, a.BaselineKWh, a.AffectedReadings)

	if event := a.CorrelatedEvent; event != nil {
		fmt.Fprintf(&b, " El reporte de %s del %s coincide con este periodo.",
			describeEvent(event.Type), event.Timestamp.Format("2006-01-02 15:04 MST"))
	} else {
		b.WriteString(" Hay un reporte de contexto en este periodo.")
	}

	if real {
		b.WriteString(" El cambio es real, pero el evento reportado lo explica: conviene confirmarlo antes de escalar.")
		b.WriteString(" " + corroborationClause(a))
	} else {
		// No corroboration clause here, deliberately. Its job everywhere else is
		// to support "this is a real change in load rather than a reporting
		// fault", and during an outage the current falling *is* that fault-free
		// drop — citing it would argue for a real change the classification has
		// just said there is no case for.
		b.WriteString(" La parada programada explica la caída completa; la corriente bajó con la carga y no hay indicios de fallo de medición. No se requiere investigar esta desviación.")
	}
	b.WriteString(" " + confidenceClause(a))
	return b.String()
}

func explainDataQuality(a Anomaly) string {
	var b strings.Builder
	fmt.Fprintf(&b, "En el periodo %s, %s registró una variación de consumo de %.1f%%, prácticamente estable, pero %d lecturas horarias presentaron incoherencias físicas.",
		window(a), a.MeterCode, a.DeviationPercent, a.AffectedReadings)

	if offenders := offendersByVariable(a.Findings); len(offenders) > 0 {
		b.WriteString(" Los valores incompatibles fueron:")
		for _, offender := range offenders {
			fmt.Fprintf(&b, " %s registró %.2f frente a %.2f esperados (%d lecturas);",
				spellOut(string(offender.variable)), offender.value, offender.expected, offender.count)
		}
		// The trailing semicolon reads as a list, so close it into a sentence.
		text := strings.TrimSuffix(b.String(), ";")
		b.Reset()
		b.WriteString(text + ".")
	}

	if event := a.CorrelatedEvent; event != nil {
		fmt.Fprintf(&b, " El reporte de %s del %s aporta contexto, pero la evidencia procede de las lecturas.",
			describeEvent(event.Type), event.Timestamp.Format("2006-01-02 15:04 MST"))
	}
	b.WriteString(" Hay que revisar el medidor y su registro antes de usar estas cifras de consumo.")
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
		return "Ninguna otra variable acompañó el cambio; solo se observa en el consumo."
	}
	if len(others) == 0 {
		return "Solo cambió el consumo; las variables eléctricas no lo acompañaron."
	}
	return capitalise(fmt.Sprintf("También cambiaron %s; esta corroboración es compatible con un cambio real de carga y no solo con un fallo del reporte.",
		joinWithAnd(others)))
}

// spellOut turns a detector variable name into the word a reader expects.
func spellOut(name string) string {
	switch name {
	case "power_factor":
		return "el factor de potencia"
	case "energy_balance":
		return "el balance energético"
	case "current":
		return "la corriente"
	case "voltage":
		return "el voltaje"
	default:
		return strings.ReplaceAll(name, "_", " ")
	}
}

func eventClause(a Anomaly) string {
	switch {
	case a.CorrelatedEvent == nil:
		return "No se registró ningún evento operativo para este medidor en el periodo; ningún reporte explica el cambio."
	case a.CorrelatedEvent.Explains:
		return "El evento reportado es coherente con la desviación, aunque por sí solo no demuestra la causa."
	case a.CorrelatedEvent.Type == string(catalog.EventTypeUnknown):
		return "Se informó explícitamente que no hubo evento operativo; ningún evento conocido explica la desviación."
	default:
		return "El evento reportado no explica la desviación."
	}
}

func confidenceClause(a Anomaly) string {
	return fmt.Sprintf("Confianza %.2f: desviación %.2f, coincidencia de evento %.2f, corroboración %.2f y persistencia %.2f; es una puntuación de evidencia, no una probabilidad.",
		a.Confidence, a.ConfidenceBasis.Deviation, a.ConfidenceBasis.EventMatch,
		a.ConfidenceBasis.Corroboration, a.ConfidenceBasis.Persistence)
}

// escalate is the recommended action for each classification. Each one names who
// should do what, because "investigate" is not an instruction.
func escalate(a Anomaly) string {
	switch a.Type {
	case AnomalyDataQuality:
		return "Solicitar al equipo de medición la revisión del medidor y el registro original de este periodo antes de utilizar las cifras de consumo."
	case AnomalyFalsePositive:
		return "No escalar. Registrar la parada programada como explicación de la caída y conservar el episodio examinado."
	case AnomalyExplainable:
		return "Confirmar con operaciones que el cambio reportado explica todo el periodo. Escalar solo si las lecturas no coinciden con la operación confirmada."
	default:
		return "Investigar con operaciones si el aumento de carga estaba previsto; de no ser así, revisar equipos defectuosos o consumos no medidos."
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
		return "por encima de"
	}
	return "por debajo de"
}

func joinWithAnd(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	case 2:
		return values[0] + " y " + values[1]
	default:
		return strings.Join(values[:len(values)-1], ", ") + " y " + values[len(values)-1]
	}
}

func describeEvent(kind string) string {
	switch kind {
	case "SCHEDULED_OUTAGE":
		return "parada programada"
	case "OPERATIONAL_CHANGE":
		return "cambio de producción"
	case "DATA_QUALITY":
		return "calidad de datos"
	case "UNKNOWN":
		return "ausencia de evento"
	default:
		return "contexto operativo"
	}
}
