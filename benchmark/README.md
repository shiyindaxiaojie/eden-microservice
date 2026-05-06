# Benchmark

This directory contains reproducible Go benchmarks for registry hot paths.

Current coverage:

- `BenchmarkConcurrentRegister/same-service`: concurrent registration into one service
- `BenchmarkConcurrentRegister/multi-service`: concurrent registration spread across 32 services

Run the benchmark:

```bash
go test ./benchmark -run ^$ -bench BenchmarkConcurrentRegister -benchmem
```

Use a fixed benchmark duration when comparing changes:

```bash
go test ./benchmark -run ^$ -bench BenchmarkConcurrentRegister -benchmem -benchtime=3s
```

The benchmark targets the in-process catalog registry so the result reflects concurrent registration cost without HTTP/gRPC transport noise.
