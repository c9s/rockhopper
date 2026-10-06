package rockhopper

import "time"

// Profile records the start, end, and duration of a profiled operation.
type Profile struct {
	name               string
	startTime, endTime time.Time
	duration           time.Duration
}

func startProfile(name string) *Profile {
	return &Profile{name: name, startTime: time.Now()}
}

// Stop records the end time and duration of the operation.
func (p *Profile) Stop() {
	p.endTime = time.Now()
	p.duration = p.endTime.Sub(p.startTime)
}

func (p *Profile) String() string {
	if p.duration > 0 {
		return p.duration.String()
	}

	return p.startTime.String()
}
