package event

import (
	"math"
	"math/rand"
	"time"

	"github.com/go-resty/resty/v2"
)

type Target struct {
	client             *resty.Client
	writePeriodSeconds float64
	windowSeconds      float64
	rateClasses        []*rateClass
	stop               chan bool
	lastEvent          time.Time
	events             []*Event
}

func (target *Target) AddRateClass(grantRate float64, vals ...any) {
	if len(vals)%2 != 0 {
		panic(vals)
	}

	erc := &rateClass{
		grantRate: grantRate * target.windowSeconds,
		criteria:  map[string]any{},
	}

	for i := 0; i < len(vals); i += 2 {
		erc.criteria[vals[i].(string)] = vals[i+1]
	}

	target.rateClasses = append(target.rateClasses, erc)
}

func (target *Target) writeEvent(ev *Event) {
	now := time.Now()
	secondsSinceLastEvent := now.Sub(target.lastEvent).Seconds()
	target.lastEvent = now

	// Example:
	//   windowSeconds = 100
	//   secondsSinceLastEvent = 25
	//   eventRateMultiplier = 0.75
	eventRateMultiplier := (target.windowSeconds - secondsSinceLastEvent) / target.windowSeconds

	maxProb := 0.0

	for _, erc := range target.rateClasses {
		if !erc.match(ev) {
			continue
		}

		erc.eventRate++
		erc.eventRate *= eventRateMultiplier

		classProb := erc.grantRate / erc.eventRate
		maxProb = math.Max(maxProb, classProb)
	}

	if maxProb <= 0.0 || rand.Float64() > maxProb { //nolint:gosec
		return
	}

	ev2 := *ev
	ev2.SampleRate = int64(math.Max(math.Round(1.0/maxProb), 1.0))
	target.events = append(target.events, &ev2)
}
