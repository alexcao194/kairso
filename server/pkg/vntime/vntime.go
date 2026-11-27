// Package vntime holds Vietnam's time zone, loaded once (time.LoadLocation reads the zone
// database from disk on every call).
package vntime

import "time"

// Location is Asia/Ho_Chi_Minh, or a fixed UTC+7 zone when the zone database is missing.
var Location = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Ho_Chi_Minh"); err == nil {
		return loc
	}
	return time.FixedZone("ICT", 7*3600)
}()

// ParseDate parses YYYY-MM-DD as midnight in Vietnam.
func ParseDate(value string) (time.Time, error) {
	return time.ParseInLocation(time.DateOnly, value, Location)
}
