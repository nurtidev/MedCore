package domain

import "time"

// PatientInfo — данные пациента из eGov API.
type PatientInfo struct {
	IIN        string
	FirstName  string
	LastName   string
	MiddleName string
	BirthDate  time.Time
	Gender     string
	Address    string
	IsValid    bool // ИИН существует в базе
}
