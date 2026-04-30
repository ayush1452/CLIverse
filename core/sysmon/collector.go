package sysmon

import (
	"sort"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// Snapshot holds a point-in-time sample of all system metrics.
type Snapshot struct {
	Time time.Time

	CPUTotal float64
	CPUCores []float64

	MemTotal uint64
	MemUsed  uint64
	MemPct   float64

	SwapTotal uint64
	SwapUsed  uint64
	SwapPct   float64

	DiskReadBPS  float64
	DiskWriteBPS float64

	NetRecvBPS float64
	NetSentBPS float64

	Procs []ProcInfo
}

// ProcInfo summarises a single running process.
type ProcInfo struct {
	PID    int32
	Name   string
	CPU    float64
	MemRSS uint64
}

// Collector gathers metrics and computes per-second I/O rates between calls.
type Collector struct {
	prevDiskRead  uint64
	prevDiskWrite uint64
	prevNetRecv   uint64
	prevNetSent   uint64
	prevTime      time.Time
}

// New returns a ready-to-use Collector.
func New() *Collector { return &Collector{} }

// Collect gathers a fresh Snapshot. Safe to call from any goroutine.
func (c *Collector) Collect() *Snapshot {
	now := time.Now()
	elapsed := now.Sub(c.prevTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}

	s := &Snapshot{Time: now}

	// --- CPU ---
	if pcts, err := cpu.Percent(0, false); err == nil && len(pcts) > 0 {
		s.CPUTotal = pcts[0]
	}
	if pcts, err := cpu.Percent(0, true); err == nil {
		s.CPUCores = pcts
	}

	// --- Memory ---
	if vm, err := mem.VirtualMemory(); err == nil {
		s.MemTotal, s.MemUsed, s.MemPct = vm.Total, vm.Used, vm.UsedPercent
	}
	if sw, err := mem.SwapMemory(); err == nil {
		s.SwapTotal, s.SwapUsed, s.SwapPct = sw.Total, sw.Used, sw.UsedPercent
	}

	// --- Disk I/O (aggregate all physical disks) ---
	if counters, err := disk.IOCounters(); err == nil {
		var r, w uint64
		for _, d := range counters {
			r += d.ReadBytes
			w += d.WriteBytes
		}
		if !c.prevTime.IsZero() {
			s.DiskReadBPS = float64(r-c.prevDiskRead) / elapsed
			s.DiskWriteBPS = float64(w-c.prevDiskWrite) / elapsed
		}
		c.prevDiskRead, c.prevDiskWrite = r, w
	}

	// --- Network I/O (aggregate all interfaces) ---
	if counters, err := psnet.IOCounters(false); err == nil && len(counters) > 0 {
		recv, sent := counters[0].BytesRecv, counters[0].BytesSent
		if !c.prevTime.IsZero() {
			s.NetRecvBPS = float64(recv-c.prevNetRecv) / elapsed
			s.NetSentBPS = float64(sent-c.prevNetSent) / elapsed
		}
		c.prevNetRecv, c.prevNetSent = recv, sent
	}

	// --- Processes ---
	if procs, err := process.Processes(); err == nil {
		infos := make([]ProcInfo, 0, len(procs))
		for _, p := range procs {
			name, _ := p.Name()
			if name == "" {
				continue
			}
			cpuPct, _ := p.CPUPercent()
			mi, _ := p.MemoryInfo()
			var rss uint64
			if mi != nil {
				rss = mi.RSS
			}
			infos = append(infos, ProcInfo{PID: p.Pid, Name: name, CPU: cpuPct, MemRSS: rss})
		}
		sort.Slice(infos, func(i, j int) bool { return infos[i].CPU > infos[j].CPU })
		if len(infos) > 64 {
			infos = infos[:64]
		}
		s.Procs = infos
	}

	c.prevTime = now
	return s
}
