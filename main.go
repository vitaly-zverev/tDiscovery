package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"

	pb "github.com/siderolabs/discovery-api/api/v1alpha1/server/pb"
	"google.golang.org/grpc"
)

type watchSubscriber struct {
	clusterID string
	updates   chan *pb.WatchResponse
}

type server struct {
	pb.UnimplementedClusterServer
	mu              sync.RWMutex
	affiliates      map[string]*AffiliateRecord
	watchers        map[int]*watchSubscriber
	watchID         int
	watchBufferSize int
}

type AffiliateRecord struct {
	AffiliateID string
	Data        []byte
	Endpoints   [][]byte
	ExpiresAt   time.Time
	ClusterID   string
}

func makeKey(clusterID, affiliateID string) string {
	return clusterID + "->" + affiliateID
}

func NewServer(bufferSize int) *server {
	return &server{
		affiliates:      make(map[string]*AffiliateRecord),
		watchers:        make(map[int]*watchSubscriber),
		watchBufferSize: bufferSize,
	}
}

// mergeEndpoints добавляет новые endpoints, избегая дубликатов

func mergeEndpoints(existing, incoming [][]byte) [][]byte {
	seen := make(map[string]struct{})
	result := make([][]byte, 0, len(existing)+len(incoming))

	// добавим уже существующие
	for _, ep := range existing {
		k := string(ep)
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			result = append(result, ep)
		}
	}

	// добавим новые, если их ещё не было
	for _, ep := range incoming {
		k := string(ep)
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			result = append(result, ep)
		}
	}

	return result
}

func (s *server) AffiliateUpdate(ctx context.Context, req *pb.AffiliateUpdateRequest) (*pb.AffiliateUpdateResponse, error) {
	var updated *pb.Affiliate
	var watchers []*watchSubscriber

	s.mu.Lock()
	key := makeKey(req.GetClusterId(), req.GetAffiliateId())
	rec, exists := s.affiliates[key]
	if !exists {
		rec = &AffiliateRecord{
			ClusterID:   req.GetClusterId(),
			AffiliateID: req.GetAffiliateId(),
		}
		s.affiliates[key] = rec
	}

	if len(req.GetAffiliateData()) > 0 {
		rec.Data = req.GetAffiliateData()
	}
	if len(req.GetAffiliateEndpoints()) > 0 {
		//rec.Endpoints = append(rec.Endpoints, req.GetAffiliateEndpoints()...)
		rec.Endpoints = mergeEndpoints(rec.Endpoints, req.GetAffiliateEndpoints())
	}
	if req.Ttl != nil {
		rec.ExpiresAt = time.Now().Add(req.Ttl.AsDuration())
	}

	updated = &pb.Affiliate{
		Id:        rec.AffiliateID,
		Data:      rec.Data,
		Endpoints: rec.Endpoints,
	}

	// Копируем нужных подписчиков
	for _, sub := range s.watchers {
		if sub.clusterID == req.GetClusterId() {
			watchers = append(watchers, sub)
		}
	}

	s.mu.Unlock()

	// Отсылаем уведомление без блокировки
	update := &pb.WatchResponse{
		Affiliates: []*pb.Affiliate{updated},
	}

	for _, sub := range watchers {
		select {
		case sub.updates <- update:
		default:
			// Канал переполнен — пропускаем
		}
	}

	return &pb.AffiliateUpdateResponse{}, nil
}

func (s *server) Hello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloResponse, error) {
	log.Printf("Hello called: cluster_id=%q, client_version=%q", req.GetClusterId(), req.GetClientVersion())

	// Попытка получить IP из заголовка x-real-ip
	var clientIP net.IP
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-real-ip"); len(vals) > 0 {
			ip := net.ParseIP(strings.TrimSpace(vals[0]))
			if ip != nil {
				clientIP = ip
				log.Printf("Client IP from x-real-ip header: %s", clientIP)
			} else {
				log.Printf("Invalid IP in x-real-ip header: %q", vals[0])
			}
		}
	}

	// Если IP не взяли из заголовка, берём из peer
	if clientIP == nil {
		p, ok := peer.FromContext(ctx)
		if !ok {
			log.Println("Failed to get peer from context")
			return &pb.HelloResponse{}, nil
		}

		addr := p.Addr.String()
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			log.Printf("Failed to split host and port from addr %q: %v", addr, err)
			return &pb.HelloResponse{}, nil
		}

		ip := net.ParseIP(host)
		if ip == nil {
			log.Printf("Failed to parse IP from host %q", host)
			return &pb.HelloResponse{}, nil
		}
		clientIP = ip
		log.Printf("Client IP from peer: %s", clientIP)
	}

	return &pb.HelloResponse{
		ClientIp: clientIP,
	}, nil
}

func (s *server) AffiliateDelete(ctx context.Context, req *pb.AffiliateDeleteRequest) (*pb.AffiliateDeleteResponse, error) {
	var watchers []*watchSubscriber
	var deletedID string

	s.mu.Lock()
	key := makeKey(req.GetClusterId(), req.GetAffiliateId())

	if _, exists := s.affiliates[key]; exists {
		delete(s.affiliates, key)
		deletedID = req.GetAffiliateId()

		for _, sub := range s.watchers {
			if sub.clusterID == req.GetClusterId() {
				watchers = append(watchers, sub)
			}
		}
	}
	s.mu.Unlock()

	// Если удаление было — отправим обновление
	if deletedID != "" {
		update := &pb.WatchResponse{
			Affiliates: []*pb.Affiliate{
				{
					Id: deletedID,
				},
			},
			Deleted: true,
		}

		for _, sub := range watchers {
			select {
			case sub.updates <- update:
			default:
				// Пропускаем, если клиент не успевает читать
			}
		}
	}

	return &pb.AffiliateDeleteResponse{}, nil
}

