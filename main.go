package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

type MetricsServer struct {
	client        *kubernetes.Clientset
	metricsClient *metricsclientset.Clientset
}

func NewMetricsServer() *MetricsServer {
	var config *rest.Config
	var err error

	config, err = rest.InClusterConfig()
	if err != nil {
		var kubeconfig string
		switch runtime.GOOS {
		case "windows":
			kubeconfig = filepath.Join(os.Getenv("USERPROFILE"), ".kube", "config")
		case "linux":
			kubeconfig = filepath.Join(os.Getenv("HOME"), ".config", "config")
		case "darwin":
			kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
		default:
			log.Fatalf("Unsupported OS: %v", runtime.GOOS)
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("Error creating K8s config: %v", err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating K8s client: %v", err)
	}

	metricsClient, err := metricsclientset.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Metrics client: %v", err)
	}

	return &MetricsServer{client: client, metricsClient: metricsClient}
}

// Fetch Node Metrics (CPU, RAM, Storage, Temperature)
func (m *MetricsServer) FetchNodeMetrics() (map[string]interface{}, error) {
	metrics := make(map[string]interface{})
	nodes, err := m.client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, node := range nodes.Items {
		nodeMetrics := make(map[string]string)
		for _, condition := range node.Status.Conditions {
			nodeMetrics[string(condition.Type)] = string(condition.Status)
		}
		nodeMetrics["CPU"] = node.Status.Capacity.Cpu().String()
		nodeMetrics["Memory"] = node.Status.Capacity.Memory().String()
		nodeMetrics["Storage"] = node.Status.Capacity.StorageEphemeral().String()
		metrics[node.Name] = nodeMetrics
	}
	return metrics, nil
}

// Fetch Pod Metrics (CPU, RAM Usage)
func (m *MetricsServer) FetchPodMetrics() (map[string]interface{}, error) {
	metrics := make(map[string]interface{})
	podMetrics, err := m.metricsClient.MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pod := range podMetrics.Items {
		containerMetrics := make(map[string]string)
		for _, container := range pod.Containers {
			containerMetrics[container.Name] = "CPU: " + container.Usage.Cpu().String() + " | RAM: " + container.Usage.Memory().String()
		}
		metrics[pod.Namespace+"/"+pod.Name] = containerMetrics
	}
	return metrics, nil
}

func (m *MetricsServer) FetchMetrics() (map[string]interface{}, error) {
	allMetrics := make(map[string]interface{})

	nodeMetrics, err := m.FetchNodeMetrics()
	if err != nil {
		return nil, err
	}
	allMetrics["nodes"] = nodeMetrics

	podMetrics, err := m.FetchPodMetrics()
	if err != nil {
		return nil, err
	}
	allMetrics["pods"] = podMetrics

	return allMetrics, nil
}

func (m *MetricsServer) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	data, err := m.FetchMetrics()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(data)
}

func main() {
	server := NewMetricsServer()
	http.HandleFunc("/metrics", server.MetricsHandler)

	srv := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Println("Metrics server started on :8080")
	log.Fatal(srv.ListenAndServe())
}
