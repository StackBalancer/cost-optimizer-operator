package metrics

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesAPIfake "k8s.io/client-go/kubernetes/fake"
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
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Name: "app"},
						{Name: "app2"},
					},
				},
			},
		},
	}

	metricsClient := fake.NewSimpleClientset()
	podsGVR := v1beta1.SchemeGroupVersion.WithResource("pods")
	corePodsGVR := v1.SchemeGroupVersion.WithResource("pods")
	if err := metricsClient.Tracker().Create(podsGVR, podMetrics1, namespaceName); err != nil {
		t.Fatalf("seed pod-1: %v", err)
	}
	if err := metricsClient.Tracker().Create(podsGVR, podMetrics2, namespaceName); err != nil {
		t.Fatalf("seed pod-2: %v", err)
	}

	kubeClient := kubernetesAPIfake.NewSimpleClientset()
	if err := kubeClient.Tracker().Create(corePodsGVR, &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: namespaceName,
			Labels:    map[string]string{"app": "test"},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}, namespaceName); err != nil {
		t.Fatalf("seed pod-1: %v", err)
	}
	if err := kubeClient.Tracker().Create(corePodsGVR, &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-2",
			Namespace: namespaceName,
			Labels:    map[string]string{"app": "test"},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}, namespaceName); err != nil {
		t.Fatalf("seed pod-2: %v", err)
	}
	collector := NewCollector(kubeClient, metricsClient)

	t.Run("single usage sample, no clamping", func(t *testing.T) {

		result, err := collector.CollectWorkloadMetrics(ctx, deployment)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantCPU := map[string]resource.Quantity{
			"app":  resource.MustParse("100m"),
			"app2": resource.MustParse("200m"),
		}
		wantMemory := map[string]resource.Quantity{
			"app":  resource.MustParse("256Mi"),
			"app2": resource.MustParse("512Mi"),
		}

		for ctr, usage := range result.Usage {
			wantCPUUsage := wantCPU[ctr]
			wantMemoryUsage := wantMemory[ctr]
			for _, sample := range usage {
				if sample.CPUUsage.Cmp(wantCPUUsage) != 0 {
					t.Errorf("%s CPU = %s, want %s", ctr, sample.CPUUsage.String(), wantCPUUsage.String())
				}

				if sample.MemoryUsage.Cmp(wantMemoryUsage) != 0 {
					t.Errorf("%s Memory = %s, want %s", ctr, sample.MemoryUsage.String(), wantMemoryUsage.String())
				}
			}
		}
	})

	t.Run("single pod with sidecar", func(t *testing.T) {
		podMetrics := &v1beta1.PodMetrics{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-1",
				Namespace: namespaceName,
				Labels:    map[string]string{"app": "test"},
			},
			Timestamp: metav1.Time{Time: time.Now()},
			Containers: []v1beta1.ContainerMetrics{
				{
					Name: "app",
					Usage: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("100m"),
						v1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
				{
					Name: "sidecar", // not in deployment's template, must be filtered out
					Usage: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("50m"),
						v1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
		}

		metricsClient := fake.NewSimpleClientset()
		if err := metricsClient.Tracker().Create(podsGVR, podMetrics, namespaceName); err != nil {
			t.Fatalf("seed pod-1: %v", err)
		}
		collector := NewCollector(kubeClient, metricsClient)

		result, err := collector.CollectWorkloadMetrics(ctx, deployment)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, ok := result.Usage["sidecar"]; ok {
			t.Errorf("unexpected sidecar container in result")
		}

		wantCPUUsage := resource.MustParse("100m")
		wantMemoryUsage := resource.MustParse("256Mi")

		for _, sample := range result.Usage["app"] {
			if sample.CPUUsage.Cmp(wantCPUUsage) != 0 {
				t.Errorf("app CPU = %s, want %s", sample.CPUUsage.String(), wantCPUUsage.String())
			}
			if sample.MemoryUsage.Cmp(wantMemoryUsage) != 0 {
				t.Errorf("app Memory = %s, want %s", sample.MemoryUsage.String(), wantMemoryUsage.String())
			}
		}
	})

	t.Run("single pod, two containers both in template", func(t *testing.T) {
		deploymentWithSidecar := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespaceName,
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{Name: "app"},
							{Name: "sidecar"},
						},
					},
				},
			},
		}

		podMetrics := &v1beta1.PodMetrics{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-1",
				Namespace: namespaceName,
				Labels:    map[string]string{"app": "test"},
			},
			Timestamp: metav1.Time{Time: time.Now()},
			Containers: []v1beta1.ContainerMetrics{
				{
					Name: "app",
					Usage: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("100m"),
						v1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
				{
					Name: "sidecar", // in the template this time, so it must NOT be filtered out
					Usage: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("50m"),
						v1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
		}

		metricsClient := fake.NewSimpleClientset()
		if err := metricsClient.Tracker().Create(podsGVR, podMetrics, namespaceName); err != nil {
			t.Fatalf("seed pod-1: %v", err)
		}
		collector := NewCollector(kubeClient, metricsClient)

		result, err := collector.CollectWorkloadMetrics(ctx, deploymentWithSidecar)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Usage) != 2 {
			t.Fatalf("got %d containers in result, want 2", len(result.Usage))
		}

		wantCPU := map[string]resource.Quantity{
			"app":     resource.MustParse("100m"),
			"sidecar": resource.MustParse("50m"),
		}
		wantMemory := map[string]resource.Quantity{
			"app":     resource.MustParse("256Mi"),
			"sidecar": resource.MustParse("128Mi"),
		}

		for ctr, usage := range result.Usage {
			wantCPUUsage := wantCPU[ctr]
			wantMemoryUsage := wantMemory[ctr]
			for _, sample := range usage {
				if sample.CPUUsage.Cmp(wantCPUUsage) != 0 {
					t.Errorf("%s CPU = %s, want %s", ctr, sample.CPUUsage.String(), wantCPUUsage.String())
				}
				if sample.MemoryUsage.Cmp(wantMemoryUsage) != 0 {
					t.Errorf("%s Memory = %s, want %s", ctr, sample.MemoryUsage.String(), wantMemoryUsage.String())
				}
			}
		}
	})
}
