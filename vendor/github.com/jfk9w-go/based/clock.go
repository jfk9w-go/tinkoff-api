package based

import "time"

// Clock is a simple clock interface.
type Clock interface {

	// Now returns the current time according to this Clock.
	Now() time.Time
}

// ClockFunc is a functional Clock adapter.
type ClockFunc func() time.Time

func (fn ClockFunc) Now() time.Time {
	return fn()
}

// StandardClock provides the current local time using time.Now.
var StandardClock Clock = ClockFunc(time.Now)
