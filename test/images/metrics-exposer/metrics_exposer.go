package main

import (
	"fmt"
	"flag"
	"time"
	"log"

	gce "cloud.google.com/go/compute/metadata"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	v3 "google.golang.org/api/monitoring/v3"
	"strings"
)

func main() {
	// Gather pod information
	podIdFlag := flag.String("pod_id", "", "a string")
	metricNameFlag := flag.String("metric_name", "foo", "a string")
	metricValueFlag := flag.Int64("metric_value", 0, "an int")
	flag.Parse()
	project_id, _ := gce.ProjectID()
	zone, _ := gce.Zone()
	cluster_name, _ := gce.InstanceAttributeValue("cluster-name")
	cluster_name = strings.TrimSpace(cluster_name)
	container_name := ""
	pod_id := *podIdFlag
	metric_name := *metricNameFlag
	metric_value := *metricValueFlag

	oauthClient := oauth2.NewClient(context.Background(), google.ComputeTokenSource(""))
	stackdriverService, err := v3.New(oauthClient)
	if err != nil {
		log.Print("error: %s", err)
		return
	}

	for {
		// Prepare an individual data point
		dataPoint := &v3.Point{
			Interval: &v3.TimeInterval{
				EndTime: time.Now().Format(time.RFC3339),
			},
			Value: &v3.TypedValue{
				Int64Value: &metric_value,
			},
		}
		// Write time series data.
		request := &v3.CreateTimeSeriesRequest{
			TimeSeries: []*v3.TimeSeries{
				{
					Metric: &v3.Metric{
						Type: "custom.googleapis.com/" + metric_name,
					},
					Resource: &v3.MonitoredResource{
						Type: "gke_container",
						Labels: map[string]string{
							"project_id": project_id,
							"zone": zone,
							"cluster_name": cluster_name,
							"container_name": container_name,
							"pod_id": pod_id,
							// namespace_id and instance_id don't matter
							"namespace_id": "default",
							"instance_id": "",
						},
					},
					Points: []*v3.Point{
						dataPoint,
					},
				},
			},
		}
		_, err := stackdriverService.Projects.TimeSeries.Create(fmt.Sprintf("projects/%s", project_id), request).Do()
		if err != nil {
			log.Printf("Failed to write time series data: %v\n", err)
		} else {
			log.Printf("Finished writing time series with value: %v\n", metric_value)
		}
		time.Sleep(5000 * time.Millisecond)
	}
}

