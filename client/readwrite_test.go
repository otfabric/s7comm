package client

import (
	"reflect"
	"testing"
)

func TestPlanReadChunks(t *testing.T) {
	got := planReadChunks(20, 6)
	want := []int{6, 6, 6, 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected chunks: got=%v want=%v", got, want)
	}
}

func TestPlanReadChunksEdgeCases(t *testing.T) {
	if got := planReadChunks(0, 8); len(got) != 0 {
		t.Fatalf("expected no chunks, got %v", got)
	}
	if got := planReadChunks(5, 0); !reflect.DeepEqual(got, []int{5}) {
		t.Fatalf("unexpected fallback chunking: %v", got)
	}
}
