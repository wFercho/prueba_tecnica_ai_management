package catalog

import (
	"fmt"
	"sort"
	"time"
)

// FromReadings derives the meter catalogue from the readings that mention each meter.
//
// The delivered dataset has readings and events but no catalogue of its own, so a
// meter exists here because something reported a reading for it. The name is derived
// from the code and the location is left empty rather than invented: a dashboard that
// says "Planta 1" is making a claim no data supports, and an operator who later
// learns it was invented has to distrust the rest of the page.
func FromReadings(readings []Reading) []Meter {
	seen := map[MeterCode]bool{}
	var meters []Meter
	for _, reading := range readings {
		if seen[reading.MeterCode] {
			continue
		}
		seen[reading.MeterCode] = true
		meters = append(meters, Meter{
			Code: reading.MeterCode,
			Name: "Medidor " + string(reading.MeterCode),
		})
	}
	sort.Slice(meters, func(i, j int) bool { return meters[i].Code < meters[j].Code })
	return meters
}

// Validate refuses readings that cannot be stored, or that would quietly poison the
// detector's arithmetic.
//
// It reports the one-based row number and the field, because a CSV with a mistake in
// row 2,000 needs an answer an operator can act on rather than a rejected file.
//
// Only values that are structurally impossible are refused. M-112's readings are the
// reason this line is drawn where it is: every one of them is individually possible —
// the power factors sit inside 0..1 — and the fault is that the four values disagree
// with each other. That is the detector's case to find, and a validator that rejected
// them here would make the fourth delivered finding undetectable.
func Validate(readings []Reading) error {
	for i, reading := range readings {
		switch {
		case reading.MeterCode == "":
			return rowError(i, "meter_id", "it is empty")
		case reading.Timestamp.IsZero():
			return rowError(i, "timestamp", "it is empty")
		case reading.IngestedStatus == "":
			return rowError(i, "status", "the source reported no status")
		case reading.PowerFactor < 0 || reading.PowerFactor > 1:
			return rowError(i, "power_factor",
				fmt.Sprintf("%g is outside 0..1, which no power factor can be", reading.PowerFactor))
		case reading.ConsumptionKWh < 0:
			return rowError(i, "consumption_kwh",
				fmt.Sprintf("%g is negative, which no metered hour can be", reading.ConsumptionKWh))
		case reading.VoltageV < 0:
			return rowError(i, "voltage_v",
				fmt.Sprintf("%g is negative", reading.VoltageV))
		case reading.CurrentA < 0:
			return rowError(i, "current_a",
				fmt.Sprintf("%g is negative", reading.CurrentA))
		}
	}
	return nil
}

// ValidateEvents refuses events that cannot be stored, on the same terms.
func ValidateEvents(events []OperationalEvent) error {
	for i, event := range events {
		switch {
		case event.MeterCode == "":
			return fmt.Errorf("event row %d: meter_id is empty", i+1)
		case event.Timestamp.IsZero():
			return fmt.Errorf("event row %d: timestamp is empty", i+1)
		case event.Type == "":
			return fmt.Errorf("event row %d: type is empty", i+1)
		}
	}
	return nil
}

func rowError(index int, field, problem string) error {
	return fmt.Errorf("reading row %d: %s: %s", index+1, field, problem)
}

// Window returns the earliest and latest timestamp in the readings, and whether there
// were any. A caller importing a file needs both, and neither is worth a second pass.
func Window(readings []Reading) (from, to time.Time, ok bool) {
	for _, reading := range readings {
		if reading.Timestamp.IsZero() {
			continue
		}
		if !ok {
			from, to, ok = reading.Timestamp, reading.Timestamp, true
			continue
		}
		if reading.Timestamp.Before(from) {
			from = reading.Timestamp
		}
		if reading.Timestamp.After(to) {
			to = reading.Timestamp
		}
	}
	return from, to, ok
}
