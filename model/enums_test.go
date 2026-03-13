package model

import "testing"

func TestAreaString(t *testing.T) {
	if AreaInputs.String() != "I" {
		t.Fatalf("unexpected area string for inputs: %q", AreaInputs.String())
	}
	if Area(0xFF).String() != "?" {
		t.Fatalf("unexpected fallback area string: %q", Area(0xFF).String())
	}
}

func TestBlockTypeString(t *testing.T) {
	if BlockDB.String() != "DB" {
		t.Fatalf("unexpected block type string: %q", BlockDB.String())
	}
	if BlockType(0x00).String() != "?" {
		t.Fatalf("unexpected unknown block type string: %q", BlockType(0x00).String())
	}
}

func TestBlockLangString(t *testing.T) {
	if BlockLangGraph.String() != "GRAPH" {
		t.Fatalf("unexpected block language string: %q", BlockLangGraph.String())
	}
	if BlockLang(0xFF).String() != "?" {
		t.Fatalf("unexpected unknown block language string: %q", BlockLang(0xFF).String())
	}
}

func TestCPUStateString(t *testing.T) {
	if CPUStateRun.String() != "RUN" {
		t.Fatalf("unexpected CPU state string: %q", CPUStateRun.String())
	}
	if CPUState(255).String() != "UNKNOWN" {
		t.Fatalf("unexpected unknown CPU state string: %q", CPUState(255).String())
	}
}

func TestProtectionLevelString(t *testing.T) {
	if ProtectionReadWrite.String() != "Read/Write Protected" {
		t.Fatalf("unexpected protection string: %q", ProtectionReadWrite.String())
	}
	if ProtectionLevel(255).String() != "Unknown" {
		t.Fatalf("unexpected unknown protection string: %q", ProtectionLevel(255).String())
	}
}
