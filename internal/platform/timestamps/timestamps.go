package timestamps

import "time"

type Clock interface {
	Now() time.Time
}

type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time {
	return f().UTC()
}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

func Now() time.Time {
	return SystemClock{}.Now()
}

func RFC3339(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
