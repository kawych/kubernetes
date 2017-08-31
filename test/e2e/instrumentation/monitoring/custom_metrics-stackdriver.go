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
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	clientset "k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo"
	"k8s.io/kubernetes/test/e2e/framework"
	instrumentation "k8s.io/kubernetes/test/e2e/instrumentation/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gcm "google.golang.org/api/monitoring/v3"
	kubeaggrcs "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"k8s.io/apimachinery/pkg/runtime/schema"
	customclient "k8s.io/metrics/pkg/client/custom_metrics"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/discovery"
)

var _ = instrumentation.SIGDescribe("Stackdriver Monitoring", func() {
	BeforeEach(func() {
		framework.SkipUnlessProviderIs("gce", "gke")
	})

	f := framework.NewDefaultFramework("stackdriver-monitoring")
	var kubeClient clientset.Interface
	var kubeAggrClient kubeaggrcs.Interface
	var customMetricsClient customclient.CustomMetricsClient
	var discoveryClient *discovery.DiscoveryClient

	// TODO: CHANGE TO Feature:StackdriverMonitoring
	It("should run Custom Metrics - Stackdriver Adapter [Feature:StackdriverMonitoringo]", func() {
		kubeClient = f.ClientSet
		kubeAggrClient = f.AggregatorClient
		config, err := framework.LoadConfig()
		if err != nil {
			framework.Failf("Failed to load config: %s", err)
		}
		customMetricsClient = customclient.NewForConfigOrDie(config)
		discoveryClient = discovery.NewDiscoveryClientForConfigOrDie(config)
		testAdapter(f, kubeClient, kubeAggrClient, customMetricsClient, discoveryClient)
	})
})

func testAdapter(f *framework.Framework, kubeClient clientset.Interface, kubeAggrClient kubeaggrcs.Interface, customMetricsClient customclient.CustomMetricsClient, discoveryClient *discovery.DiscoveryClient) {
	projectId := framework.TestContext.CloudConfig.ProjectID

	//ctx := context.Background()
	//client, err := google.DefaultClient(ctx, gcm.CloudPlatformScope)

	// Hack for running tests locally
	// If this is your use case, create application default credentials:
	// $ gcloud auth application-default login
	// and uncomment following lines:
	ts, err := google.DefaultTokenSource(oauth2.NoContext)
	framework.Logf("Couldn't get application default credentials, %v", err)
	if err != nil {
		framework.Failf("Error accessing application default credentials, %v", err)
	}
	client := oauth2.NewClient(oauth2.NoContext, ts)

	gcmService, err := gcm.New(client)
	if err != nil {
		framework.Failf("Failed to create gcm service, %v", err)
	}

	framework.ExpectNoError(err)

	// Set up a cluster: create a custom metric and set up k8s-sd adapter
	err = createDescriptors(gcmService, projectId)
	if err != nil {
		framework.Failf("Failed to create metric descriptor: %s", err)
	}
	defer cleanupDescriptors(gcmService, projectId)

	err = createAdapter(kubeClient, kubeAggrClient)
	if err != nil {
		framework.Failf("Failed to set up: %s", err)
	}
	defer cleanupAdapter(kubeClient, kubeAggrClient)

	// Run application that exports the metric
	err = createMetricExposerPod(kubeClient)
	if err != nil {
		framework.Failf("Failed to create metric-exposer pod: %s", err)
	}
	defer cleanupMetricExposerPod(kubeClient)

	// Wait a short amount of time to create a pod and export some metrics
	time.Sleep(60 * time.Second)

	// Verify responses from Custom Metrics API
	resources, err := discoveryClient.ServerResourcesForGroupVersion("custom-metrics.metrics.k8s.io/v1alpha1")
	if err != nil {
		framework.Failf("Failed to retrieve a list of supported metrics: %s", err)
	}
	for _, resource := range resources.APIResources {
		if resource.Name != "pods/" + CustomMetricName && resource.Name != "pods/" + UnusedMetricName {
			framework.Failf("Unexpected metric %s. Only metric %s should be supported", resource.Name, CustomMetricName)
		}
	}
	value, err := customMetricsClient.NamespacedMetrics("default").GetForObject(schema.GroupKind{Group: "", Kind: "Pod"}, "metrics-exposer-1", CustomMetricName)
	if err != nil {
		framework.Failf("Failed query: %s", err)
	}
	if value.Value.Value() != MetricValue1 {
		framework.Failf("Unexpected metric value for metric %s: expected %v but received %v", CustomMetricName, MetricValue1, value.Value)
	}
	filter, err := labels.NewRequirement("name", selection.Equals, []string{"metric-exporter"})
	if err != nil {
		framework.Failf("Couldn't create a label filter")
	}
	values, err := customMetricsClient.NamespacedMetrics("default").GetForObjects(schema.GroupKind{Group: "", Kind: "Pod"}, labels.NewSelector().Add(*filter), CustomMetricName)
	if err != nil {
		framework.Failf("Failed query: %s", err)
	}
	if len(values.Items) != 2 {
		framework.Failf("Expected results for exactly 2 pods, but %v results received", len(values.Items))
	}
	for _, value := range values.Items {
		if (value.DescribedObject.Name != MetricsExposerPod1.Name && value.Value.Value() != MetricValue1) ||
				(value.DescribedObject.Name != MetricsExposerPod2.Name && value.Value.Value() != MetricValue2) {
			framework.Failf("Unexpected metric value for metric %s and pod %s: %v", CustomMetricName, value.DescribedObject.Name, value.Value)
		}
	}

	framework.ExpectNoError(err)
}

