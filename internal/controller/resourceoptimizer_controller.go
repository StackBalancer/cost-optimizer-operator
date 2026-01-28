/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	optimizationv1 "github.com/stackbalancer/cost-optimizer-operator/api/v1"
	"github.com/stackbalancer/cost-optimizer-operator/internal/metrics"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

// ResourceOptimizerReconciler reconciles a ResourceOptimizer object
type ResourceOptimizerReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	recorder         record.EventRecorder
	metricsClient    metricsv.Interface
	kubeClient       kubernetes.Interface
	metricsCollector *metrics.Collector
	analyzer         *metrics.Analyzer
}

// +kubebuilder:rbac:groups=optimization.stackbalancer.io,resources=resourceoptimizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=optimization.stackbalancer.io,resources=resourceoptimizers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=optimization.stackbalancer.io,resources=resourceoptimizers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=metrics.k8s.io,resources=pods,verbs=get;list
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ResourceOptimizer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *ResourceOptimizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	resourceOptimizer := &optimizationv1.ResourceOptimizer{}
	if err := r.Get(ctx, req.NamespacedName, resourceOptimizer); err != nil {
		log.Error(err, "Failed to get resourceOptimizer")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("Reconciling resourceOptimizer", "targetRef", resourceOptimizer.Spec.TargetRef, "policy", resourceOptimizer.Spec.Policy)

	// Get target deployment
	deployment, err := r.getDeploymentObject(ctx, resourceOptimizer)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			addCondition(
				&resourceOptimizer.Status,
				"DeploymentReady",
				metav1.ConditionFalse,
				"TargetNotFound",
				"Target Deployment does not exist yet",
			)
			_ = r.updateStatus(ctx, resourceOptimizer)
			return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
		}
		return ctrl.Result{}, err
	}

	addCondition(
		&resourceOptimizer.Status,
		"DeploymentReady",
		metav1.ConditionTrue,
		"TargetFound",
		"Target Deployment exists",
	)

	// Collect metrics and analyze
	if err := r.analyzeAndOptimize(ctx, resourceOptimizer, deployment); err != nil {
		log.Error(err, "Failed to analyze workload")
		addCondition(
			&resourceOptimizer.Status,
			"OptimizationReady",
			metav1.ConditionFalse,
			"AnalysisFailed",
			err.Error(),
		)
		_ = r.updateStatus(ctx, resourceOptimizer)
		return ctrl.Result{RequeueAfter: time.Minute * 10}, nil
	}

	addCondition(&resourceOptimizer.Status, "Ready", metav1.ConditionTrue, "AllSubresourcesReady", "All subresources are ready")

	if err := r.updateStatus(ctx, resourceOptimizer); err != nil {
		log.Error(err, "Failed to update ResourceOptimizer status")
		return ctrl.Result{}, err
	}

	log.Info("Reconciliation complete")

	return ctrl.Result{RequeueAfter: time.Minute * 15}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceOptimizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("cost-optimizer-controller")

	// Initialize metrics components
	config := mgr.GetConfig()
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	r.kubeClient = kubeClient

	metricsClient, err := metricsv.NewForConfig(config)
	if err != nil {
		return err
	}
	r.metricsClient = metricsClient

	r.metricsCollector = metrics.NewCollector(kubeClient, metricsClient)
	r.analyzer = metrics.NewAnalyzer()

	return ctrl.NewControllerManagedBy(mgr).
		For(&optimizationv1.ResourceOptimizer{}).
		Named("resourceoptimizer").
		Complete(r)
}

// Function to add a condition to the ResourceOptimizerStatus
func addCondition(status *optimizationv1.ResourceOptimizerStatus, condType string, statusType metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()

	for i, existingCondition := range status.Conditions {
		if existingCondition.Type == condType {
			if existingCondition.Status != statusType {
				status.Conditions[i].LastTransitionTime = now
			}
			// Condition already exists, update it
			status.Conditions[i].Status = statusType
			status.Conditions[i].Reason = reason
			status.Conditions[i].Message = message
			status.Conditions[i].LastTransitionTime = metav1.Now()
			return
		}
	}

	// Condition does not exist, add it
	condition := metav1.Condition{
		Type:               condType,
		Status:             statusType,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	}
	status.Conditions = append(status.Conditions, condition)
}

