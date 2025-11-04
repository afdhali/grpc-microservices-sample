package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	pb "api-gateway/proto/user"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type APIGateway struct {
	userClient pb.UserServiceClient
}

func NewAPIGateway(userServiceAddr string) (*APIGateway, error) {
	// ✅ Menggunakan grpc.NewClient (bukan grpc.Dial yang deprecated)
	conn, err := grpc.NewClient(
		userServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to user service: %v", err)
	}

	client := pb.NewUserServiceClient(conn)

	return &APIGateway{
		userClient: client,
	}, nil
}

func (gw *APIGateway) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int32  `json:"age"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := gw.userClient.CreateUser(ctx, &pb.CreateUserRequest{
		Name:  req.Name,
		Email: req.Email,
		Age:   req.Age,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (gw *APIGateway) GetUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userId := r.URL.Query().Get("id")
	if userId == "" {
		http.Error(w, "id parameter required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := gw.userClient.GetUser(ctx, &pb.GetUserRequest{
		Id: userId,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (gw *APIGateway) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := gw.userClient.ListUsers(ctx, &pb.ListUsersRequest{
		Limit: 10,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var users []*pb.User
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		users = append(users, resp.User)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
		"count": len(users),
	})
}

func main() {
	gateway, err := NewAPIGateway("localhost:50051")
	if err != nil {
		log.Fatalf("❌ Failed to create gateway: %v", err)
	}

	http.HandleFunc("/users/create", gateway.CreateUserHandler)
	http.HandleFunc("/users/get", gateway.GetUserHandler)
	http.HandleFunc("/users/list", gateway.ListUsersHandler)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Println("🌐 API Gateway running on :5050")
	log.Println("📍 Endpoints:")
	log.Println("   POST   http://localhost:5050/users/create")
	log.Println("   GET    http://localhost:5050/users/get?id=xxx")
	log.Println("   GET    http://localhost:5050/users/list")
	log.Println("   GET    http://localhost:5050/health")

	if err := http.ListenAndServe(":5050", nil); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}