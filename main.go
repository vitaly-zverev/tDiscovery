package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Node struct {
	ID        string    `json:"id"`
	ClusterID string    `json:"cluster_id"`
	Announce  []string  `json:"announce"`
	TTL       string    `json:"ttl"`
	Expiry    time.Time `json:"expiry"`
	Updated   time.Time `json:"updated"`
}

var (
	nodes = make(map[string]map[string]*Node) // clusterID -> nodeID -> Node
	mu    sync.RWMutex
)

const (
	defaultTTL = 10 * time.Minute
	maxTTL     = 1 * time.Hour
)

func listNodes(w http.ResponseWriter, r *http.Request) {
	clusterID := r.URL.Query().Get("cluster")
	if clusterID == "" {
		http.Error(w, "missing cluster id", http.StatusBadRequest)
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	peers, ok := nodes[clusterID]
	if !ok {
		json.NewEncoder(w).Encode([]*Node{})
		return
	}

	var result []*Node
	for _, node := range peers {
		if time.Now().Before(node.Expiry) {
			result = append(result, node)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func registerNode(w http.ResponseWriter, r *http.Request) {
	var node Node
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if node.ClusterID == "" || node.ID == "" {
		http.Error(w, "missing cluster_id or id", http.StatusBadRequest)
		return
	}

	if _, err := uuid.Parse(node.ID); err != nil {
		http.Error(w, "invalid node id", http.StatusBadRequest)
		return
	}

	parsedTTL := defaultTTL
	if node.TTL != "" {
		if d, err := time.ParseDuration(node.TTL); err == nil {
			if d <= maxTTL {
				parsedTTL = d
			}
		}
	}

	node.Updated = time.Now()
	node.Expiry = node.Updated.Add(parsedTTL)

	mu.Lock()
	if nodes[node.ClusterID] == nil {
		nodes[node.ClusterID] = make(map[string]*Node)
	}
	nodes[node.ClusterID][node.ID] = &node
	mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/v1/registry", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listNodes(w, r)
		case http.MethodPut:
			registerNode(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Printf("Starting discovery server on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