// Function to update the status of the resourceOptimizer object
func (r *ResourceOptimizerReconciler) updateStatus(ctx context.Context, resourceOptimizer *optimizationv1.ResourceOptimizer) error {
	// Update the status of the resourceOptimizer object
	if err := r.Status().Update(ctx, resourceOptimizer); err != nil {
		return err
	}

	return nil
}

func (r *ResourceOptimizerReconciler) getDeploymentObject(ctx context.Context, resourceOptimizer *optimizationv1.ResourceOptimizer) (*appsv1.Deployment, error) {
	log := logf.FromContext(ctx)

	existingDeployment := &appsv1.Deployment{}
	objKey := client.ObjectKey{
		Namespace: resourceOptimizer.Spec.TargetRef.Namespace,
		Name:      resourceOptimizer.Spec.TargetRef.Name,
	}

	if err := r.Get(ctx, objKey, existingDeployment); err != nil {
		log.Info("Target Deployment not found yet",
			"namespace", resourceOptimizer.Spec.TargetRef.Namespace,
			"name", resourceOptimizer.Spec.TargetRef.Name)
		return nil, err
	}

	log.Info("Deployment found", "name", existingDeployment.Name)
	r.recorder.Event(resourceOptimizer, corev1.EventTypeNormal, "DeploymentFound", "Deployment found successfully")
	return existingDeployment, nil
}

func (r *ResourceOptimizerReconciler) analyzeAndOptimize(ctx context.Context, resourceOptimizer *optimizationv1.ResourceOptimizer, deployment *appsv1.Deployment) error {
	log := logf.FromContext(ctx)

	// Collect current metrics
	workloadMetrics, err := r.metricsCollector.CollectWorkloadMetrics(ctx, deployment)
	if err != nil {
		return err
	}

	if len(workloadMetrics.Usage) == 0 {
		log.Info("No metrics data available yet, skipping optimization")
		addCondition(
			&resourceOptimizer.Status,
			"OptimizationReady",
			metav1.ConditionFalse,
			"NoMetricsData",
			"Waiting for metrics data to be available",
		)
		return nil
	}

	// Generate recommendations
	recommendation, err := r.analyzer.GenerateRecommendation(workloadMetrics, resourceOptimizer.Spec.Policy)
	if err != nil {
		return err
	}

	log.Info("Generated optimization recommendation",
		"cpuRequest", recommendation.CPURequest.String(),
		"cpuLimit", recommendation.CPULimit.String(),
		"memoryRequest", recommendation.MemoryRequest.String(),
		"memoryLimit", recommendation.MemoryLimit.String(),
		"confidence", recommendation.Confidence,
		"reason", recommendation.Reason)

	// Record recommendation event
	r.recorder.Eventf(resourceOptimizer, corev1.EventTypeNormal, "RecommendationGenerated",
		"CPU: %s/%s, Memory: %s/%s (confidence: %.2f)",
		recommendation.CPURequest.String(),
		recommendation.CPULimit.String(),
		recommendation.MemoryRequest.String(),
		recommendation.MemoryLimit.String(),
		recommendation.Confidence)

	// Update status with recommendation
	resourceOptimizer.Status.CurrentRecommendation = &optimizationv1.ResourceRecommendation{
		CPU: optimizationv1.CPURecommendation{
			Request: recommendation.CPURequest.String(),
			Limit:   recommendation.CPULimit.String(),
		},
		Memory: optimizationv1.MemoryRecommendation{
			Request: recommendation.MemoryRequest.String(),
			Limit:   recommendation.MemoryLimit.String(),
		},
		Confidence:  int32(recommendation.Confidence * 100),
		Reason:      recommendation.Reason,
		GeneratedAt: metav1.Now(),
	}

	addCondition(
		&resourceOptimizer.Status,
		"OptimizationReady",
		metav1.ConditionTrue,
		"RecommendationGenerated",
		recommendation.Reason,
	)

	return nil
}
