package rockhopper

import "time"

type Profile struct {
	name               string
	startTime, endTime time.Time
	duration           time.Duration
}

func startProfile(name string) *Profile {
	return &Profile{name: name, startTime: time.Now()}
}

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