func (s *server) List(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var affiliates []*pb.Affiliate
	clusterID := req.GetClusterId()

	for key, rec := range s.affiliates {
		// Быстрая проверка по префиксу cluster_id| (чтобы не парсить)
		if !strings.HasPrefix(key, clusterID+"->") {
			continue
		}
		if !rec.ExpiresAt.IsZero() && time.Now().After(rec.ExpiresAt) {
			continue
		}

		affiliates = append(affiliates, &pb.Affiliate{
			Id:        rec.AffiliateID,
			Data:      rec.Data,
			Endpoints: rec.Endpoints,
		})
	}

	return &pb.ListResponse{Affiliates: affiliates}, nil
}

func (s *server) Watch(req *pb.WatchRequest, stream pb.Cluster_WatchServer) error {
	clusterID := req.GetClusterId()

	updates := make(chan *pb.WatchResponse, s.watchBufferSize)
	s.mu.Lock()
	id := s.watchID
	s.watchID++
	s.watchers[id] = &watchSubscriber{
		clusterID: clusterID,
		updates:   updates,
	}
	s.mu.Unlock()

	// Отправим начальный снапшот
	s.mu.RLock()
	var snapshot []*pb.Affiliate
	for key, rec := range s.affiliates {
		if !strings.HasPrefix(key, clusterID+"|") {
			continue
		}
		if !rec.ExpiresAt.IsZero() && time.Now().After(rec.ExpiresAt) {
			continue
		}
		snapshot = append(snapshot, &pb.Affiliate{
			Id:        rec.AffiliateID,
			Data:      rec.Data,
			Endpoints: rec.Endpoints,
		})
	}
	s.mu.RUnlock()

	_ = stream.Send(&pb.WatchResponse{Affiliates: snapshot})

	// Начинаем слушать обновления
	for {
		select {
		case <-stream.Context().Done():
			s.mu.Lock()
			delete(s.watchers, id)
			s.mu.Unlock()
			return nil
		case update := <-updates:
			if err := stream.Send(update); err != nil {
				s.mu.Lock()
				delete(s.watchers, id)
				s.mu.Unlock()
				return err
			}
		}
	}
}

func (s *server) StartGC(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			s.pruneExpired()
		}
	}()
}

func uniqueClusterIDs(items []struct {
	clusterID, affiliateID string
}) int {
	seen := make(map[string]struct{})
	for _, item := range items {
		seen[item.clusterID] = struct{}{}
	}
	return len(seen)
}

func (s *server) pruneExpired() {
	now := time.Now()
	var expired []struct {
		clusterID   string
		affiliateID string
	}

	s.mu.Lock()
	for key, rec := range s.affiliates {
		if !rec.ExpiresAt.IsZero() && now.After(rec.ExpiresAt) {
			expired = append(expired, struct {
				clusterID   string
				affiliateID string
			}{
				clusterID:   rec.ClusterID,
				affiliateID: rec.AffiliateID,
			})
			delete(s.affiliates, key)
		}
	}

	// Статистика
	clusterSet := make(map[string]struct{})
	currentEndpoints := 0
	for _, rec := range s.affiliates {
		clusterSet[rec.ClusterID] = struct{}{}
		currentEndpoints += len(rec.Endpoints)
	}

	currentClusters := len(clusterSet)
	currentAffiliates := len(s.affiliates)
	currentSubscriptions := len(s.watchers)

	watchers := make([]*watchSubscriber, 0, currentSubscriptions)
	for _, sub := range s.watchers {
		watchers = append(watchers, sub)
	}
	s.mu.Unlock()

	// Уведомления
	for _, exp := range expired {
		update := &pb.WatchResponse{
			Affiliates: []*pb.Affiliate{
				{Id: exp.affiliateID},
			},
			Deleted: true,
		}
		for _, sub := range watchers {
			if sub.clusterID == exp.clusterID {
				select {
				case sub.updates <- update:
				default:
					// клиент не читает
				}
			}
		}
	}

	// Лог отчёта
	log.Printf("garbage collection run  {\"removed_clusters\": %d, \"removed_affiliates\": %d, \"current_clusters\": %d, \"current_affiliates\": %d, \"current_endpoints\": %d, \"current_subscriptions\": %d}",
		uniqueClusterIDs(expired),
		len(expired),
		currentClusters,
		currentAffiliates,
		currentEndpoints,
		currentSubscriptions,
	)
}

func main() {

	// Параметры командной строки
	var (
		port            = flag.Int("port", 3001, "Port to listen on")
		gcInterval      = flag.Duration("gc-interval", 15*time.Second, "Garbage collection interval (e.g. 10s, 1m)")
		watchBufferSize = flag.Int("watch-buffer-size", 32, "Size of buffered channel for watch updates")
	)
	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	ns := NewServer(*watchBufferSize)
	ns.StartGC(*gcInterval)

	pb.RegisterClusterServer(s, ns)

	log.Printf("gRPC server listening on %s (GC interval: %s, Watch buffer size: %s )", fmt.Sprintf(":%d", *port), gcInterval.String(), fmt.Sprintf("%d", *watchBufferSize))
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
