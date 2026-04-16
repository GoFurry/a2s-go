# a2s-go Benchmark Baseline

## Current Commands

- `go test -bench BenchmarkCollect -benchmem ./internal/multipacket`
- `go test -bench BenchmarkReadBzip2 -benchmem ./internal/multipacket`
- `go test -bench BenchmarkCollectInfo -benchmem ./scanner`
- `go test -bench BenchmarkAcquireReleaseReadBuffer -benchmem ./internal/transport`

## Current Results

Recorded on Windows amd64, Intel Core i7-1360P:

- `BenchmarkCollectUncompressed`: `18072 ns/op`, `220 B/op`, `6 allocs/op`
- `BenchmarkCollectCompressed`: `1061334 ns/op`, `3610707 B/op`, `23 allocs/op`
- `BenchmarkReadBzip2`: `1222770 ns/op`, `3608237 B/op`, `12 allocs/op`
- `BenchmarkCollectInfo`: `1690219 ns/op`, `23476 B/op`, `548 allocs/op`
- `BenchmarkAcquireReleaseReadBuffer`: `83.01 ns/op`, `24 B/op`, `1 allocs/op`

## Regression Thresholds

- Investigate `internal/multipacket` latency changes above 15%.
- Investigate `scanner` latency changes above 20%.
- Investigate allocation-count changes above 10% in any tracked benchmark.
- Update this file when an intentional optimization or tradeoff changes the accepted baseline.

## Zero-Copy Assessment

The current `internal/multipacket` hot path already avoids retaining oversized UDP read buffers and only copies the exact payload needed for assembly. Additional zero-copy reuse was evaluated and deferred for now:

- split-packet assembly still needs one contiguous output buffer for downstream decoders
- compressed responses must allocate a decompressed output buffer by design
- more aggressive slice or object reuse would make the code more stateful without clear wins unless the benchmarks regress

Adopt more reuse only if the tracked benchmarks show a measurable bottleneck in real scanner workloads.
