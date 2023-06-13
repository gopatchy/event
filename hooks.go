package event

import (
	"context"
	"runtime/debug"
	"runtime/metrics"
	"strings"
	"syscall"
	"time"
	"unicode"
)

func HookBuildInfo(_ context.Context, ev *Event) {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		panic("ReadBuildInfo() failed")
	}

	ev.Set(
		"goVersion", buildInfo.GoVersion,
		"goPackagePath", buildInfo.Path,
		"goMainModuleVersion", buildInfo.Main.Version,
	)
}

func HookMetrics(_ context.Context, ev *Event) {
	descs := metrics.All()

	samples := make([]metrics.Sample, len(descs))
	for i := range samples {
		samples[i].Name = descs[i].Name
	}

	metrics.Read(samples)

	for _, sample := range samples {
		name := convertMetricName(sample.Name)

		switch sample.Value.Kind() { //nolint:exhaustive
		case metrics.KindUint64:
			ev.Set(name, sample.Value.Uint64())
		case metrics.KindFloat64:
			ev.Set(name, sample.Value.Float64())
		}
	}
}

func HookRUsage(_ context.Context, ev *Event) {
	rusage := &syscall.Rusage{}

	err := syscall.Getrusage(syscall.RUSAGE_SELF, rusage)
	if err != nil {
		panic(err)
	}

	ev.Set(
		"rUsageUTime", time.Duration(rusage.Utime.Nano()).Seconds(),
		"rUsageSTime", time.Duration(rusage.Stime.Nano()).Seconds(),
	)
}

func convertMetricName(in string) string {
	upperNext := false

	in = strings.TrimLeft(in, "/")

	ret := strings.Builder{}

	for _, r := range in {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			upperNext = true
			continue
		}

		if upperNext {
			r = unicode.ToUpper(r)
			upperNext = false
		}

		ret.WriteRune(r)
	}

	return ret.String()
}
