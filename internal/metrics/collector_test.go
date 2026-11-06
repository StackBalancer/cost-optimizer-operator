package metrics

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	fake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func TestCollectWorkloadMetrics(t *testing.T) {
	const namespaceName = "test"

	ctx := context.Background()

	podMetrics1 := &v1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",       // arbitrary, just an identifier
			Namespace: namespaceName, // must match deployment.Namespace in your test
			Labels:    map[string]string{"app": "test"},
		},
		Timestamp: metav1.Time{Time: time.Now()},
		Containers: []v1beta1.ContainerMetrics{
			{
				Name: "app", // arbitrary too — collector doesn't filter by name yet
				Usage: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("100m"),
					v1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		},
	}
	podMetrics2 := &v1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-2",       // arbitrary, just an identifier
			Namespace: namespaceName, // must match deployment.Namespace in your test
			Labels:    map[string]string{"app": "test"},
		},
		Timestamp: metav1.Time{Time: time.Now()},
		Containers: []v1beta1.ContainerMetrics{
			{
				Name: "app2", // arbitrary too — collector doesn't filter by name yet
				Usage: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("200m"),
					v1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		},
	}

	metricsClient := fake.NewSimpleClientset()
	podsGVR := v1beta1.SchemeGroupVersion.WithResource("pods")
	if err := metricsClient.Tracker().Create(podsGVR, podMetrics1, namespaceName); err != nil {
		t.Fatalf("seed pod-1: %v", err)
	}
	if err := metricsClient.Tracker().Create(podsGVR, podMetrics2, namespaceName); err != nil {
		t.Fatalf("seed pod-2: %v", err)
	}

	collector := NewCollector(nil, metricsClient)

	t.Run("single usage sample, no clamping", func(t *testing.T) {

		result, err := collector.CollectWorkloadMetrics(ctx, deployment)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantCPUPod1 := resource.MustParse("100m")
		wantCPUPod2 := resource.MustParse("200m")

		if result.Usage[0].CPUUsage.Cmp(wantCPUPod1) != 0 {
			t.Errorf("totalCPUPod1 = %s, want %s", result.Usage[0].CPUUsage.String(), wantCPUPod1.String())
		}

		if result.Usage[1].CPUUsage.Cmp(wantCPUPod2) != 0 {
			t.Errorf("totalCPUPod2 = %s, want %s", result.Usage[1].CPUUsage.String(), wantCPUPod2.String())
		}

		wantMemoryPod1 := resource.MustParse("256Mi")
		wantMemoryPod2 := resource.MustParse("512Mi")

		if result.Usage[0].MemoryUsage.Cmp(wantMemoryPod1) != 0 {
			t.Errorf("totalMemoryPod1 = %s, want %s", result.Usage[0].MemoryUsage.String(), wantMemoryPod1.String())
		}

		if result.Usage[1].MemoryUsage.Cmp(wantMemoryPod2) != 0 {
			t.Errorf("totalMemoryPod2 = %s, want %s", result.Usage[1].MemoryUsage.String(), wantMemoryPod2.String())
		}
	})

}
