package domain

import "time"

type EventType string

const (
	EventAppointmentCreated   EventType = "appointment.created"
	EventAppointmentCompleted EventType = "appointment.completed"
	EventAppointmentNoShow    EventType = "appointment.no_show"
	EventAppointmentCancelled EventType = "appointment.cancelled"
	EventPaymentCompleted     EventType = "payment.completed"
	EventPaymentFailed        EventType = "payment.failed"
	EventLabResultReceived    EventType = "lab_result.received"
	EventUserLogin            EventType = "user.login"
)

// ClinicEvent is the base analytics unit — written from Kafka, stored in ClickHouse.
type ClinicEvent struct {
	EventID   string    // UUID
	ClinicID  string
	DoctorID  string
	PatientID string
	EventType EventType
	Amount    float64 // for payment events
	Currency  string
	CreatedAt time.Time
	Metadata  string // JSON
}