func createDescriptors(service *gcm.Service, projectId string) error {
	_, err := service.Projects.MetricDescriptors.Create(fmt.Sprintf("projects/%s", projectId), &gcm.MetricDescriptor{
		Name: CustomMetricName,
		ValueType: "INT64",
		Type: "custom.googleapis.com/" + CustomMetricName,
		MetricKind: "GAUGE",
	}).Do()
	if err != nil {
		return err
	}
	_, err = service.Projects.MetricDescriptors.Create(fmt.Sprintf("projects/%s", projectId), &gcm.MetricDescriptor{
		Name: UnusedMetricName,
		ValueType: "INT64",
		Type: "custom.googleapis.com/" + UnusedMetricName,
		MetricKind: "GAUGE",
	}).Do()
	return err
}

func createMetricExposerPod(cs clientset.Interface) error {
	_, err := cs.Core().Pods("default").Create(MetricsExposerPod1)
	if err != nil {
		return err
	}
	_, err = cs.Core().Pods("default").Create(MetricsExposerPod2)
	return err
}

func createAdapter(kubeClient clientset.Interface, kubeAggrClient kubeaggrcs.Interface) error {
	var err error
	_, err = kubeClient.RbacV1().RoleBindings("kube-system").Create(CustomMetricsExtensionAuthReader)
	if err != nil {
		framework.Failf("Failed to create role binding: %s", err)
	}
	_, err = kubeClient.Core().ServiceAccounts("default").Create(CustomMetricsServiceAccount)
	if err != nil {
		return err
	}
	_, err = kubeClient.ExtensionsV1beta1().Deployments("default").Create(CustomMetricsAdapterDeployment)
	if err != nil {
		return err
	}
	_, err = kubeClient.Core().Services("default").Create(CustomMetricsAdapterService)
	if err != nil {
		return err
	}
	_, err = kubeClient.RbacV1().ClusterRoleBindings().Create(CustomMetricsSystemAuthDelegator)
	if err != nil {
		return err
	}
	/*_, err = kubeClient.RbacV1().Roles("default").Create(CustomMetricsAuthenticationReader)
	if err != nil {
		return err
	}*/
	_, err = kubeClient.RbacV1().RoleBindings("default").Create(CustomMetricsAuthReader)
	if err != nil {
		return err
	}
	_, err = kubeAggrClient.ApiregistrationV1beta1().APIServices().Create(CustomMetricsAPIService)
	if err != nil {
		return err
	}
	_, err = kubeClient.RbacV1().ClusterRoles().Create(CustomMetricsReader)
	if err != nil {
		return err
	}
	_, err = kubeClient.RbacV1().ClusterRoleBindings().Create(CustomMetricsMetricsReader)
	if err != nil {
		return err
	}
	return nil
}

func cleanupDescriptors(service *gcm.Service, projectId string) {
	_, _ = service.Projects.MetricDescriptors.Delete(fmt.Sprintf("projects/%s/metricDescriptors/custom.googleapis.com/%s", projectId, CustomMetricName)).Do()
	_, _ = service.Projects.MetricDescriptors.Delete(fmt.Sprintf("projects/%s/metricDescriptors/custom.googleapis.com/%s", projectId, UnusedMetricName)).Do()
}

func cleanupMetricExposerPod(cs clientset.Interface) {
	_ = cs.Core().Pods("default").Delete("metric-exposer-1", &metav1.DeleteOptions{})
	_ = cs.Core().Pods("default").Delete("metric-exposer-2", &metav1.DeleteOptions{})
}

func cleanupAdapter(cs clientset.Interface, cs2 kubeaggrcs.Interface) error {
	_ = cs.RbacV1().ClusterRoleBindings().Delete(CustomMetricsMetricsReader.Name, &metav1.DeleteOptions{})
	_ = cs.RbacV1().ClusterRoles().Delete(CustomMetricsReader.Name, &metav1.DeleteOptions{})
	_ = cs2.ApiregistrationV1beta1().APIServices().Delete(CustomMetricsAPIService.Name, &metav1.DeleteOptions{})
	_ = cs.RbacV1().RoleBindings("default").Delete(CustomMetricsAuthReader.Name, &metav1.DeleteOptions{})
	_ = cs.RbacV1().ClusterRoleBindings().Delete(CustomMetricsSystemAuthDelegator.Name, &metav1.DeleteOptions{})
	_ = cs.Core().Services("default").Delete(CustomMetricsAdapterService.Name, &metav1.DeleteOptions{})
	_ = cs.ExtensionsV1beta1().Deployments("default").Delete(CustomMetricsAdapterDeployment.Name, &metav1.DeleteOptions{})
	_ = cs.Core().ServiceAccounts("default").Delete(CustomMetricsServiceAccount.Name, &metav1.DeleteOptions{})
	_ = cs.RbacV1().RoleBindings("kube-system").Delete(CustomMetricsExtensionAuthReader.Name, &metav1.DeleteOptions{})
	return nil
}
