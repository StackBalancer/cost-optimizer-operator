package metrics

import (
	"fmt"
	"math"

	optimizationv1 "github.com/stackbalancer/cost-optimizer-operator/api/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Recommendation struct {
	CPURequest    *resource.Quantity
	CPULimit      *resource.Quantity
	MemoryRequest *resource.Quantity
	MemoryLimit   *resource.Quantity
	Reason        string
	Confidence    float64
}

type Analyzer struct{}

func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

func (a *Analyzer) GenerateRecommendation(metrics *WorkloadMetrics, policy optimizationv1.Policy) (*Recommendation, error) {
	if len(metrics.Usage) == 0 {
		return nil, fmt.Errorf("no usage data available")
	}

	// Calculate CPU recommendation
	cpuRec, cpuReason := a.calculateCPURecommendation(metrics, policy.Cpu)

	// Calculate Memory recommendation
	memRec, memReason := a.calculateMemoryRecommendation(metrics, policy.Memory)

	confidence := a.calculateConfidence(metrics)

	return &Recommendation{
		CPURequest:    cpuRec.Request,
		CPULimit:      cpuRec.Limit,
		MemoryRequest: memRec.Request,
		MemoryLimit:   memRec.Limit,
		Reason:        fmt.Sprintf("CPU: %s, Memory: %s", cpuReason, memReason),
		Confidence:    confidence,
	}, nil
}

type cpuRecommendation struct {
	Request *resource.Quantity
	Limit   *resource.Quantity
}

type memoryRecommendation struct {
	Request *resource.Quantity
	Limit   *resource.Quantity
}

func (a *Analyzer) calculateCPURecommendation(metrics *WorkloadMetrics, policy optimizationv1.CPUPolicy) (*cpuRecommendation, string) {
	// Get peak and average usage
	var peak, total int64
	for _, usage := range metrics.Usage {
		cpu := usage.CPUUsage.MilliValue()
		if cpu > peak {
			peak = cpu
		}
		total += cpu
	}

	avg := total / int64(len(metrics.Usage))

	// Calculate target based on utilization policy
	targetCPU := int64(float64(avg) / (float64(policy.TargetUtilization) / 100.0))

	// Apply min/max constraints
	minCPU := resource.MustParse(policy.Min)
	maxCPU := resource.MustParse(policy.Max)

	if targetCPU < minCPU.MilliValue() {
		targetCPU = minCPU.MilliValue()
	}
	if targetCPU > maxCPU.MilliValue() {
		targetCPU = maxCPU.MilliValue()
	}

	// Set request to target, limit to 1.5x target (with peak consideration)
	request := resource.NewMilliQuantity(targetCPU, resource.DecimalSI)
	limitValue := int64(math.Max(float64(targetCPU)*1.5, float64(peak)*1.1))
	limit := resource.NewMilliQuantity(limitValue, resource.DecimalSI)

	reason := fmt.Sprintf("avg=%dm, peak=%dm, target=%dm", avg, peak, targetCPU)

	return &cpuRecommendation{
		Request: request,
		Limit:   limit,
	}, reason
}

func (a *Analyzer) calculateMemoryRecommendation(metrics *WorkloadMetrics, policy optimizationv1.MemoryPolicy) (*memoryRecommendation, string) {
	// Get peak memory usage
	var peak, total int64
	for _, usage := range metrics.Usage {
		mem := usage.MemoryUsage.Value()
		if mem > peak {
			peak = mem
		}
		total += mem
	}

	avg := total / int64(len(metrics.Usage))

	// Memory recommendation: peak + buffer percentage
	bufferMultiplier := 1.0 + (float64(policy.BufferPercent) / 100.0)
	targetMemory := int64(float64(peak) * bufferMultiplier)

	request := resource.NewQuantity(targetMemory, resource.BinarySI)
	limit := resource.NewQuantity(int64(float64(targetMemory)*1.2), resource.BinarySI) // 20% headroom for limit

	reason := fmt.Sprintf("avg=%s, peak=%s, buffer=%d%%",
		resource.NewQuantity(avg, resource.BinarySI).String(),
		resource.NewQuantity(peak, resource.BinarySI).String(),
		policy.BufferPercent)

	return &memoryRecommendation{
		Request: request,
		Limit:   limit,
	}, reason
}

func (a *Analyzer) calculateConfidence(metrics *WorkloadMetrics) float64 {
	// Confidence based on data points and variance
	dataPoints := len(metrics.Usage)

	if dataPoints < 5 {
		return 0.3 // Low confidence with few data points
	}
	if dataPoints < 20 {
		return 0.6 // Medium confidence
	}

	// Calculate variance for additional confidence scoring
	var cpuValues []float64
	for _, usage := range metrics.Usage {
		cpuValues = append(cpuValues, float64(usage.CPUUsage.MilliValue()))
	}

	variance := a.calculateVariance(cpuValues)

	// Lower variance = higher confidence
	if variance < 100 { // Low variance in CPU usage
		return 0.9
	}
	if variance < 500 {
		return 0.7
	}

	return 0.5 // Medium confidence for high variance
}

func (a *Analyzer) calculateVariance(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Calculate mean
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate variance
	var variance float64
	for _, v := range values {
		variance += math.Pow(v-mean, 2)
	}

	return variance / float64(len(values))
}
