package event

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

// TODO: Switch to opentelemetry protocol
// TODO: Add protocol-level tests

type Client struct {
	targets []*Target
	hooks   []Hook

	mu sync.Mutex
}

type Hook func(context.Context, *Event)

func New() *Client {
	return &Client{}
}

func (c *Client) AddTarget(url string, headers map[string]string, writePeriodSeconds float64) *Target {
	target := &Target{
		client:             resty.New().SetBaseURL(url).SetHeaders(headers),
		writePeriodSeconds: writePeriodSeconds,
		windowSeconds:      100.0,
		stop:               make(chan bool),
		lastEvent:          time.Now(),
	}

	go target.flushLoop(c)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.targets = append(c.targets, target)

	return target
}

func (c *Client) AddHook(hook Hook) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.hooks = append(c.hooks, hook)

	return c
}

func (c *Client) Log(ctx context.Context, vals ...any) string {
	ev := NewEvent("log", vals...)
	c.WriteEvent(ctx, ev)

	parts := []string{}

	for i := 0; i < len(vals); i += 2 {
		parts = append(parts, fmt.Sprintf("%s=%s", vals[i], vals[i+1]))
	}

	msg := strings.Join(parts, " ")

	log.Print(msg)

	return msg
}

func (c *Client) Fatal(ctx context.Context, vals ...any) {
	msg := c.Log(ctx, vals...)
	c.Close()
	panic(msg)
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, target := range c.targets {
		target.close()
	}
}

func (c *Client) WriteEvent(ctx context.Context, ev *Event) {
	ev.Set("durationMS", time.Since(ev.start).Milliseconds())

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, hook := range c.hooks {
		hook(ctx, ev)
	}

	for _, target := range c.targets {
		target.writeEvent(ev)
	}
}
