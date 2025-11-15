package main

import (
	"io"
	"log"
	"net/http"
	"os"
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

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func makeHandler(minioPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[Polling] update metrics from %s", minioPath)

		client := &http.Client{}
		req, err := http.NewRequest("GET", minioBaseURL+minioPath, nil)
		if err != nil {
			http.Error(w, "Error creating request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "Error fetching metrics: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			http.Error(w, "MinIO returned status: "+resp.Status, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			log.Println("Error copying response:", err)
		}
	}
}

func main() {
	port := getenv("EXPORTER_PORT", "8080")

	log.Printf("[Registering MinIo] BaseUrl:", minioBaseURL)

	for _, endpoint := range minioV2Endpoints {
		http.HandleFunc("/metrics/cluter", makeHandler(endpoint.Url))
		log.Printf("[Registering Handler] service:", endpoint.Name, "endpoint:", endpoint.Url)
	}

	log.Printf("Starting MinIO pass-through exporter on :%s/metrics", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
