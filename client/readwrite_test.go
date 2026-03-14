package client

import (
	"context"
	"reflect"
	"testing"

	"github.com/otfabric/s7comm/model"
)

func TestReadAreaNotConnected(t *testing.T) {
	c := New("host")
	ctx := context.Background()
	_, err := c.ReadArea(ctx, model.Address{Area: model.AreaDB, DBNumber: 1, Start: 0, Size: 4})
	if err == nil {
		t.Fatal("ReadArea without connection should return error")
	}
	if err.Error() != "not connected" {
		t.Errorf("expected 'not connected', got %q", err.Error())
	}
}

func TestWriteAreaNotConnected(t *testing.T) {
	c := New("host")
	ctx := context.Background()
	err := c.WriteArea(ctx, model.Address{Area: model.AreaDB, DBNumber: 1, Start: 0, Size: 2}, []byte{0x01, 0x02})
	if err == nil {
		t.Fatal("WriteArea without connection should return error")
	}
	if err.Error() != "not connected" {
		t.Errorf("expected 'not connected', got %q", err.Error())
	}
}

func TestReadAreaContextCancelled(t *testing.T) {
	c := New("host")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.ReadArea(ctx, model.Address{Area: model.AreaDB, DBNumber: 1, Start: 0, Size: 4})
	if err == nil {
		t.Fatal("ReadArea with cancelled context should return error")
	}
}

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
