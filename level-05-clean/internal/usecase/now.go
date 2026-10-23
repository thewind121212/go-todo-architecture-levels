package usecase

import "time"

// nowUTC is called once per Execute, so a todo change and its activities share one timestamp.
// It is truncated to milliseconds because Mongo stores milliseconds; both adapters then
// round-trip identical values.
func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}
