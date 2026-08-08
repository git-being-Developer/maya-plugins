package memorytest

import "testing"

func TestSetGetRoundTrip(t *testing.T) {
	m := New()
	if err := m.Set("last_group", "Roommates"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok := m.Get("last_group")
	if !ok || got != "Roommates" {
		t.Fatalf("Get = (%q, %v), want (\"Roommates\", true)", got, ok)
	}
}

func TestTierIsolation(t *testing.T) {
	m := New()
	if err := m.SetPrivate("upi_id", "choco@upi"); err != nil {
		t.Fatalf("SetPrivate: %v", err)
	}
	if err := m.Set("last_payee", "Landlord"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if got, ok := m.GetPrivate("upi_id"); !ok || got != "choco@upi" {
		t.Fatalf("GetPrivate(upi_id) = (%q, %v), want (\"choco@upi\", true)", got, ok)
	}
	if got, ok := m.Get("last_payee"); !ok || got != "Landlord" {
		t.Fatalf("Get(last_payee) = (%q, %v), want (\"Landlord\", true)", got, ok)
	}

	if got, ok := m.Get("upi_id"); ok {
		t.Fatalf("Get(upi_id) returned a private value: %q", got)
	}
	if got, ok := m.GetPrivate("last_payee"); ok {
		t.Fatalf("GetPrivate(last_payee) returned a tool-visible value: %q", got)
	}
}

func TestDelete(t *testing.T) {
	m := New()
	_ = m.Set("k", "v")
	if err := m.Delete("k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := m.Get("k"); ok {
		t.Fatal("Get returned a value after Delete")
	}
}

func TestList(t *testing.T) {
	m := New()
	_ = m.Set("greeting", "hello")
	_ = m.SetPrivate("secret", "shh")

	entries := m.List()
	if len(entries) != 2 {
		t.Fatalf("List returned %d entries, want 2", len(entries))
	}
	if entries["greeting"].Private {
		t.Fatal("greeting marked private, should not be")
	}
	if !entries["secret"].Private {
		t.Fatal("secret not marked private, should be")
	}
}
