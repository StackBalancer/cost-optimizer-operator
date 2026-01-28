package metrics

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/client/clientset/versioned"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type UsageData struct {
	CPUUsage    resource.Quantity
	MemoryUsage resource.Quantity
	Timestamp   time.Time
}

type WorkloadMetrics struct {
	Deployment *appsv1.Deployment
	Usage      []UsageData
}

type Collector struct {
	kubeClient    kubernetes.Interface
	metricsClient versioned.Interface
}

func NewCollector(kubeClient kubernetes.Interface, metricsClient versioned.Interface) *Collector {
	return &Collector{
		kubeClient:    kubeClient,
		metricsClient: metricsClient,
	}
}

func (c *Collector) CollectWorkloadMetrics(ctx context.Context, deployment *appsv1.Deployment) (*WorkloadMetrics, error) {
	log := logf.FromContext(ctx)

	// Get pod metrics for the deployment
	podMetrics, err := c.metricsClient.MetricsV1beta1().PodMetricses(deployment.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deployment.Spec.Selector),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod metrics: %w", err)
	}

	var totalCPU, totalMemory resource.Quantity
	var usageData []UsageData

	for _, podMetric := range podMetrics.Items {
		for _, container := range podMetric.Containers {
			totalCPU.Add(container.Usage["cpu"])
			totalMemory.Add(container.Usage["memory"])
		}

		usageData = append(usageData, UsageData{
			CPUUsage:    totalCPU,
			MemoryUsage: totalMemory,
			Timestamp:   podMetric.Timestamp.Time,
		})

		log.V(1).Info("Collected metrics",
			"pod", podMetric.Name,
			"cpu", totalCPU.String(),
			"memory", totalMemory.String())
	}

	return &WorkloadMetrics{
		Deployment: deployment,
		Usage:      usageData,
	}, nil
}

func (c *Collector) GetAverageUsage(metrics *WorkloadMetrics, duration time.Duration) (cpuAvg, memoryAvg resource.Quantity) {
	if len(metrics.Usage) == 0 {
		return
	}

	cutoff := time.Now().Add(-duration)
	var validSamples []UsageData

	for _, usage := range metrics.Usage {
		if usage.Timestamp.After(cutoff) {
			validSamples = append(validSamples, usage)
		}
	}

	if len(validSamples) == 0 {
		return
	}

	var totalCPU, totalMemory int64
	for _, sample := range validSamples {
		totalCPU += sample.CPUUsage.MilliValue()
		totalMemory += sample.MemoryUsage.Value()
	}

	cpuAvg = *resource.NewMilliQuantity(totalCPU/int64(len(validSamples)), resource.DecimalSI)
	memoryAvg = *resource.NewQuantity(totalMemory/int64(len(validSamples)), resource.BinarySI)

	return
}
