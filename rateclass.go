package event

type rateClass struct {
	grantRate float64
	criteria  map[string]any

	eventRate float64
}

func (rc *rateClass) match(ev *Event) bool {
	for k, v := range rc.criteria {
		if ev.Data[k] != v {
			return false
		}
	}

	return true
}
