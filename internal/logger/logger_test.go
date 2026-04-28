package logger

import (
	"bytes"
	"log"
	"testing"
)

// captureLog redirects the standard logger output to a buffer for assertions.
func captureLog(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	prevOut := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	return &buf, func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	}
}

func TestInfo_AlwaysLogs(t *testing.T) {
	buf, restore := captureLog(t)
	defer restore()

	SetDebug(false)
	Info("hello %s", "world")

	if got := buf.String(); got != "hello world\n" {
		t.Errorf("got %q, want %q", got, "hello world\n")
	}
}

func TestDebug_OnlyWhenEnabled(t *testing.T) {
	buf, restore := captureLog(t)
	defer restore()

	SetDebug(false)
	Debug("hidden %d", 1)
	if buf.Len() != 0 {
		t.Errorf("debug logged while disabled: %q", buf.String())
	}

	SetDebug(true)
	defer SetDebug(false)
	Debug("visible %d", 2)
	if got := buf.String(); got != "visible 2\n" {
		t.Errorf("got %q, want %q", got, "visible 2\n")
	}
}
