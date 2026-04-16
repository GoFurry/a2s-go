# a2s-go Release Checklist

## Automated Gate

- `go test ./...`
- `go test -race ./...`
- `go test -bench BenchmarkCollect -benchmem ./internal/multipacket`
- `go test -bench BenchmarkReadBzip2 -benchmem ./internal/multipacket`
- `go test -bench BenchmarkCollectInfo -benchmem ./scanner`
- `go test -bench BenchmarkAcquireReleaseReadBuffer -benchmem ./internal/transport`

Benchmark regressions should be compared against [benchmark-baseline.md](benchmark-baseline.md). Investigate before release when:

- `internal/multipacket` latency regresses by more than 15%
- `scanner` latency regresses by more than 20%
- allocation counts increase by more than 10% in any tracked benchmark

## Manual Live Regression

Recommended command:

```bash
go run ./examples/live-regression -servers=1.2.3.4:27015,5.6.7.8 -mode=all -scanner=true
```

Recommended target mix:

- one modern Source server with normal `A2S_INFO`
- one server that requires challenge for `A2S_PLAYER`
- one server with a larger `A2S_RULES` payload
- one server known to return split packets
- one older or non-mainstream server implementation

Record these outcomes before release:

- `info` success/failure
- `players` success/failure
- `rules` success/failure
- `rules.Truncated` behavior
- scanner batch result consistency vs single-server probes
- timeout and error-code behavior for known bad or unreachable targets

## Compatibility Notes

Release notes should explicitly mention any observed caveats around:

- old game compatibility
- split packet handling
- compressed packet handling
- challenge refresh behavior
- IPv4-only scanner address support

## Ship Criteria

Okay to release when:

- automated gate passes
- no new regressions appear in live regression targets
- scanner and single-server paths agree on core success/failure outcomes
- benchmark changes are understood and accepted

Hold the release when:

- live targets show new decode/protocol regressions
- scanner batch behavior diverges from single probes without explanation
- allocation or latency regressions are unexplained
