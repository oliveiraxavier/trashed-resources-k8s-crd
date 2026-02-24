package utils

import (
	"time"
)

const timeFormat = time.RFC3339

// DateTime struct to encapsulate time functionality
type DateTime struct {
	Time time.Time `json:"time"`
}

var GetTimeRemaining = func(futureDateToCompare string) time.Duration {
	parsedTime, _ := time.Parse(timeFormat, futureDateToCompare)
	return parsedTime.Sub(Now().Time) // return time.Until(parsedTime)
}

// Now initializes DateTime with the current time
func Now() DateTime {
	dt := DateTime{Time: time.Now()}
	return dt.AddSeconds(0)
}

// AddMinutes adds minutes to the current DateTime
func (dt DateTime) AddHours(hours int64) DateTime {
	return DateTime{Time: dt.Time.Add(time.Duration(hours) * time.Hour)}
}

// AddMinutes adds minutes to the current DateTime
func (dt DateTime) AddMinutes(minutes int64) DateTime {
	return DateTime{Time: dt.Time.Add(time.Duration(minutes) * time.Minute)}
}

// AddSeconds adds seconds to the current DateTime
func (dt DateTime) AddSeconds(seconds int64) DateTime {
	return DateTime{Time: dt.Time.Add(time.Duration(seconds) * time.Second)}
}

func NowIsAfterOrEqualCompareDate(dateToCompare string) bool {
	parsedTime, _ := time.Parse(timeFormat, dateToCompare)
	return !Now().Time.Before(parsedTime)
}

// ToString returns the default string representation
func (dt DateTime) ToString() string {
	return dt.Format(timeFormat)
}

// Format formats the datetime in a given layout
func (dt DateTime) Format(layout string) string {
	return dt.Time.Format(layout)
}
