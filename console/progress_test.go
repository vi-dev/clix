package console

import (
	"bytes"
	"strings"
	"testing"
)

type progressCall struct {
	done, total int64
	unit        Unit
}

type fakeTask struct {
	Task
	calls []progressCall
}

func (f *fakeTask) Progress(done, total int64, unit Unit) {
	f.calls = append(f.calls, progressCall{done, total, unit})
}

func TestProgressWriterReportsAfterEveryWrite(t *testing.T) {
	ft := &fakeTask{}
	w := ProgressWriter(ft, 10)
	if _, err := w.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("defg")); err != nil {
		t.Fatal(err)
	}
	want := []progressCall{{3, 10, Bytes}, {7, 10, Bytes}}
	if len(ft.calls) != len(want) || ft.calls[0] != want[0] || ft.calls[1] != want[1] {
		t.Errorf("calls = %v, want %v", ft.calls, want)
	}
}

func TestProgressReaderReportsBytesRead(t *testing.T) {
	ft := &fakeTask{}
	r := ProgressReader(ft, strings.NewReader("abcdefg"), 7)
	var out bytes.Buffer
	buf := make([]byte, 3)
	for {
		n, err := r.Read(buf)
		out.Write(buf[:n])
		if err != nil {
			break
		}
	}
	if out.String() != "abcdefg" {
		t.Errorf("read = %q", out.String())
	}
	want := []progressCall{{3, 7, Bytes}, {6, 7, Bytes}, {7, 7, Bytes}}
	if len(ft.calls) != len(want) {
		t.Fatalf("calls = %v, want %v", ft.calls, want)
	}
	for i := range want {
		if ft.calls[i] != want[i] {
			t.Errorf("call %d = %v, want %v", i, ft.calls[i], want[i])
		}
	}
}

func TestProgressHelpersTolerateNilTask(t *testing.T) {
	n, err := ProgressWriter(nil, 5).Write([]byte("abc"))
	if n != 3 || err != nil {
		t.Errorf("write = %d, %v", n, err)
	}
	b := make([]byte, 4)
	n, _ = ProgressReader(nil, strings.NewReader("xy"), 2).Read(b)
	if n != 2 {
		t.Errorf("read = %d", n)
	}
}
