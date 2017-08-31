/*
Copyright 2017 The Kubernetes Authors.

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

package monitoring

import (
	"fmt"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	registration "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
	extensions "k8s.io/api/extensions/v1beta1"
)

var (
	replicas = int32(1)
	cpuNeed = resource.NewMilliQuantity(200, resource.DecimalSI)
	memNeed = resource.NewScaledQuantity(250, resource.Mega)
	CustomMetricName = "foo-metric"
	UnusedMetricName = "unused-metric"
	MetricValue1 = int64(448)
	MetricValue2 = int64(446)
	CustomMetricsServiceAccount = &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-metrics-stackdriver-adapter",
			Namespace: "default",
		},
	}
	CustomMetricsAdapterDeployment = &extensions.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-metrics-stackdriver-adapter",
			Namespace: "default",
			Labels: map[string]string{
				"run": "custom-metrics-stackdriver-adapter",
				"k8s-app": "custom-metrics-stackdriver-adapter",
			},
		},
		Spec: extensions.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"run": "custom-metrics-stackdriver-adapter",
					"k8s-app": "custom-metrics-stackdriver-adapter",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"run": "custom-metrics-stackdriver-adapter",
						"k8s-app": "custom-metrics-stackdriver-adapter",
						"kubernetes.io/cluster-service": "true",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "gcr.io/kawych-test/custom-metrics-stackdriver-adapter:v1.0",
							ImagePullPolicy: "Always",
							Name: "pod-custom-metrics-stackdriver-adapter",
							Command: []string{"/adapter", "--requestheader-client-ca-file=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"},
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									"cpu": *cpuNeed,
									"memory": *memNeed,
								},
								Requests: v1.ResourceList{
									"cpu": *cpuNeed,
									"memory": *memNeed,
								},
							},
						},
					},
				},
			},
		},
	}
	CustomMetricsAdapterService = &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-metrics-stackdriver-adapter",
			Namespace: "default",
			Labels: map[string]string{
				"run": "custom-metrics-stackdriver-adapter",
				"k8s-app": "custom-metrics-stackdriver-adapter",
				"kubernetes.io/cluster-service": "true",
		    	"kubernetes.io/name": "Adapter",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Port: 443,
					Protocol: v1.ProtocolTCP,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 443},
				},
			},
			Selector: map[string]string{
				"run": "custom-metrics-stackdriver-adapter",
				"k8s-app": "custom-metrics-stackdriver-adapter",
			},
			Type: v1.ServiceTypeClusterIP,
		},
	}
	CustomMetricsSystemAuthDelegator = &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-metrics:system:auth-delegator",
		},
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind: "ClusterRole",
			//Name: "system:auth-delegator",
			Name: "cluster-admin",
		},
		Subjects: []rbac.Subject{
			{
				APIGroup: "",
				Kind: "ServiceAccount",
				Name: "custom-metrics-stackdriver-adapter",
				Namespace: "default",
			},
			{
				APIGroup: "",
				Kind: "ServiceAccount",
				Name: "default",
				Namespace: "default",
			},
		},
	}
	/*CustomMetricsAuthenticationReader = &rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-metrics-authentication-readerro",
			Namespace: "default",
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{"custom-metrics.metrics.k8s.io"},
				Resources: []string{"*", "pods"},
				Verbs: []string{"list", "get", "watch"},
			},
		},
	}*/
	CustomMetricsAuthReader = &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-metrics-auth-reader",
			Namespace: "default",
		},
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind: "Role",
			Name: "custom-metrics-authentication-readerro",
		},
		Subjects: []rbac.Subject{
			{
				APIGroup: "",
				Kind: "ServiceAccount",
				Name: "custom-metrics-stackdriver-adapter",
				Namespace: "default",
			},
			{
				APIGroup: "",
				Kind: "ServiceAccount",
				Name: "default",
				Namespace: "default",
			},
		},
	}
	CustomMetricsAPIService = &registration.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "v1alpha1.custom-metrics.metrics.k8s.io",
		},
		Spec: registration.APIServiceSpec{
			InsecureSkipTLSVerify: true,
			Group: "custom-metrics.metrics.k8s.io",
			GroupPriorityMinimum: 100,
			VersionPriority: 100,
			Version: "v1alpha1",
			Service: &registration.ServiceReference{
				Name: "custom-metrics-stackdriver-adapter",
				Namespace: "default",
			},
		},
	}
	CustomMetricsReader = &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-metrics-reader",
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{"custom-metrics.metrics.k8s.io"},
				Resources: []string{"*"},
				Verbs: []string{"list", "get", "watch"},
			},
		},
	}
	CustomMetricsMetricsReader = &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "all-metrics-reader",
		},
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind: "ClusterRole",
			Name: "custom-metrics-reader",
		},
		Subjects: []rbac.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind: "Group",
				Name: "system:anonymous",
			},
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind: "Group",
				Name: "system:unauthenticated",
			},
			{
				APIGroup: "",
				Kind: "ServiceAccount",
				Name: "default",
				Namespace: "default",
			},
		},
	}
	CustomMetricsExtensionAuthReader = &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-metrics-authentication-reader",
			Namespace: "kube-system",
		},
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind: "Role",
			Name: "extension-apiserver-authentication-reader",
		},
		Subjects: []rbac.Subject{
			{
				APIGroup: "",
				Kind: "ServiceAccount",
				Name: "default",
				Namespace: "default",
			},
		},
	}
	MetricsExposerPod1 = &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "metrics-exposer-1",
			Namespace: "default",
			Labels: map[string]string{
				"name": "metric-exposer",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "metrics-exposer",
					Image: "gcr.io/kawych-test/metrics-exposer:v1.0",
					ImagePullPolicy: v1.PullPolicy("Always"),
					Command: []string{"/metrics_exposer", "--pod_id=$(POD_ID)", "--metric_name=" + CustomMetricName, fmt.Sprintf("--metric_value=%v", MetricValue1)},
					Env: []v1.EnvVar{
						{
							Name: "POD_ID",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{
									FieldPath: "metadata.uid",
								},
							},
						},
					},
					Ports: []v1.ContainerPort{{ContainerPort: 80}},
				},
			},
		},
	}
	MetricsExposerPod2 = &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "metrics-exposer-1",
			Namespace: "default",
			Labels: map[string]string{
				"name": "metric-exposer",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "metrics-exposer",
					Image: "gcr.io/kawych-test/metrics-exposer:v1.0",
					ImagePullPolicy: v1.PullPolicy("Always"),
					Command: []string{"/metrics_exposer", "--pod_id=$(POD_ID)", "--metric_name=" + CustomMetricName, fmt.Sprintf("--metric_value=%v", MetricValue2)},
					Env: []v1.EnvVar{
						{
							Name: "POD_ID",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{
									FieldPath: "metadata.uid",
								},
							},
						},
					},
					Ports: []v1.ContainerPort{{ContainerPort: 80}},
				},
			},
		},
	}
)
