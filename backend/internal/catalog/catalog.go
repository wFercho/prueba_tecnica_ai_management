// Package catalog holds the vocabulary of things the platform monitors: meters,
// readings and the operational events that can account for an episode.
//
// The names here are the ones in CONTEXT.md. They are not interchangeable with the
// analysis vocabulary in package analysis.
package catalog

import "time"

// MeterCode is the stable human-readable code such as M-101 that keys everything else.
type MeterCode string

// Meter is an electrical meter under monitoring.
type Meter struct {
	ID        int64     `json:"id"`
	Code      MeterCode `json:"meter_id"`
	Name      string    `json:"name"`
	Location  string    `json:"location"`
	CreatedAt time.Time `json:"created_at"`
}

// Reading is one hour of measured values for one meter.
//
// IngestedStatus is the value the source supplied alongside the reading, preserved
// verbatim. It is evidence about the ingest pipeline, never a verdict about the
// reading: all 4,032 delivered rows carry OK, including the corrupt ones.
type Reading struct {
	MeterCode      MeterCode
	Timestamp      time.Time
	ConsumptionKWh float64
	VoltageV       float64
	CurrentA       float64
	PowerFactor    float64
	IngestedStatus string
}

// EventType classifies an operational event.
type EventType string

const (
	// EventTypeUnknown is a row that explicitly records the absence of a known
	// cause. It is not an explanation: it is evidence that nothing explains.
	EventTypeUnknown EventType = "UNKNOWN"
	// EventTypeOperationalChange is a known change in how a meter is used, such as
	// a production line starting.
	EventTypeOperationalChange EventType = "OPERATIONAL_CHANGE"
	// EventTypeScheduledOutage is planned downtime at a meter.
	EventTypeScheduledOutage EventType = "SCHEDULED_OUTAGE"
	// EventTypeMaintenance is a maintenance intervention.
	EventTypeMaintenance EventType = "MAINTENANCE"
	// EventTypeEquipmentFault is a reported physical fault.
	EventTypeEquipmentFault EventType = "EQUIPMENT_FAULT"
	// EventTypeDataQuality is a reported fault of the metering or its reporting.
	EventTypeDataQuality EventType = "DATA_QUALITY"
)

// OperationalEvent is something known to have happened at a meter that can account
// for an episode. Used to explain, or to excuse, an episode.
type OperationalEvent struct {
	ID          int64     `json:"id"`
	MeterCode   MeterCode `json:"meter_id"`
	Timestamp   time.Time `json:"timestamp"`
	Type        EventType `json:"type"`
	Description string    `json:"description"`
}

// Explains reports whether this event can account for a change in consumption.
//
// A row of type UNKNOWN is a positive statement that no cause was reported, so it
// never explains anything; treating it as an explanation would excuse the one
// anomaly in the dataset that has no excuse.
func (e OperationalEvent) Explains() bool {
	switch e.Type {
	case EventTypeUnknown:
		return false
	case EventTypeOperationalChange, EventTypeScheduledOutage, EventTypeMaintenance,
		EventTypeEquipmentFault, EventTypeDataQuality:
		return true
	default:
		return false
	}
}

// AnomalyStatus is where an anomaly sits in the investigation workflow. It is the
// operator's decision and is never written by the detector.
type AnomalyStatus string

const (
	StatusOpen AnomalyStatus = "OPEN"
)

// ExplanationSource is who wrote an anomaly's prose.
type ExplanationSource string

const (
	// SourceRules is the deterministic template. It is what every anomaly carries
	// the moment it is persisted, so a row never renders without an explanation
	// even if the narrator is unavailable (ADR-0006).
	SourceRules ExplanationSource = "rules"
	// SourceLLM is the narrator's prose, which only ever replaces the template
	// after the deterministic result is already durable.
	SourceLLM ExplanationSource = "llm"
)

// ExplanationStatus is whether an anomaly's prose is the template, the
// narrator's, or neither arrived.
type ExplanationStatus string

const (
	ExplanationPending ExplanationStatus = "PENDING"
	ExplanationReady   ExplanationStatus = "READY"
	ExplanationFailed  ExplanationStatus = "FAILED"
)
