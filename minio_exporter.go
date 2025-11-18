package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type MinioEndpoint struct {
	Name string
	Url  string
}

var (
	minioBaseURL = getenv("MINIO_URL", "http://localhost:9000")
	// minioAccessKey = getenv("MINIO_ACCESS_KEY", "minioaccess")
	// minioSecretKey = getenv("MINIO_SECRET_KEY", "miniosecret")
	minioV2Endpoints = []MinioEndpoint{
		{Name: "cluster", Url: minioBaseURL + "/minio/v2/metrics/cluster"},
		{Name: "bucket", Url: minioBaseURL + "/minio/v2/metrics/bucket"},
		{Name: "resource", Url: minioBaseURL + "/minio/v2/metrics/resource"},
		{Name: "node", Url: minioBaseURL + "/minio/v2/metrics/node"},
	}
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func makeHandler(minioPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("[Polling] GET %s", minioPath)

		resp, err := httpClient.Get(minioPath)
		if err != nil {
			http.Error(w, "[Fetching] Error fetching metrics: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			http.Error(w, "[Fetching] MinIO returned "+resp.Status+" - "+string(body), http.StatusBadGateway)
			return
		}

		// Forward headers from MinIO
		for k, values := range resp.Header {
			for _, v := range values {
				w.Header().Add(k, v)
			}
		}

		// overwrite content-type with Prometheus standard
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		_, err = io.Copy(w, resp.Body)
		if err != nil {
			log.Println("[Responsing] Error copying response:", err)
		}

		log.Printf("[Done] %s (duration: %v)", minioPath, time.Since(start))
	}
}

func main() {
	port := getenv("EXPORTER_PORT", "8080")

	log.Println("[Registering MinIO] BaseUrl:", minioBaseURL)

	for _, endpoint := range minioV2Endpoints {
		log.Println("[Registering Handler] service:", endpoint.Name, "endpoint:", endpoint.Url)
		http.HandleFunc("/metrics/"+endpoint.Name, makeHandler(endpoint.Url))
	}

	log.Printf("Starting MinIO pass-through exporter on :%s/metrics", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
