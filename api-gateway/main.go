package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	// Import proto (sama seperti di server)
	pb "api-gateway/proto/user"

	// gRPC client packages
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// APIGateway struct menyimpan gRPC client connections
// Pattern ini memungkinkan kita connect ke multiple microservices
type APIGateway struct {
	userClient pb.UserServiceClient // gRPC client untuk User Service
	// orderClient pb.OrderServiceClient // Contoh: service lain
	// productClient pb.ProductServiceClient // Contoh: service lain
}

// NewAPIGateway adalah constructor yang membuat koneksi ke gRPC services
// Parameter: address dari masing-masing service
func NewAPIGateway(userServiceAddr string) (*APIGateway, error) {
	log.Println("🔌 Connecting to User Service at", userServiceAddr)

	// CREATE gRPC CLIENT CONNECTION
	// grpc.NewClient() membuat connection (lazy connection)
	// Actual connection dibuat saat first RPC call
	conn, err := grpc.NewClient(
		userServiceAddr, // Address service: "localhost:50051"
		
		// WithTransportCredentials: cara authentication/encryption
		// insecure.NewCredentials() = tanpa TLS (hanya untuk development!)
		// Production: pakai credentials.NewClientTLSFromFile()
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		
		// Options lain (opsional):
		// grpc.WithBlock() - tunggu sampai connected (synchronous)
		// grpc.WithTimeout() - timeout untuk connection
		// grpc.WithDefaultCallOptions() - default options untuk semua RPC calls
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to user service: %v", err)
	}

	log.Println("✅ Connected to User Service")

	// CREATE CLIENT STUB
	// NewUserServiceClient() di-generate dari proto
	// Stub ini berisi semua method yang bisa dipanggil
	client := pb.NewUserServiceClient(conn)

	return &APIGateway{
		userClient: client,
	}, nil
}

