package event

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"log"
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
	done               chan bool
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

func (target *Target) close() {
	close(target.stop)
	<-target.done
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

func (target *Target) flushLoop(c *Client) {
	defer close(target.done)

	t := time.NewTicker(time.Duration(target.writePeriodSeconds * float64(time.Second)))
	defer t.Stop()

	for {
		select {
		case <-target.stop:
			target.flush(c)
			return

		case <-t.C:
			target.flush(c)
		}
	}
}

func (target *Target) flush(c *Client) {
	c.mu.Lock()
	events := target.events
	target.events = nil
	c.mu.Unlock()

	if len(events) == 0 {
		return
	}

	buf := &bytes.Buffer{}
	g := gzip.NewWriter(buf)
	enc := json.NewEncoder(g)

	err := enc.Encode(events)
	if err != nil {
		panic(err)
	}

	err = g.Close()
	if err != nil {
		panic(err)
	}

	resp, err := target.client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Content-Encoding", "gzip").
		SetBody(buf).
		Post("")
	if err != nil {
		log.Printf("failed write to event target: %s", err)
		return
	}

	if resp.IsError() {
		log.Printf("failed write to event target: %d %s: %s", resp.StatusCode(), resp.Status(), resp.String())
		return
	}
}
