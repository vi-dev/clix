package console

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func marshalString(t *testing.T, e Event) string {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEventMarshalBegin(t *testing.T) {
	e := Event{
		Seq:     1,
		Time:    time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Event:   "begin",
		App:     "nem",
		Version: "0.9.3",
		Schema:  1,
	}
	want := `{"seq":1,"time":"2026-01-01T12:00:00.000000Z","event":"begin","app":"nem","version":"0.9.3","schema":1}`
	if got := marshalString(t, e); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestEventMarshalProgressKeepsZeroDone(t *testing.T) {
	e := Event{
		Seq:   3,
		Time:  time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Event: "task",
		Task:  2,
		State: "progress",
		Done:  0,
		Total: -1,
		Unit:  "bytes",
	}
	got := marshalString(t, e)
	if !strings.Contains(got, `"done":0,"total":-1,"unit":"bytes"`) {
		t.Fatalf("got %s", got)
	}
}

func TestEventMarshalExitKeepsZeroCode(t *testing.T) {
	e := Event{
		Seq:   12,
		Time:  time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Event: "exit",
	}
	want := `{"seq":12,"time":"2026-01-01T12:00:00.000000Z","event":"exit","code":0}`
	if got := marshalString(t, e); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestEventMarshalOutputPartial(t *testing.T) {
	e := Event{
		Seq:     4,
		Time:    time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Event:   "output",
		Stream:  "stderr",
		Text:    "x",
		Partial: true,
	}
	got := marshalString(t, e)
	if !strings.Contains(got, `"stream":"stderr","text":"x","partial":true`) {
		t.Fatalf("got %s", got)
	}
	if strings.Contains(got, `"task"`) {
		t.Fatalf("unexpected task key in %s", got)
	}
}

func TestEventRoundTrip(t *testing.T) {
	cases := []Event{
		{
			Seq: 1, Time: time.Date(2026, 3, 4, 5, 6, 7, 123456000, time.UTC), Event: "begin",
			App: "nem", Version: "1.2.3", Schema: 1,
		},
		{
			Seq: 2, Time: time.Date(2026, 3, 4, 5, 6, 8, 0, time.UTC), Event: "line",
			Level: "error", Text: "failed", Hint: "try again", Task: 3,
		},
		{
			Seq: 3, Time: time.Date(2026, 3, 4, 5, 6, 9, 0, time.UTC), Event: "task",
			Task: 3, State: "started", Label: "install",
		},
		{
			Seq: 4, Time: time.Date(2026, 3, 4, 5, 6, 9, 0, time.UTC), Event: "task",
			Task: 3, State: "progress", Phase: "download", Done: 10, Total: 20, Unit: "items",
		},
		{
			Seq: 5, Time: time.Date(2026, 3, 4, 5, 6, 9, 0, time.UTC), Event: "task",
			Task: 3, State: "failed", Outcome: "boom", ElapsedMS: 1500,
		},
		{
			Seq: 6, Time: time.Date(2026, 3, 4, 5, 6, 9, 0, time.UTC), Event: "output",
			Task: 3, Stream: "stdout", Text: "hello", Partial: true,
		},
		{
			Seq: 7, Time: time.Date(2026, 3, 4, 5, 6, 9, 0, time.UTC), Event: "exit",
			Code: 130, Signal: "interrupt",
		},
		{
			Seq: 9, Time: time.Date(2026, 3, 4, 5, 6, 9, 0, time.UTC), Event: "task",
			Task: 4, State: "progress", Phase: "unpack", Done: 0, Total: -1, Unit: "bytes",
		},
		{
			Seq: 8, Time: time.Date(2026, 3, 4, 5, 6, 9, 0, time.UTC), Event: "exit",
		},
	}
	for _, want := range cases {
		b, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		var got Event
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("%s: %v", b, err)
		}
		if !got.Time.Equal(want.Time) {
			t.Fatalf("time: got %v, want %v", got.Time, want.Time)
		}
		got.Time = want.Time
		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	}
}

func TestReadEventsSkipsNonJSONLines(t *testing.T) {
	in := `{"seq":1,"time":"2026-01-01T12:00:00.000000Z","event":"begin","app":"nem","version":"0.9.3","schema":1}
panic: boom

goroutine 1 [running]:
{"seq":2,"time":"2026-01-01T12:00:01.000000Z","event":"exit","code":0}
`
	events, err := ReadEvents(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Event != "begin" || events[1].Event != "exit" {
		t.Fatalf("got %+v", events)
	}
}

func TestReadEventsReportsBadJSONObject(t *testing.T) {
	if _, err := ReadEvents(strings.NewReader("{not json\n")); err == nil {
		t.Fatal("expected an error")
	}
}

func TestEventMarshalOutputWithTaskHasNoState(t *testing.T) {
	e := Event{
		Seq:    6,
		Time:   time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Event:  "output",
		Task:   2,
		Stream: "stderr",
		Text:   "make: *** [all] Error 2\n",
	}
	want := `{"seq":6,"time":"2026-01-01T12:00:00.000000Z","event":"output","task":2,"stream":"stderr","text":"make: *** [all] Error 2\n"}`
	if got := marshalString(t, e); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestEventMarshalLineWithTaskHasNoState(t *testing.T) {
	e := Event{
		Seq:   7,
		Time:  time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Event: "line",
		Level: "warn",
		Text:  "careful",
		Task:  2,
	}
	want := `{"seq":7,"time":"2026-01-01T12:00:00.000000Z","event":"line","level":"warn","text":"careful","task":2}`
	if got := marshalString(t, e); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestEventMarshalEscapesText(t *testing.T) {
	text := "q\" b\\ n\n r\r c\x01 t\t"
	e := Event{
		Seq:   1,
		Time:  time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Event: "line",
		Level: "info",
		Text:  text,
	}
	want := `{"seq":1,"time":"2026-01-01T12:00:00.000000Z","event":"line","level":"info","text":"q\" b\\ n\n r\r c\u0001 t\t"}`
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != want {
		t.Fatalf("got %s, want %s", b, want)
	}
	var got Event
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Text != text {
		t.Fatalf("got %q, want %q", got.Text, text)
	}
}
