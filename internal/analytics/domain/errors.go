package domain

import "errors"

var (
	ErrInvalidPeriod   = errors.New("invalid period format, expected YYYY-MM")
	ErrInvalidGrouping = errors.New("invalid grouping, expected day|week|month")
	ErrClinicNotFound  = errors.New("clinic not found")
	ErrNoData          = errors.New("no data for the requested period")
)
