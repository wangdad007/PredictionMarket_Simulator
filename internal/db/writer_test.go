package db

import "testing"

func TestParseStoredBigInt(t *testing.T) {
	value, err := parseStoredBigInt("reserve_yes", " 123456 ")
	if err != nil {
		t.Fatal(err)
	}
	if value.String() != "123456" {
		t.Fatalf("value = %s, want 123456", value)
	}
}

func TestParseStoredBigIntRejectsInvalidValue(t *testing.T) {
	if _, err := parseStoredBigInt("reserve_yes", "not-a-number"); err == nil {
		t.Fatal("parseStoredBigInt succeeded, want error")
	}
}
