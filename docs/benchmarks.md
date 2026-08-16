# Benchmarks

Run the Go benchmarks with:

```bash
go test -bench=. -benchmem ./...
```

The benchmark suite records routing, authentication, provider selection,
non-streaming, streaming, concurrency, allocations, and bytes allocated. Use
`benchstat` to compare changes and record Docker RSS separately for realistic
runtime measurements.
