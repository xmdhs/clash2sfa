package utils

import "testing"

func TestAnyGetAndSetPointerForms(t *testing.T) {
	m := map[string]any{"tag": "n1"}
	if got := AnyGet[string](&m, "tag"); got != "n1" {
		t.Fatalf("AnyGet(*map) = %q, want n1", got)
	}
	if !AnySet(&m, "n2", "tag") || AnyGet[string](m, "tag") != "n2" {
		t.Fatal("AnySet(*map) did not update the map")
	}

	var value any = m
	if got := AnyGet[string](&value, "tag"); got != "n2" {
		t.Fatalf("AnyGet(*any) = %q, want n2", got)
	}
	if !AnySet(&value, "n3", "tag") || AnyGet[string](m, "tag") != "n3" {
		t.Fatal("AnySet(*any) did not update the map")
	}
}

func TestAnySetPointerToPointerMap(t *testing.T) {
	m := map[string]any{}
	pm := &m
	if !AnySet(&pm, "value", "field") {
		t.Fatal("AnySet(**map) returned false")
	}
	if got := AnyGet[string](m, "field"); got != "value" {
		t.Fatalf("AnySet(**map) wrote %q, want value", got)
	}
}
