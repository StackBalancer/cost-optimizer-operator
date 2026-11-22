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

func (a *Analyzer) GenerateRecommendation(metrics *WorkloadMetrics, policy optimizationv1.Policy) (map[string]*Recommendation, error) {
	if len(metrics.Usage) == 0 {
		return nil, fmt.Errorf("no usage data available")
	}

	recs := make(map[string]*Recommendation)
	for ctr, usage := range metrics.Usage {
		// Calculate CPU recommendation
		cpuRec, err := a.calculateCPURecommendation(usage, policy.Cpu)
		if err != nil {
			return nil, fmt.Errorf("container %q: %w", ctr, err)
		}

		// Calculate Memory recommendation
		memRec := a.calculateMemoryRecommendation(usage, policy.Memory)
		confidence := a.calculateConfidence(usage)
		recs[ctr] = &Recommendation{
			CPURequest:    cpuRec.Request,
			CPULimit:      cpuRec.Limit,
			MemoryRequest: memRec.Request,
			MemoryLimit:   memRec.Limit,
			Reason:        fmt.Sprintf("CPU: %s, Memory: %s", cpuRec.Reason, memRec.Reason),
			Confidence:    confidence,
		}
	}

	return recs, nil
}

type cpuRecommendation struct {
	Request *resource.Quantity
	Limit   *resource.Quantity
	Reason  string
}

type memoryRecommendation struct {
	Request *resource.Quantity
	Limit   *resource.Quantity
	Reason  string
}

func (a *Analyzer) calculateCPURecommendation(usage []UsageSample, policy optimizationv1.CPUPolicy) (*cpuRecommendation, error) {
	// Get peak and average usage

	var peak, total, sampleCount int64
	for _, sample := range usage {
		cpu := sample.CPUUsage.MilliValue()
		if cpu > peak {
			peak = cpu
		}
		total += cpu
		sampleCount += 1
	}
	avg := int64(total / int64(sampleCount))
	// Calculate target based on utilization policy
	targetCPU := int64(float64(total/sampleCount) / (float64(policy.TargetUtilization) / 100.0))
	// Apply min/max constraints
	minCPU, err := resource.ParseQuantity(policy.Min)
	if err != nil {
		return nil, fmt.Errorf("invalid cpu policy min %q: %w", policy.Min, err)
	}
	maxCPU, err := resource.ParseQuantity(policy.Max)
	if err != nil {
		return nil, fmt.Errorf("invalid cpu policy max %q: %w", policy.Max, err)
	}
	if minCPU.MilliValue() > maxCPU.MilliValue() {
		return nil, fmt.Errorf("cpu policy min %s is greater than max %s", policy.Min, policy.Max)
	}

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
		Reason:  reason,
	}, nil
}

func (a *Analyzer) calculateMemoryRecommendation(usage []UsageSample, policy optimizationv1.MemoryPolicy) *memoryRecommendation {
	// Get peak memory usage
	var peak, total int64

	for _, sample := range usage {
		mem := sample.MemoryUsage.Value()
		if mem > peak {
			peak = mem
		}
		total += mem
	}

	avg := total / int64(len(usage))

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
		Reason:  reason,
	}
}

func (a *Analyzer) calculateConfidence(usage []UsageSample) float64 {
	// Confidence based on data points and variance
	dataPoints := len(usage)

	if dataPoints < 5 {
		return 0.3 // Low confidence with few data points
	}
	if dataPoints < 20 {
		return 0.6 // Medium confidence
	}

	// Calculate variance for additional confidence scoring
	var cpuValues []float64

	for _, sample := range usage {
		cpuValues = append(cpuValues, float64(sample.CPUUsage.MilliValue()))
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
