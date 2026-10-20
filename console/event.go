package console

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	schemaVersion = 1
	timeLayout    = "2006-01-02T15:04:05.000000Z"
)

// Event is one event of a JSON narration stream.
type Event struct {
	// Seq numbers the events of a run in order, from 1.
	Seq int
	// Time is when the event happened.
	Time time.Time
	// Event is the kind of event: begin, line, task, output or exit.
	Event string
	// App is the program name, on begin.
	App string
	// Version is the program version, on begin.
	Version string
	// Schema is the version of the stream format, on begin.
	Schema int
	// Level is the level of a line, such as info or warn.
	Level string
	// Text is the text of a line or of output.
	Text string
	// Hint is the hint given with a line.
	Hint string
	// Task is the id of the task the event belongs to; zero means none.
	Task int
	// State is what happened to a task, such as started, phase, progress, done or failed.
	State string
	// Label is the task label, when the task starts.
	Label string
	// Phase is the task's current phase.
	Phase string
	// Done is the amount of the phase finished.
	Done int64
	// Total is the full amount of the phase; negative means unknown.
	Total int64
	// Unit is what progress counts, items or bytes.
	Unit string
	// Outcome is the text a task ended with.
	Outcome string
	// ElapsedMS is how long the task ran, in milliseconds.
	ElapsedMS int64
	// Stream is the stream output was written to, stdout or stderr.
	Stream string
	// Partial reports that the output text does not end its line.
	Partial bool
	// Code is the exit code, on exit.
	Code int
	// Signal names the signal that ended the run, on exit.
	Signal string
}

type wireEvent struct {
	Seq       int    `json:"seq"`
	Time      string `json:"time"`
	Event     string `json:"event"`
	App       string `json:"app"`
	Version   string `json:"version"`
	Schema    *int   `json:"schema"`
	Level     string `json:"level"`
	Text      string `json:"text"`
	Hint      string `json:"hint"`
	Task      int    `json:"task"`
	State     string `json:"state"`
	Label     string `json:"label"`
	Phase     string `json:"phase"`
	Done      *int64 `json:"done"`
	Total     *int64 `json:"total"`
	Unit      string `json:"unit"`
	Outcome   string `json:"outcome"`
	ElapsedMS *int64 `json:"elapsed_ms"`
	Stream    string `json:"stream"`
	Partial   bool   `json:"partial"`
	Code      *int   `json:"code"`
	Signal    string `json:"signal"`
}

type fieldWriter struct {
	buf []byte
	err error
}

func (w *fieldWriter) key(name string) {
	if len(w.buf) > 1 {
		w.buf = append(w.buf, ',')
	}
	w.buf = append(w.buf, '"')
	w.buf = append(w.buf, name...)
	w.buf = append(w.buf, '"', ':')
}

func (w *fieldWriter) str(name, value string) {
	w.key(name)
	b, err := json.Marshal(value)
	if err != nil && w.err == nil {
		w.err = err
	}
	w.buf = append(w.buf, b...)
}

func (w *fieldWriter) num(name string, value int64) {
	w.key(name)
	w.buf = strconv.AppendInt(w.buf, value, 10)
}

func (w *fieldWriter) flag(name string, value bool) {
	w.key(name)
	w.buf = strconv.AppendBool(w.buf, value)
}

// MarshalJSON encodes the event with only the fields its kind carries.
func (e Event) MarshalJSON() ([]byte, error) {
	w := &fieldWriter{buf: []byte{'{'}}
	w.num("seq", int64(e.Seq))
	w.str("time", e.Time.UTC().Format(timeLayout))
	w.str("event", e.Event)
	if e.Event == "begin" {
		w.str("app", e.App)
		w.str("version", e.Version)
		w.num("schema", int64(e.Schema))
	}
	if e.Event == "line" {
		w.str("level", e.Level)
		w.str("text", e.Text)
	}
	if e.Hint != "" {
		w.str("hint", e.Hint)
	}
	if e.Task != 0 {
		w.num("task", int64(e.Task))
	}
	if e.Event == "task" {
		w.str("state", e.State)
	}
	if e.State == "started" {
		w.str("label", e.Label)
	}
	if e.State == "phase" || e.State == "progress" {
		w.str("phase", e.Phase)
	}
	if e.State == "progress" {
		w.num("done", e.Done)
		w.num("total", e.Total)
		w.str("unit", e.Unit)
	}
	if e.State == "done" || e.State == "failed" {
		w.str("outcome", e.Outcome)
		w.num("elapsed_ms", e.ElapsedMS)
	}
	if e.Event == "output" {
		w.str("stream", e.Stream)
		w.str("text", e.Text)
		if e.Partial {
			w.flag("partial", true)
		}
	}
	if e.Event == "exit" {
		w.num("code", int64(e.Code))
	}
	if e.Signal != "" {
		w.str("signal", e.Signal)
	}
	if w.err != nil {
		return nil, w.err
	}
	return append(w.buf, '}'), nil
}

// UnmarshalJSON decodes one event.
func (e *Event) UnmarshalJSON(b []byte) error {
	var w wireEvent
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	var parsed time.Time
	if w.Time != "" {
		t, err := time.Parse(timeLayout, w.Time)
		if err != nil {
			return fmt.Errorf("event time: %w", err)
		}
		parsed = t
	}
	out := Event{
		Seq:     w.Seq,
		Time:    parsed,
		Event:   w.Event,
		App:     w.App,
		Version: w.Version,
		Level:   w.Level,
		Text:    w.Text,
		Hint:    w.Hint,
		Task:    w.Task,
		State:   w.State,
		Label:   w.Label,
		Phase:   w.Phase,
		Unit:    w.Unit,
		Outcome: w.Outcome,
		Stream:  w.Stream,
		Partial: w.Partial,
		Signal:  w.Signal,
	}
	if w.Schema != nil {
		out.Schema = *w.Schema
	}
	if w.Done != nil {
		out.Done = *w.Done
	}
	if w.Total != nil {
		out.Total = *w.Total
	}
	if w.ElapsedMS != nil {
		out.ElapsedMS = *w.ElapsedMS
	}
	if w.Code != nil {
		out.Code = *w.Code
	}
	*e = out
	return nil
}

// ReadEvents reads a JSON narration stream from r, skipping lines that are not events.
func ReadEvents(r io.Reader) ([]Event, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var events []Event
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return events, fmt.Errorf("decode event: %w", err)
		}
		events = append(events, e)
	}
	if err := sc.Err(); err != nil {
		return events, err
	}
	return events, nil
}
