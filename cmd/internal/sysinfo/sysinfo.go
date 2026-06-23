package sysinfo

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Stats holds a point-in-time snapshot of host resource usage.
type Stats struct {
	CPUPercent  float64 `json:"cpu_percent"`
	MemUsed     uint64  `json:"mem_used"`
	MemTotal    uint64  `json:"mem_total"`
	MemPercent  float64 `json:"mem_percent"`
	DiskUsed    uint64  `json:"disk_used"`
	DiskTotal   uint64  `json:"disk_total"`
	DiskPercent float64 `json:"disk_percent"`
	Load1       float64 `json:"load_1"`
	Load5       float64 `json:"load_5"`
	Load15      float64 `json:"load_15"`
	RecordedAt  string  `json:"recorded_at"`
}

// Collect samples all available host metrics and returns a Stats snapshot.
// CPU usage is measured over a ~200 ms window to produce a meaningful reading.
func Collect() (Stats, error) {
	cpu, err := cpuPercent()
	if err != nil {
		return Stats{}, fmt.Errorf("cpu: %w", err)
	}

	memUsed, memTotal, err := memInfo()
	if err != nil {
		return Stats{}, fmt.Errorf("mem: %w", err)
	}

	diskUsed, diskTotal, err := diskInfo("/")
	if err != nil {
		return Stats{}, fmt.Errorf("disk: %w", err)
	}

	l1, l5, l15, err := loadAvg()
	if err != nil {
		return Stats{}, fmt.Errorf("load: %w", err)
	}

	var memPct, diskPct float64
	if memTotal > 0 {
		memPct = float64(memUsed) / float64(memTotal) * 100
	}
	if diskTotal > 0 {
		diskPct = float64(diskUsed) / float64(diskTotal) * 100
	}

	return Stats{
		CPUPercent:  cpu,
		MemUsed:     memUsed,
		MemTotal:    memTotal,
		MemPercent:  round2(memPct),
		DiskUsed:    diskUsed,
		DiskTotal:   diskTotal,
		DiskPercent: round2(diskPct),
		Load1:       l1,
		Load5:       l5,
		Load15:      l15,
		RecordedAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// cpuPercent returns the CPU busy percentage averaged over ~200 ms.
func cpuPercent() (float64, error) {
	t1, err := readCPUStat()
	if err != nil {
		return 0, err
	}
	time.Sleep(200 * time.Millisecond)
	t2, err := readCPUStat()
	if err != nil {
		return 0, err
	}

	idle := (t2.idle + t2.iowait) - (t1.idle + t1.iowait)
	total := t2.total() - t1.total()
	if total == 0 {
		return 0, nil
	}
	return round2(float64(total-idle) / float64(total) * 100), nil
}

type cpuTimes struct {
	user, nice, system, idle, iowait, irq, softirq, steal uint64
}

func (c cpuTimes) total() uint64 {
	return c.user + c.nice + c.system + c.idle + c.iowait + c.irq + c.softirq + c.steal
}

func readCPUStat() (cpuTimes, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuTimes{}, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			break
		}
		parse := func(s string) uint64 { v, _ := strconv.ParseUint(s, 10, 64); return v }
		return cpuTimes{
			user:    parse(fields[1]),
			nice:    parse(fields[2]),
			system:  parse(fields[3]),
			idle:    parse(fields[4]),
			iowait:  parse(fields[5]),
			irq:     parse(fields[6]),
			softirq: parse(fields[7]),
			steal:   func() uint64 { if len(fields) > 8 { return parse(fields[8]) }; return 0 }(),
		}, nil
	}
	return cpuTimes{}, fmt.Errorf("cpu line not found in /proc/stat")
}

// memInfo parses /proc/meminfo and returns (used, total) in bytes.
func memInfo() (used, total uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	vals := map[string]uint64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		val, _ := strconv.ParseUint(parts[1], 10, 64)
		vals[key] = val
	}

	totalKB := vals["MemTotal"]
	availKB := vals["MemAvailable"]
	if totalKB == 0 {
		return 0, 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
	}
	usedKB := totalKB - availKB
	return usedKB * 1024, totalKB * 1024, nil
}

// diskInfo returns (used, total) bytes for the filesystem mounted at path.
func diskInfo(path string) (used, total uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	//nolint:unconvert
	total = uint64(st.Blocks) * uint64(st.Bsize)
	avail := uint64(st.Bavail) * uint64(st.Bsize)
	if avail > total {
		avail = total
	}
	return total - avail, total, nil
}

// loadAvg parses /proc/loadavg and returns the 1-, 5-, and 15-minute averages.
func loadAvg() (l1, l5, l15 float64, err error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected /proc/loadavg format")
	}
	parse := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	return parse(fields[0]), parse(fields[1]), parse(fields[2]), nil
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
