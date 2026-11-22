package metrics

import (
	"testing"

	optimizationv1 "github.com/stackbalancer/cost-optimizer-operator/api/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestGenerateRecommendation(t *testing.T) {
	analyzer := NewAnalyzer()

	t.Run("single usage sample, no clamping", func(t *testing.T) {
		metrics := &WorkloadMetrics{
			Usage: map[string][]UsageSample{
				"app": {
					UsageSample{
						CPUUsage:    resource.MustParse("100m"),
						MemoryUsage: resource.MustParse("50Mi"),
					},
				},
			},
		}
		policy := optimizationv1.Policy{
			Cpu:    optimizationv1.CPUPolicy{Min: "50m", Max: "500m", TargetUtilization: 70},
			Memory: optimizationv1.MemoryPolicy{BufferPercent: 20},
		}

		got, err := analyzer.GenerateRecommendation(metrics, policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantCPURequest := resource.MustParse("142m") // 100m / 0.70 = 142.8m, truncated
		wantMemRequest := resource.MustParse("60Mi") // 50Mi * 1.2 buffer

		if got["app"].CPURequest.Cmp(wantCPURequest) != 0 {
			t.Errorf("CPURequest = %s, want %s", got["app"].CPURequest.String(), wantCPURequest.String())
		}
		if got["app"].MemoryRequest.Cmp(wantMemRequest) != 0 {
			t.Errorf("MemoryRequest = %s, want %s", got["app"].MemoryRequest.String(), wantMemRequest.String())
		}
		if got["app"].Confidence != 0.3 {
			t.Errorf("Confidence = %v, want 0.3", got["app"].Confidence) // calculateConfidence: <5 data points -> 0.3
		}
	})

	t.Run("many usage samples", func(t *testing.T) {
		metrics := &WorkloadMetrics{
			Usage: map[string][]UsageSample{
				"app": {
					{CPUUsage: resource.MustParse("100m"), MemoryUsage: resource.MustParse("50Mi")},
					{CPUUsage: resource.MustParse("150m"), MemoryUsage: resource.MustParse("100Mi")},
					{CPUUsage: resource.MustParse("200m"), MemoryUsage: resource.MustParse("150Mi")},
				},
			},
		}
		policy := optimizationv1.Policy{
			Cpu:    optimizationv1.CPUPolicy{Min: "50m", Max: "500m", TargetUtilization: 70},
			Memory: optimizationv1.MemoryPolicy{BufferPercent: 20},
		}

		got, err := analyzer.GenerateRecommendation(metrics, policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantCPURequest := resource.MustParse("214m")  // (100m + 150m + 200m) / 3 / 0.70 = 214.2m, truncated
		wantMemRequest := resource.MustParse("180Mi") // 150Mi (peak) * 1.2 buffer

		if got["app"].CPURequest.Cmp(wantCPURequest) != 0 {
			t.Errorf("CPURequest = %s, want %s", got["app"].CPURequest.String(), wantCPURequest.String())
		}
		if got["app"].MemoryRequest.Cmp(wantMemRequest) != 0 {
			t.Errorf("MemoryRequest = %s, want %s", got["app"].MemoryRequest.String(), wantMemRequest.String())
		}
		if got["app"].Confidence != 0.3 {
			t.Errorf("Confidence = %v, want 0.3", got["app"].Confidence) // calculateConfidence: <5 data points -> 0.3
		}
	})

	t.Run("zero usage", func(t *testing.T) {
		metrics := &WorkloadMetrics{
			Usage: map[string][]UsageSample{},
		}
		policy := optimizationv1.Policy{
			Cpu:    optimizationv1.CPUPolicy{Min: "50m", Max: "500m", TargetUtilization: 70},
			Memory: optimizationv1.MemoryPolicy{BufferPercent: 20},
		}

		got, err := analyzer.GenerateRecommendation(metrics, policy)
		if err == nil {
			t.Errorf("expected an error for empty usage data, got nil")
		}

		if got != nil {
			t.Errorf("expected recommendation is nil, got non-nil value")
		}
	})

	t.Run("min clamping", func(t *testing.T) {
		metrics := &WorkloadMetrics{
			Usage: map[string][]UsageSample{
				"app": {
					{CPUUsage: resource.MustParse("15m"), MemoryUsage: resource.MustParse("50Mi")},
				},
			},
		}
		policy := optimizationv1.Policy{
			Cpu:    optimizationv1.CPUPolicy{Min: "50m", Max: "500m", TargetUtilization: 70},
			Memory: optimizationv1.MemoryPolicy{BufferPercent: 20},
		}

		got, err := analyzer.GenerateRecommendation(metrics, policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantCPURequest := resource.MustParse(policy.Cpu.Min)

		if got["app"].CPURequest.Cmp(wantCPURequest) != 0 {
			t.Errorf("CPURequest = %s, want %s", got["app"].CPURequest.String(), wantCPURequest.String())
		}
	})

	t.Run("max clamping", func(t *testing.T) {
		metrics := &WorkloadMetrics{
			Usage: map[string][]UsageSample{
				"app": {
					{CPUUsage: resource.MustParse("500m"), MemoryUsage: resource.MustParse("50Mi")},
				},
			},
		}
		policy := optimizationv1.Policy{
			Cpu:    optimizationv1.CPUPolicy{Min: "50m", Max: "500m", TargetUtilization: 70},
			Memory: optimizationv1.MemoryPolicy{BufferPercent: 20},
		}

		got, err := analyzer.GenerateRecommendation(metrics, policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantCPURequest := resource.MustParse(policy.Cpu.Max)

		if got["app"].CPURequest.Cmp(wantCPURequest) != 0 {
			t.Errorf("CPURequest = %s, want %s", got["app"].CPURequest.String(), wantCPURequest.String())
		}
	})

	t.Run("invalid min quantity", func(t *testing.T) {
		metrics := &WorkloadMetrics{
			Usage: map[string][]UsageSample{
				"app": {
					{CPUUsage: resource.MustParse("100m"), MemoryUsage: resource.MustParse("50Mi")},
				},
			},
		}
		policy := optimizationv1.Policy{
			Cpu:    optimizationv1.CPUPolicy{Min: "not-a-quantity", Max: "500m", TargetUtilization: 70},
			Memory: optimizationv1.MemoryPolicy{BufferPercent: 20},
		}

		got, err := analyzer.GenerateRecommendation(metrics, policy)
		if err == nil {
			t.Fatal("expected an error for an invalid min quantity, got nil")
		}
		if got != nil {
			t.Errorf("expected recommendation to be nil, got %v", got)
		}
	})

	t.Run("min greater than max", func(t *testing.T) {
		metrics := &WorkloadMetrics{
			Usage: map[string][]UsageSample{
				"app": {
					{CPUUsage: resource.MustParse("100m"), MemoryUsage: resource.MustParse("50Mi")},
				},
			},
		}
		policy := optimizationv1.Policy{
			Cpu:    optimizationv1.CPUPolicy{Min: "500m", Max: "100m", TargetUtilization: 70},
			Memory: optimizationv1.MemoryPolicy{BufferPercent: 20},
		}

		got, err := analyzer.GenerateRecommendation(metrics, policy)
		if err == nil {
			t.Fatal("expected an error when min > max, got nil")
		}
		if got != nil {
			t.Errorf("expected recommendation to be nil, got %v", got)
		}
	})
}
