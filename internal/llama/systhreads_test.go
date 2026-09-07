package llama

import (
	"strings"
	"testing"
)

func TestParseCPUInfoPhysicalCores(t *testing.T) {
	// 4 physical cores, 2 hyperthreads each, one socket.
	smt4c8t := ""
	for cpu := 0; cpu < 8; cpu++ {
		core := cpu % 4
		smt4c8t += "processor\t: " + itoa(cpu) + "\n" +
			"physical id\t: 0\n" +
			"core id\t\t: " + itoa(core) + "\n\n"
	}

	// 2 sockets × 2 cores, no SMT.
	dual2c := ""
	n := 0
	for sock := 0; sock < 2; sock++ {
		for core := 0; core < 2; core++ {
			dual2c += "processor\t: " + itoa(n) + "\n" +
				"physical id\t: " + itoa(sock) + "\n" +
				"core id\t\t: " + itoa(core) + "\n\n"
			n++
		}
	}

	// ARM-style: no physical id / core id fields at all.
	armNoIDs := "processor\t: 0\nBogoMIPS\t: 48.00\n\nprocessor\t: 1\nBogoMIPS\t: 48.00\n\n"

	tests := []struct {
		name string
		in   string
		want int
	}{
		{"4 cores / 8 threads, one socket", smt4c8t, 4},
		{"2 sockets x 2 cores", dual2c, 4},
		{"no id fields -> 0", armNoIDs, 0},
		{"empty -> 0", "", 0},
		{"trailing record without blank line still counted", "physical id\t: 0\ncore id\t\t: 0\n", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseCPUInfoPhysicalCores(strings.NewReader(tt.in)); got != tt.want {
				t.Errorf("parseCPUInfoPhysicalCores() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestServerThreads(t *testing.T) {
	tests := []struct {
		name                       string
		gpuLayers, logical, physic int
		want                       int
	}{
		{"gpu offload -> let llama.cpp decide", 999, 16, 8, 0},
		{"cpu + SMT -> physical cores", 0, 16, 8, 8},
		{"cpu, no SMT -> 0", 0, 8, 8, 0},
		{"cpu, physical unknown -> 0", 0, 12, 0, 0},
		{"cpu, weird physical > logical -> 0", 0, 4, 8, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serverThreads(tt.gpuLayers, tt.logical, tt.physic); got != tt.want {
				t.Errorf("serverThreads(%d,%d,%d) = %d, want %d",
					tt.gpuLayers, tt.logical, tt.physic, got, tt.want)
			}
		})
	}
}
