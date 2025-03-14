package metrics

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/gofiber/fiber/v2"
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

	var totalMemory int64
	var totalCPU int64
	var totalStorage int64

	for _, node := range nodes.Items {
		nodeMetrics := make(map[string]string)
		for _, condition := range node.Status.Conditions {
			nodeMetrics[string(condition.Type)] = string(condition.Status)
		}
		cpu := node.Status.Capacity.Cpu().MilliValue()
		memory := node.Status.Capacity.Memory().Value()
		storage := node.Status.Capacity.StorageEphemeral().Value()

		nodeMetrics["CPU"] = fmt.Sprintf("%dm", cpu)
		nodeMetrics["Memory"] = fmt.Sprintf("%dMi", memory/(1024*1024))
		nodeMetrics["Storage"] = fmt.Sprintf("%dGi", storage/(1024*1024*1024))

		totalCPU += cpu
		totalMemory += memory
		totalStorage += storage

		metrics[node.Name] = nodeMetrics
	}

	metrics["totalCPU"] = fmt.Sprintf("%d cores", totalCPU/1000)
	metrics["totalMemory"] = fmt.Sprintf("%d GiB", totalMemory/(1024*1024*1024))
	metrics["totalStorage"] = fmt.Sprintf("%d GiB", totalStorage/(1024*1024*1024))

	return metrics, nil
}

// Fetch Node Specific Metrics and Pods
func (m *MetricsServer) FetchNodeSpecificMetrics(nodeName string) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})
	node, err := m.client.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	nodeMetrics := make(map[string]string)
	for _, condition := range node.Status.Conditions {
		nodeMetrics[string(condition.Type)] = string(condition.Status)
	}
	cpu := node.Status.Capacity.Cpu().MilliValue()
	memory := node.Status.Capacity.Memory().Value()
	storage := node.Status.Capacity.StorageEphemeral().Value()

	nodeMetrics["CPU"] = fmt.Sprintf("%dm", cpu)
	nodeMetrics["Memory"] = fmt.Sprintf("%dMi", memory/(1024*1024))
	nodeMetrics["Storage"] = fmt.Sprintf("%dGi", storage/(1024*1024*1024))

	metrics["nodeMetrics"] = nodeMetrics

	// Fetch Pods assigned to the node
	pods, err := m.client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		return nil, err
	}

	podMetrics := make(map[string]interface{})
	for _, pod := range pods.Items {
		containerMetrics := make(map[string]string)
		for _, container := range pod.Spec.Containers {
			containerMetrics[container.Name] = "CPU: " + container.Resources.Requests.Cpu().String() + " | RAM: " + container.Resources.Requests.Memory().String()
		}
		containerMetrics["Namespace"] = pod.Namespace
		podMetrics[pod.Name] = containerMetrics
	}

	metrics["podMetrics"] = podMetrics

	return metrics, nil
}

func (m *MetricsServer) FetchNodeTemperature(nodeName string) (map[string]interface{}, error) {
	temperatureMetrics := make(map[string]interface{})
	nodeExporterURL := fmt.Sprintf("http://%s:9100/metrics", nodeName) // Adjust the URL as needed

	resp, err := http.Get(nodeExporterURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch metrics from Node Exporter: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse the metrics to find temperature
	metrics := string(body)
	tempMatches := regexp.MustCompile(`node_hwmon_temp_celsius{.*} (\d+\.\d+)`).FindAllStringSubmatch(metrics, -1)

	for _, match := range tempMatches {
		if len(match) > 1 {
			temperatureMetrics[match[0]] = match[1] // Store the temperature value
		}
	}

	return temperatureMetrics, nil
}

// Fetch Pod Metrics (CPU, RAM Usage)
func (m *MetricsServer) FetchPodMetrics(namespace string) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})
	podMetrics, err := m.metricsClient.MetricsV1beta1().PodMetricses(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pod := range podMetrics.Items {
		containerMetrics := make(map[string]string)
		for _, container := range pod.Containers {
			containerMetrics[container.Name] = "CPU: " + container.Usage.Cpu().String() + " | RAM: " + container.Usage.Memory().String()
		}
		// Fetch the Pod object to get the node name
		podObj, err := m.client.CoreV1().Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		containerMetrics["Node"] = podObj.Spec.NodeName
		containerMetrics["Namespace"] = pod.Namespace
		metrics[pod.Name] = containerMetrics
	}
	return metrics, nil
}

func (m *MetricsServer) FetchMetrics(namespace string) (map[string]interface{}, error) {
	allMetrics := make(map[string]interface{})

	if namespace == metav1.NamespaceAll {
		nodeMetrics, err := m.FetchNodeMetrics()
		if err != nil {
			return nil, err
		}
		allMetrics["nodes"] = nodeMetrics
	}

	podMetrics, err := m.FetchPodMetrics(namespace)
	if err != nil {
		return nil, err
	}
	allMetrics["pods"] = podMetrics

	return allMetrics, nil
}

func (m *MetricsServer) MetricsHandler(c *fiber.Ctx) error {
	path := strings.TrimPrefix(c.Path(), "/metrics/")
	parts := strings.Split(path, "/")

	var data map[string]interface{}
	var err error

	if len(parts) > 1 && parts[0] == "node" {
		nodeName := parts[1]
		if len(parts) > 2 && parts[2] == "temperature" {
			data, err = m.FetchNodeTemperature(nodeName)
		} else {
			data, err = m.FetchNodeSpecificMetrics(nodeName)
		}
	} else {
		namespace := path
		if namespace == "" {
			namespace = metav1.NamespaceAll
		}
		data, err = m.FetchMetrics(namespace)
	}

	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString(err.Error())
	}
	return c.JSON(data)
}
