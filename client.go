package event

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

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

	go c.flushLoop(target)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.targets = append(c.targets, target)

	return target
}

func (c *Client) AddHook(hook Hook) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.hooks = append(c.hooks, hook)
}

func (c *Client) Log(ctx context.Context, vals ...any) {
	ev := newEvent("log", vals...)
	c.writeEvent(ctx, ev)

	parts := []string{}

	for i := 0; i < len(vals); i += 2 {
		parts = append(parts, fmt.Sprintf("%s=%s", vals[i], vals[i+1]))
	}

	log.Print(strings.Join(parts, " "))
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, target := range c.targets {
		close(target.stop)
	}
}

func (c *Client) writeEvent(ctx context.Context, ev *Event) {
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

func (c *Client) flushLoop(target *Target) {
	t := time.NewTicker(time.Duration(target.writePeriodSeconds * float64(time.Second)))
	defer t.Stop()

	for {
		select {
		case <-target.stop:
			c.flush(target)
			return

		case <-t.C:
			c.flush(target)
		}
	}
}

func (c *Client) flush(target *Target) {
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