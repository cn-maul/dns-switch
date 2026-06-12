//go:build windows

package main

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows"
)

// ShowResults displays benchmark results in a Windows message box.
func ShowResults(results []BenchResult, bestIdx int) {
	title := "DNS 测速结果"

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

	windows.MessageBox(
		0,
		windows.StringToUTF16Ptr(b.String()),
		windows.StringToUTF16Ptr(title),
		0,
	)
}