// CreateUserHandler adalah HTTP handler yang mengkonversi HTTP request ke gRPC call
// Pattern: HTTP Gateway → gRPC Client → gRPC Server
func (gw *APIGateway) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	// 1. VALIDASI HTTP METHOD
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2. PARSE HTTP REQUEST BODY (JSON)
	// Struct anonymous untuk input dari client
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int32  `json:"age"`
	}

	// Decode JSON dari request body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("📥 Received CreateUser request: %s (%s)", req.Name, req.Email)

	// 3. CREATE CONTEXT dengan TIMEOUT
	// Context penting untuk:
	// - Timeout: batalkan request jika terlalu lama
	// - Cancellation: user cancel request
	// - Deadline: hard deadline untuk request
	// - Metadata: kirim extra info (auth token, trace ID, dll)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // Cleanup context

	// 4. CALL gRPC METHOD
	// userClient.CreateUser() adalah blocking call
	// Request: HTTP JSON → Protobuf binary
	// Response: Protobuf binary → Go struct
	resp, err := gw.userClient.CreateUser(ctx, &pb.CreateUserRequest{
		Name:  req.Name,
		Email: req.Email,
		Age:   req.Age,
	})

	// 5. ERROR HANDLING
	if err != nil {
		log.Printf("❌ gRPC call failed: %v", err)
		
		// Bisa check specific gRPC status codes:
		// status.Code(err) == codes.NotFound
		// status.Code(err) == codes.InvalidArgument
		// status.Code(err) == codes.DeadlineExceeded
		
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ User created: %s", resp.User.Id)

	// 6. RETURN HTTP RESPONSE (JSON)
	// Convert protobuf response → JSON untuk HTTP client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetUserHandler menghandle GET request untuk ambil user by ID
// Pattern sama: HTTP → gRPC → HTTP
func (gw *APIGateway) GetUserHandler(w http.ResponseWriter, r *http.Request) {
	// 1. VALIDASI METHOD
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2. PARSE QUERY PARAMETER
	// URL: /users/get?id=123
	userId := r.URL.Query().Get("id")
	if userId == "" {
		http.Error(w, "id parameter required", http.StatusBadRequest)
		return
	}

	log.Printf("📥 Received GetUser request: %s", userId)

	// 3. CONTEXT dengan TIMEOUT
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 4. CALL gRPC METHOD (Unary RPC)
	resp, err := gw.userClient.GetUser(ctx, &pb.GetUserRequest{
		Id: userId,
	})

	// 5. ERROR HANDLING
	if err != nil {
		log.Printf("❌ gRPC call failed: %v", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	log.Printf("✅ User found: %s", resp.User.Name)

	// 6. RETURN RESPONSE
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ListUsersHandler menghandle streaming response dari gRPC
// Ini contoh bagaimana handle Server Streaming RPC
func (gw *APIGateway) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	// 1. VALIDASI METHOD
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("📥 Received ListUsers request")

	// 2. CONTEXT dengan TIMEOUT (lebih lama untuk streaming)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 3. CALL gRPC STREAMING METHOD
	// Ini return stream object, bukan response langsung
	stream, err := gw.userClient.ListUsers(ctx, &pb.ListUsersRequest{
		Limit: 10,
	})

	if err != nil {
		log.Printf("❌ gRPC call failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. RECEIVE STREAMING DATA
	var users []*pb.User
	
	// Loop untuk receive semua messages dari stream
	for {
		// stream.Recv() adalah blocking call
		// Akan wait sampai message baru datang atau stream selesai
		resp, err := stream.Recv()
		
		// EOF = End of File = stream selesai (sukses)
		if err == io.EOF {
			log.Println("✅ Stream finished")
			break // Keluar dari loop
		}
		
		// Error lain = ada masalah
		if err != nil {
			log.Printf("❌ Stream error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		// Append user ke slice
		users = append(users, resp.User)
		log.Printf("📦 Received user: %s", resp.User.Name)
	}

	log.Printf("✅ Total users received: %d", len(users))

	// 5. RETURN AGGREGATED RESPONSE
	// Convert semua streaming data menjadi 1 HTTP response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
		"count": len(users),
	})
}

func main() {
	log.Println("🚀 Starting API Gateway...")

	// 1. CONNECT TO gRPC SERVICES
	// Ini biasanya dari environment variable atau config file
	gateway, err := NewAPIGateway("localhost:50051")
	if err != nil {
		log.Fatalf("❌ Failed to create gateway: %v", err)
	}

	log.Println("✅ All gRPC connections established")

	// 2. SETUP HTTP ROUTES
	// Map HTTP endpoints ke handler functions
	http.HandleFunc("/users/create", gateway.CreateUserHandler)
	http.HandleFunc("/users/get", gateway.GetUserHandler)
	http.HandleFunc("/users/list", gateway.ListUsersHandler)

	// Health check endpoint (untuk load balancer/monitoring)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 3. PRINT ROUTES INFO
	log.Println("🌐 API Gateway running on :8080")
	log.Println("📍 Endpoints:")
	log.Println("   POST   http://localhost:8080/users/create")
	log.Println("   GET    http://localhost:8080/users/get?id=xxx")
	log.Println("   GET    http://localhost:8080/users/list")
	log.Println("   GET    http://localhost:8080/health")
	log.Println("⏳ Press Ctrl+C to stop")

	// 4. START HTTP SERVER
	// ListenAndServe adalah blocking call
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}

/*
📚 ARCHITECTURE PATTERN:

┌─────────────┐         ┌─────────────────┐         ┌──────────────┐
│  HTTP       │         │  API Gateway    │         │  User        │
│  Client     │────────▶│  (Port 8080)    │────────▶│  Service     │
│  (Browser)  │  JSON   │                 │  gRPC   │  (Port 50051)│
└─────────────┘         └─────────────────┘         └──────────────┘

Keuntungan Pattern ini:
✅ Client tidak perlu tahu tentang gRPC (pakai HTTP/JSON biasa)
✅ Gateway bisa aggregate multiple services
✅ Gateway bisa handle auth, rate limiting, logging
✅ Internal services pakai gRPC (lebih efisien)

🔐 Production Improvements:

1. Use TLS:
   grpc.WithTransportCredentials(
       credentials.NewClientTLSFromFile("ca.crt", "")
   )

2. Add Timeout per request:
   ctx, cancel := context.WithTimeout(ctx, 2*time.Second)

3. Add Retry logic:
   grpc.WithDefaultCallOptions(
       grpc.WaitForReady(true),
       grpc.MaxCallRecvMsgSize(10*1024*1024),
   )

4. Add Middleware:
   - Authentication
   - Request logging
   - Metrics (Prometheus)
   - Tracing (Jaeger/Zipkin)

5. Connection pooling:
   - Reuse gRPC connection (sudah built-in)
   - jangan create conn untuk setiap request

🎯 Context Usage:

Context digunakan untuk:
1. Timeout - cancel jika terlalu lama
2. Cancellation - user cancel request
3. Metadata - kirim auth token, trace ID, dll

Contoh:
ctx := context.Background()
ctx = context.WithValue(ctx, "user-id", "123")
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
*/