package analysis

import (
	"sort"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// sortReadingsByTime orders readings oldest first. Ties break on meter code so
// the ordering is total and reproducible.
func sortReadingsByTime(readings []catalog.Reading) {
	sort.SliceStable(readings, func(i, j int) bool {
		if !readings[i].Timestamp.Equal(readings[j].Timestamp) {
			return readings[i].Timestamp.Before(readings[j].Timestamp)
		}
		return readings[i].MeterCode < readings[j].MeterCode
	})
}

// timestampLayout is the wire format for every timestamp the API returns.
const timestampLayout = "2006-01-02T15:04:05Z07:00"
