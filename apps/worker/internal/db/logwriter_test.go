package db

import (
	"testing"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func TestSubmit_EnqueuesWhenBufferAvailable(t *testing.T) {
	w := &LogWriter{ch: make(chan logEntry, 10)}
	ok := w.Submit(types.RelayLog{RequestModelName: "gpt-4"}, nil, "")
	if !ok {
		t.Error("Submit should return true when buffer has space")
	}
	if len(w.ch) != 1 {
		t.Errorf("expected 1 entry in channel, got %d", len(w.ch))
	}
}

func TestSubmit_DropsWhenBufferFull(t *testing.T) {
	w := &LogWriter{ch: make(chan logEntry, 1)}
	// Fill the buffer
	w.Submit(types.RelayLog{}, nil, "")

	// This should be dropped
	ok := w.Submit(types.RelayLog{RequestModelName: "dropped"}, nil, "")
	if ok {
		t.Error("Submit should return false when buffer is full")
	}
	if w.DroppedCount() != 1 {
		t.Errorf("expected drop count 1, got %d", w.DroppedCount())
	}
}

func TestSubmit_DropCountAccumulates(t *testing.T) {
	w := &LogWriter{ch: make(chan logEntry, 1)}
	w.Submit(types.RelayLog{}, nil, "") // fill

	for i := 0; i < 5; i++ {
		w.Submit(types.RelayLog{}, nil, "")
	}

	if w.DroppedCount() != 5 {
		t.Errorf("expected drop count 5, got %d", w.DroppedCount())
	}
	// Buffer should still have exactly 1 entry
	if len(w.ch) != 1 {
		t.Errorf("expected 1 entry in channel, got %d", len(w.ch))
	}
}
