package event

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
)

var (
	eventTargetURL       *string
	eventHeaders         *string
	eventSecondsPerWrite *float64
	eventRateClasses     *string
	eventTotalPerSecond  *float64

	ErrInvalidFlags     = errors.New("invalid flags")
	ErrInvalidHeader    = fmt.Errorf("%w: invalid --event-headers (missing =)", ErrInvalidFlags)
	ErrInvalidRateClass = fmt.Errorf("%w: invalid --event-rate-classes (missing =)", ErrInvalidFlags)
)

func RegisterFlags() {
	eventTargetURL = flag.String("event-target-url", "", "URL to send events to")
	eventHeaders = flag.String("event-headers", "", "key=value|key=value|... headers to include with events")
	eventSecondsPerWrite = flag.Float64("event-seconds-per-write", 5.0, "write events to target every N seconds")
	eventRateClasses = flag.String("event-rate-classes", "", "key=value=rate|key=value=rate|...")
	eventTotalPerSecond = flag.Float64("event-total-per-second", 0.2, "write on average N total events per second")
}

func (c *Client) HandleFlags() error {
	if *eventTargetURL == "" {
		return nil
	}

	headers := map[string]string{}

	for _, pair := range strings.Split(*eventHeaders, "|") {
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return ErrInvalidHeader
		}

		headers[parts[0]] = parts[1]
	}

	target := c.AddTarget(
		*eventTargetURL,
		headers,
		*eventSecondsPerWrite,
	)

	for _, rc := range strings.Split(*eventRateClasses, "|") {
		if rc == "" {
			continue
		}

		parts := strings.SplitN(rc, "=", 3)
		if len(parts) != 3 {
			return ErrInvalidRateClass
		}

		rate, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return fmt.Errorf("%w %w", ErrInvalidRateClass, err)
		}

		target.AddRateClass(rate, parts[0], parts[1])
	}

	target.AddRateClass(
		*eventTotalPerSecond,
	)

	return nil
}
