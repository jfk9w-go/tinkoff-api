package based

import "time"

type Clock interface {
	Now() time.Time
}

type ClockFunc func() time.Time

func (fn ClockFunc) Now() time.Time {
	return fn()
}

var StandardClock Clock = ClockFunc(time.Now)
