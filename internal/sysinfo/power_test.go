package sysinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPowerAt(t *testing.T) {
	root := t.TempDir()
	bat := filepath.Join(root, "power", "BAT0")
	if err := os.MkdirAll(bat, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bat, "capacity"), []byte("72\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bat, "status"), []byte("Discharging\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	therm := filepath.Join(root, "thermal")
	wifi := filepath.Join(therm, "thermal_zone0")
	pkg := filepath.Join(therm, "thermal_zone1")
	if err := os.MkdirAll(wifi, 0o755); err != nil || os.MkdirAll(pkg, 0o755) != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wifi, "type"), []byte("iwlwifi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wifi, "temp"), []byte("39000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "type"), []byte("x86_pkg_temp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "temp"), []byte("61500\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := readPowerAt(filepath.Join(root, "power"), therm)
	if !got.HasBattery || got.Percent != 72 || got.Status != "Discharging" {
		t.Fatalf("battery %+v", got)
	}
	if !got.HasTemp || got.TempC < 61 || got.TempC > 62 {
		t.Fatalf("temp %+v", got)
	}
}

func TestReadPowerMissing(t *testing.T) {
	got := readPowerAt(filepath.Join(t.TempDir(), "nope"), filepath.Join(t.TempDir(), "nope"))
	if got.HasBattery || got.HasTemp {
		t.Fatalf("%+v", got)
	}
}
