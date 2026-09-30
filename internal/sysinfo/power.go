package sysinfo

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Power is a laptop battery and a CPU temperature, when the machine has them.
type Power struct {
	HasBattery bool
	Percent    int
	Status     string
	HasTemp    bool
	TempC      float64
}

func readPower() Power {
	return readPowerAt("/sys/class/power_supply", "/sys/class/thermal")
}

func readPowerAt(powerRoot, thermalRoot string) Power {
	var p Power
	dents, err := os.ReadDir(powerRoot)
	if err == nil {
		for _, d := range dents {
			if !strings.HasPrefix(d.Name(), "BAT") {
				continue
			}
			base := filepath.Join(powerRoot, d.Name())
			capB, err1 := os.ReadFile(filepath.Join(base, "capacity"))
			if err1 != nil {
				continue
			}
			n, errN := strconv.Atoi(strings.TrimSpace(string(capB)))
			if errN != nil {
				continue
			}
			p.HasBattery = true
			p.Percent = n
			if st, err2 := os.ReadFile(filepath.Join(base, "status")); err2 == nil {
				p.Status = strings.TrimSpace(string(st))
			}
			break
		}
	}
	p.TempC, p.HasTemp = readCPUTemp(thermalRoot)
	return p
}

func readCPUTemp(root string) (float64, bool) {
	dents, err := os.ReadDir(root)
	if err != nil {
		return 0, false
	}
	bestScore := -1
	var best float64
	for _, d := range dents {
		base := filepath.Join(root, d.Name())
		typB, errT := os.ReadFile(filepath.Join(base, "type"))
		tempB, errC := os.ReadFile(filepath.Join(base, "temp"))
		if errT != nil || errC != nil {
			continue
		}
		score := tempScore(strings.TrimSpace(string(typB)))
		if score < 0 {
			continue
		}
		milli, err := strconv.ParseFloat(strings.TrimSpace(string(tempB)), 64)
		if err != nil {
			continue
		}
		c := milli / 1000
		if c < 1 || c > 120 {
			continue
		}
		if score > bestScore {
			bestScore = score
			best = c
		}
	}
	if bestScore < 0 {
		return 0, false
	}
	return best, true
}

func tempScore(typ string) int {
	t := strings.ToLower(typ)
	switch {
	case strings.Contains(t, "pkg"):
		return 5
	case strings.Contains(t, "coretemp"), strings.Contains(t, "k10temp"), strings.Contains(t, "cpu"):
		return 4
	case t == "acpitz":
		return 3
	default:
		return -1
	}
}
