package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"google.golang.org/protobuf/types/known/durationpb"

	// Import generated protobuf from buf
	pb "gen/go/siderolabs/discovery/api/v1alpha1/server"
)

type Peer struct {
	ID       string            `json:"id"`
	Announce []string          `json:"announce"`
	Roles    []string          `json:"roles,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	TTL      string            `json:"ttl"`
}

type ClusterRequest struct {
	ClusterID string `json:"cluster_id"`
	Peer
}

type ClusterResponse struct {
	Peers []Peer `json:"peers"`
}

var (
	nodes = make(map[string]map[string]*storedPeer) // clusterID -> nodeID -> storedPeer
	mu    sync.RWMutex
)

const (
	defaultTTL = 10 * time.Minute
	maxTTL     = 1 * time.Hour
)

type storedPeer struct {
	Peer
	Expiry  time.Time
	Updated time.Time
}

func listPeers(w http.ResponseWriter, r *http.Request) {
	clusterID := r.URL.Query().Get("cluster_id")
	if clusterID == "" {
		http.Error(w, "missing cluster_id", http.StatusBadRequest)
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	peers, ok := nodes[clusterID]
	if !ok {
		json.NewEncoder(w).Encode(ClusterResponse{})
		return
	}

	var result []Peer
	for _, node := range peers {
		if time.Now().Before(node.Expiry) {
			result = append(result, node.Peer)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ClusterResponse{Peers: result})
}

func registerPeer(w http.ResponseWriter, r *http.Request) {
	var req ClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.ClusterID == "" || req.ID == "" {
		http.Error(w, "missing cluster_id or id", http.StatusBadRequest)
		return
	}

	if _, err := uuid.Parse(req.ID); err != nil {
		http.Error(w, "invalid peer id", http.StatusBadRequest)
		return
	}

	ttl := defaultTTL
	if req.TTL != "" {
		if d, err := time.ParseDuration(req.TTL); err == nil && d <= maxTTL {
			ttl = d
		}
	}

	peer := &storedPeer{
		Peer:    req.Peer,
		Updated: time.Now(),
		Expiry:  time.Now().Add(ttl),
	}

	mu.Lock()
	if nodes[req.ClusterID] == nil {
		nodes[req.ClusterID] = make(map[string]*storedPeer)
	}
	nodes[req.ClusterID][req.ID] = peer
	mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

// gRPC Server implementation

type grpcServer struct {
	pb.UnimplementedClusterDiscoveryServer
}

func (s *grpcServer) Peers(ctx context.Context, req *pb.PeersRequest) (*pb.PeersResponse, error) {
	mu.RLock()
	defer mu.RUnlock()

	clusterID := req.GetClusterId()
	peerMap, ok := nodes[clusterID]
	if !ok {
		return &pb.PeersResponse{}, nil
	}

	var out []*pb.Peer
	for _, node := range peerMap {
		if time.Now().Before(node.Expiry) {
			out = append(out, &pb.Peer{
				Id:       node.ID,
				Announce: node.Announce,
				Roles:    node.Roles,
				Metadata: node.Metadata,
				Ttl:      durationpb.New(time.Until(node.Expiry)),
			})
		}
	}

	return &pb.PeersResponse{Peers: out}, nil
}

func (s *grpcServer) Register(stream pb.ClusterDiscovery_RegisterServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		peer := req.GetPeer()
		if peer == nil || peer.Id == "" || req.GetClusterId() == "" {
			continue
		}

		ttl := defaultTTL
		if peer.Ttl != nil {
			parsed := peer.Ttl.AsDuration()
			if parsed <= maxTTL {
				ttl = parsed
			}
		}

		mu.Lock()
		if nodes[req.ClusterId] == nil {
			nodes[req.ClusterId] = make(map[string]*storedPeer)
		}
		nodes[req.ClusterId][peer.Id] = &storedPeer{
			Peer: Peer{
				ID:       peer.Id,
				Announce: peer.Announce,
				Roles:    peer.Roles,
				Metadata: peer.Metadata,
				TTL:      ttl.String(),
			},
			Updated: time.Now(),
			Expiry:  time.Now().Add(ttl),
		}
		mu.Unlock()

		_ = stream.Send(&pb.RegisterResponse{})
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50051"
	}

	http.HandleFunc("/v1/registry", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listPeers(w, r)
		case http.MethodPut:
			registerPeer(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	go func() {
		log.Printf("Starting HTTP discovery server on :%s", port)
		log.Fatal(http.ListenAndServe(":"+port, nil))
	}()

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on gRPC port: %v", err)
	}
	grpcSrv := grpc.NewServer()
	pb.RegisterClusterDiscoveryServer(grpcSrv, &grpcServer{})
	log.Printf("Starting gRPC discovery server on :%s", grpcPort)
	log.Fatal(grpcSrv.Serve(lis))
}

