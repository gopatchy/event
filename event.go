package event

import "time"

type Event struct {
	start time.Time

	Time       string         `json:"time"`
	SampleRate int64          `json:"samplerate"`
	Data       map[string]any `json:"data"`
}

func newEvent(eventType string, vals ...any) *Event {
	now := time.Now()

	ev := &Event{
		start: now,
		Time:  now.Format(time.RFC3339Nano),
		Data: map[string]any{
			"type": eventType,
		},
	}

	ev.Set(vals...)

	return ev
}

func (ev *Event) Set(vals ...any) {
	if len(vals)%2 != 0 {
		panic(vals)
	}

	for i := 0; i < len(vals); i += 2 {
		ev.Data[vals[i].(string)] = vals[i+1]
	}
}
