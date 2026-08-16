package main

import (
	"testing"
	"time"
)

func TestPercentileUsesSortedValues(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	if got := percentile(values, 0.95); got != 4 {
		t.Fatalf("p95 = %v, want 4", got)
	}
}

func TestSummarizeSeparatesErrors(t *testing.T) {
	result := summarize([]result{
		{DurationMS: 2, Bytes: 10, FirstByteMS: 1},
		{DurationMS: 4, Bytes: 20, FirstByteMS: 2},
		{Error: "timeout"},
	}, time.Second)
	if result.Completed != 2 || result.Errors != 1 {
		t.Fatalf("summary counts = %d completed, %d errors", result.Completed, result.Errors)
	}
	if result.TotalBytes != 30 || result.P50FirstByteMS != 1 {
		t.Fatalf("summary values = %+v", result)
	}
}
