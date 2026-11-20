package metrics

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/client/clientset/versioned"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type UsageSample struct {
	CPUUsage    resource.Quantity
	MemoryUsage resource.Quantity
	Timestamp   time.Time
}

type WorkloadMetrics struct {
	Deployment *appsv1.Deployment
	Usage      map[string][]UsageSample
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

	// Get pods for the deployment
	pods, err := c.kubeClient.CoreV1().Pods(deployment.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deployment.Spec.Selector),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get pods: %w", err)
	}

	// Index pod name with it's phase
	podPhases := make(map[string]v1.PodPhase)
	for _, pod := range pods.Items {
		podPhases[pod.Name] = pod.Status.Phase
	}

	// Filter our containers for measurement from deployment template
	allowedContainers := make(map[string]bool)
	for _, container := range deployment.Spec.Template.Spec.Containers {
		allowedContainers[container.Name] = true
	}

	usageData := make(map[string][]UsageSample)

	for _, podMetric := range podMetrics.Items {
		var totalCPU, totalMemory resource.Quantity

		if v, ok := podPhases[podMetric.Name]; ok {
			if v != v1.PodRunning {
				continue // skip this pod entirely
			}
		} else {
			return nil, fmt.Errorf("No matching Pod found for : %s", podMetric.Name)
		}

		for _, container := range podMetric.Containers {
			if allowedContainers[container.Name] {
				usageData[container.Name] = append(usageData[container.Name], UsageSample{
					CPUUsage:    container.Usage[v1.ResourceCPU],
					MemoryUsage: container.Usage[v1.ResourceMemory],
					Timestamp:   podMetric.Timestamp.Time,
				})
			}

		}

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

func (c *Collector) GetAverageUsage(metrics *WorkloadMetrics, duration time.Duration) (cpuAvg, memoryAvg map[string]resource.Quantity) {
	if len(metrics.Usage) == 0 {
		return
	}

	cutoff := time.Now().Add(-duration)

	cpuAvg = make(map[string]resource.Quantity)
	memoryAvg = make(map[string]resource.Quantity)

	for ctr, usage := range metrics.Usage {
		var totalCPU, totalMemory, filteredSample int64
		for _, sample := range usage {
			if sample.Timestamp.After(cutoff) {
				totalCPU += sample.CPUUsage.MilliValue()
				totalMemory += sample.MemoryUsage.Value()
				filteredSample += 1
			}
		}
		if filteredSample == 0 {
			continue
		}

		cpuAvg[ctr] = *resource.NewMilliQuantity(totalCPU/int64(filteredSample), resource.DecimalSI)
		memoryAvg[ctr] = *resource.NewQuantity(totalMemory/int64(filteredSample), resource.BinarySI)
	}
	return
}
