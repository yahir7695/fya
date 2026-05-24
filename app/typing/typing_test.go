package typing

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEstimate(t *testing.T) {
	got := NewInjector(Config{WPM: 100, Jitter: -1, SettleDelay: time.Second}).estimate("hello")

	assert.Equal(t, 5, got.characters)
	assert.Equal(t, 120*time.Millisecond, got.perRune)
	assert.Equal(t, 1600*time.Millisecond, got.total)
}

func TestValidateTimeoutGuard(t *testing.T) {
	err := NewInjector(Config{WPM: 10, TurnTimeout: time.Millisecond}).validate("this is too long")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds turn timeout")
}

func TestValidateWarning(t *testing.T) {
	var warn bytes.Buffer

	err := NewInjector(Config{WPM: 10, WarnThreshold: time.Millisecond, Warn: &warn}).validate("long prompt")

	require.NoError(t, err)
	assert.Contains(t, warn.String(), "estimated prompt typing duration")
}

func TestTypePrompt(t *testing.T) {
	sleep := &fakeSleeper{}
	var out bytes.Buffer

	err := NewInjector(Config{WPM: 100, Jitter: -1, SettleDelay: time.Millisecond, Sleeper: sleep}).Type(t.Context(), &out, "hi")

	require.NoError(t, err)
	assert.Equal(t, "hi\r", out.String())
	assert.Len(t, sleep.delays, 3, "two per-rune sleeps plus one settle delay")
}

func TestTypeMultilinePrompt(t *testing.T) {
	sleep := &fakeSleeper{}
	var out bytes.Buffer

	err := NewInjector(Config{Jitter: -1, Sleeper: sleep}).Type(t.Context(), &out, "a\nb")

	require.NoError(t, err)
	assert.Equal(t, "a\x1b\rb\r", out.String())
}

func TestJitterBounds(t *testing.T) {
	injector := NewInjector(Config{
		WPM:    100,
		Jitter: 0.25,
		Rand:   &sequenceJitter{values: []float64{0, 1, 0.5}},
	})

	assert.Equal(t, 90*time.Millisecond, injector.jitteredDelay(), "Rand=0 → base - spread")
	assert.Equal(t, 150*time.Millisecond, injector.jitteredDelay(), "Rand=1 → base + spread")
	assert.Equal(t, 120*time.Millisecond, injector.jitteredDelay(), "Rand=0.5 → base")
}

// explicit Jitter=0 must produce the base per-rune delay with no random offset,
// proving the flag --typing-jitter=0 actually disables jitter at runtime.
func TestJitterZeroHonored(t *testing.T) {
	injector := NewInjector(Config{
		WPM:    100,
		Jitter: 0,
		Rand:   &sequenceJitter{values: []float64{0.999}}, // would shift if jitter applied
	})

	assert.Equal(t, 120*time.Millisecond, injector.jitteredDelay(), "Jitter=0 must disable jitter")
}

func TestTypeRuneWriteError(t *testing.T) {
	w := &errWriter{failAfter: 0}

	err := NewInjector(Config{Jitter: -1, Sleeper: &fakeSleeper{}}).Type(t.Context(), w, "x")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "write prompt rune")
}

func TestTypeNewlineWriteError(t *testing.T) {
	w := &errWriter{failAfter: 0}

	err := NewInjector(Config{Jitter: -1, Sleeper: &fakeSleeper{}}).Type(t.Context(), w, "\n")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "write multiline newline")
}

func TestTypeSubmitWriteError(t *testing.T) {
	w := &errWriter{failAfter: 1}

	err := NewInjector(Config{Jitter: -1, Sleeper: &fakeSleeper{}}).Type(t.Context(), w, "x")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "submit prompt")
}

func TestTypeDelayCancellation(t *testing.T) {
	err := NewInjector(Config{Sleeper: cancelSleeper{}}).Type(t.Context(), &bytes.Buffer{}, "x")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "typing delay")
}

func TestTypeNilWriter(t *testing.T) {
	err := NewInjector(Config{}).Type(t.Context(), nil, "x")

	require.Error(t, err)
}

type fakeSleeper struct {
	delays []time.Duration
}

func (s *fakeSleeper) Sleep(_ context.Context, d time.Duration) error {
	s.delays = append(s.delays, d)
	return nil
}

type cancelSleeper struct{}

func (cancelSleeper) Sleep(context.Context, time.Duration) error {
	return context.Canceled
}

type sequenceJitter struct {
	values []float64
	idx    int
}

func (j *sequenceJitter) Float64() float64 {
	if j.idx >= len(j.values) {
		return 0.5
	}
	value := j.values[j.idx]
	j.idx++
	return value
}

type errWriter struct {
	failAfter int
	writes    int
}

func (w *errWriter) Write(p []byte) (int, error) {
	if w.writes >= w.failAfter {
		return 0, errors.New("write failed")
	}
	w.writes++
	return len(p), nil
}
