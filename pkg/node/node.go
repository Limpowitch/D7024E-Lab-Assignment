package node

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type Info struct {
	NodeID   string `json:"node_id"`
	Hostname string `json:"hostname"`
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func RunNode() {
	port := getenv("PORT", "8080")
	nodeID := getenv("NODE_ID", "")
	hostname, _ := os.Hostname()
	if nodeID == "" {
		nodeID = hostname
	}

	// http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
	// 	w.WriteHeader(http.StatusOK)
	// 	_, _ = w.Write([]byte("ok"))
	// })

	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Info{NodeID: nodeID, Hostname: hostname})
	})

	log.Printf("listening on :%s (node_id=%s host=%s)", port, nodeID, hostname)
	log.Fatal(http.ListenAndServe(":"+port, nil)) // binds 0.0.0.0
}
