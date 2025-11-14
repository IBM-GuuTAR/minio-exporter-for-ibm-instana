package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	minioURL       = getenv("MINIO_URL", "http://localhost:9000")
	minioAccessKey = getenv("MINIO_ACCESS_KEY", "minioaccess")
	minioSecretKey = getenv("MINIO_SECRET_KEY", "miniosecret")
	metricsPath    = "/minio/v2/metrics/cluster"
	port		   = getenv("EXPORTER_PORT", "8000")
)

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func sanitizeMetricName(name string) string {
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, "/", "_")
	return name
}

func fetchMinioMetrics() (map[string]float64, error) {
	req, err := http.NewRequest("GET", minioURL+metricsPath, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(minioAccessKey, minioSecretKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("received HTTP %d", resp.StatusCode)
	}

	metrics := make(map[string]float64)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := sanitizeMetricName(parts[0])
		value, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			continue
		}
		metrics[name] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return metrics, nil
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	reg := prometheus.NewRegistry()

	metrics, err := fetchMinioMetrics()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching MinIO metrics: %v", err), http.StatusInternalServerError)
		return
	}

	for name, value := range metrics {
		g := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: name,
			Help: "Converted metric from MinIO: " + name,
		})
		g.Set(value)
		reg.MustRegister(g)
	}

	promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

func main() {
	http.HandleFunc("/metrics", metricsHandler)
	log.Printf("Starting MinIO exporter on :%s/metrics", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}