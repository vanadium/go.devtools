// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package monitoring

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudmonitoring/v2beta2"
)

const (
	customMetricPrefix = "custom.cloudmonitoring.googleapis.com"
)

type ServiceLocation struct {
	Instance string
	Zone     string
}

var ServiceLocationMap = map[string]*ServiceLocation{
	"/ns.dev.v.io:8101": &ServiceLocation{
		Instance: "vanadium-cell-master",
		Zone:     "us-central1-c",
	},
	"/ns.dev.v.io:8151": &ServiceLocation{
		Instance: "vanadium-cell-master",
		Zone:     "us-central1-c",
	},
	"/ns.dev.staging.v.io:8101": &ServiceLocation{
		Instance: "vanadium-cell-master",
		Zone:     "us-central1-c",
	},
	"/ns.dev.staging.v.io:8151": &ServiceLocation{
		Instance: "vanadium-cell-master",
		Zone:     "us-central1-c",
	},
}

type labelData struct {
	key         string
	description string
}

// CustomMetricDescriptors is a map from metric's short names to their
// MetricDescriptor definitions.
var CustomMetricDescriptors = map[string]*cloudmonitoring.MetricDescriptor{
	// Custom metric for recording check latency of vanadium production services.
	"service-latency": createMetric("service/latency", "The check latency (ms) of vanadium production services.", "double", true, nil),

	// Custom metric for recording per-method rpc latency for a service.
	"service-permethod-latency": createMetric("service/latency/method", "Service latency (ms) per method.", "double", true, []labelData{
		labelData{
			key:         "method-name",
			description: "The method name",
		},
	}),

	// Custom metric for recording various counters of vanadium production services.
	"service-counters": createMetric("service/counters", "Various counters of vanadium production services.", "double", true, nil),

	// Custom metric for recording service metadata of vanadium production services.
	"service-metadata": createMetric("service/metadata", "Various metadata of vanadium production services.", "double", true, []labelData{
		labelData{
			key:         "metadata-name",
			description: "The metadata name",
		},
	}),

	// Custom metric for recording total rpc qps for a service.
	"service-qps-total": createMetric("service/qps/total", "Total service QPS.", "double", true, nil),

	// Custom metric for recording per-method rpc qps for a service.
	"service-qps-method": createMetric("service/qps/method", "Service QPS per method.", "double", true, []labelData{
		labelData{
			key:         "method-name",
			description: "The method name",
		},
	}),

	// Custom metric for recording gce instance stats.
	"gce-instance": createMetric("gce-instance/stats", "Various stats for GCE instances.", "double", true, nil),

	// Custom metric for recording nginx stats.
	"nginx": createMetric("nginx/stats", "Various stats for Nginx server.", "double", true, nil),

	// Custom metric for rpc load tests.
	"rpc-load-test": createMetric("rpc-load-test", "Results of rpc load test.", "double", false, nil),
}

func createMetric(metricType, description, valueType string, includeGCELabels bool, extraLabels []labelData) *cloudmonitoring.MetricDescriptor {
	labels := []*cloudmonitoring.MetricDescriptorLabelDescriptor{}
	if includeGCELabels {
		labels = append(labels, &cloudmonitoring.MetricDescriptorLabelDescriptor{
			Key:         fmt.Sprintf("%s/gce-instance", customMetricPrefix),
			Description: "The name of the GCE instance associated with this metric.",
		}, &cloudmonitoring.MetricDescriptorLabelDescriptor{
			Key:         fmt.Sprintf("%s/gce-zone", customMetricPrefix),
			Description: "The zone of the GCE instance associated with this metric.",
		})
	}
	labels = append(labels, &cloudmonitoring.MetricDescriptorLabelDescriptor{
		Key:         fmt.Sprintf("%s/metric-name", customMetricPrefix),
		Description: "The name of the metric.",
	})
	if extraLabels != nil {
		for _, data := range extraLabels {
			labels = append(labels, &cloudmonitoring.MetricDescriptorLabelDescriptor{
				Key:         fmt.Sprintf("%s/%s", customMetricPrefix, data.key),
				Description: data.description,
			})
		}
	}

	return &cloudmonitoring.MetricDescriptor{
		Name:        fmt.Sprintf("%s/vanadium/%s", customMetricPrefix, metricType),
		Description: description,
		TypeDescriptor: &cloudmonitoring.MetricDescriptorTypeDescriptor{
			MetricType: "gauge",
			ValueType:  valueType,
		},
		Labels: labels,
	}
}

func createClient(keyFilePath string) (*http.Client, error) {
	if len(keyFilePath) > 0 {
		data, err := ioutil.ReadFile(keyFilePath)
		if err != nil {
			return nil, err
		}
		conf, err := google.JWTConfigFromJSON(data, cloudmonitoring.MonitoringScope)
		if err != nil {
			return nil, fmt.Errorf("failed to create JWT config file: %v", err)
		}
		return conf.Client(oauth2.NoContext), nil
	}

	return google.DefaultClient(oauth2.NoContext, cloudmonitoring.MonitoringScope)
}

// Authenticate authenticates with the given JSON credentials file (or the
// default client if the file is not provided). If successful, it returns a
// service object that can be used in GCM API calls.
func Authenticate(keyFilePath string) (*cloudmonitoring.Service, error) {
	c, err := createClient(keyFilePath)
	if err != nil {
		return nil, err
	}
	s, err := cloudmonitoring.New(c)
	if err != nil {
		return nil, fmt.Errorf("New() failed: %v", err)
	}
	return s, nil
}
