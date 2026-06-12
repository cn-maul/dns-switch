package main

// BenchResult holds a single DNS server's benchmark result.
type BenchResult struct {
	Name   string
	IP     string
	AvgRTT float64
	Loss   int
	Err    bool
	ErrMsg string
}
