package analysis

import (
	"sort"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// Analyze is the detector's whole public surface: readings and reported events in,
// classified anomalies out.
//
// Everything a caller needs to know about how detection works is behind this one
// function. Baselines, episode grouping, event correlation, classification and
// confidence are steps inside it, not a pipeline a caller has to know the order
// of — a caller that assembled them itself could put them in the wrong order and
// get quietly different answers.
func Analyze(readings []catalog.Reading, events []catalog.OperationalEvent, cfg DetectorConfig) []Anomaly {
	byMeter := map[catalog.MeterCode][]catalog.Reading{}
	for _, reading := range readings {
		byMeter[reading.MeterCode] = append(byMeter[reading.MeterCode], reading)
	}

	meters := make([]catalog.MeterCode, 0, len(byMeter))
	for meter := range byMeter {
		meters = append(meters, meter)
	}
	// Map iteration order is random, and an unordered result would make the
	// dashboard's ranking flicker between identical runs.
	sort.Slice(meters, func(i, j int) bool { return meters[i] < meters[j] })

	var anomalies []Anomaly
	for _, meter := range meters {
		meterReadings := byMeter[meter]
		episodes := FindConsumptionEpisodes(meterReadings, cfg)
		episodes = append(episodes, FindDataQualityEpisodes(meterReadings, cfg)...)
		for _, episode := range episodes {
			Correlate(events, &episode, cfg)
			anomalies = append(anomalies, Classify(episode, cfg))
		}
	}
	return anomalies
}
