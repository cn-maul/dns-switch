//go:build !windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// ShowResults displays benchmark results via zenity or notify-send.
func ShowResults(results []BenchResult, bestIdx int) {
	var b strings.Builder

	fmt.Fprintln(&b, "📊 DNS 测速结果")
	fmt.Fprintln(&b)

	fmt.Fprintf(&b, "%-12s  %7s  %4s\n", "名称", "延迟", "丢包")
	for _, r := range results {
		rttStr := fmt.Sprintf("%.1fms", r.AvgRTT)
		lossStr := fmt.Sprintf("%d%%", r.Loss)
		if r.Err {
			rttStr = r.ErrMsg
			lossStr = "100%"
		}
		fmt.Fprintf(&b, "%-12s  %7s  %4s\n", r.Name, rttStr, lossStr)
	}
	fmt.Fprintln(&b)

	if bestIdx >= 0 && bestIdx < len(results) {
		best := results[bestIdx]
		fmt.Fprintf(&b, "最优: %s %s %.1fms\n", best.Name, best.IP, best.AvgRTT)
	} else {
		fmt.Fprintln(&b, "所有 DNS 服务器均不可达")
	}

	text := b.String()

	// Try zenity first (GUI dialog), then notify-send (notification).
	if err := exec.Command("zenity", "--info", "--text="+text).Run(); err == nil {
		return
	}
	if err := exec.Command("notify-send", "DNS 测速结果", text).Run(); err == nil {
		return
	}
	// Neither tool is available – silently ignore to avoid crash.
}
