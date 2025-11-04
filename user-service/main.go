package main

import (
	"log"
	"net"

	// Import proto package
	pb "user-service/proto/user"
	// Import business logic server
	"user-service/server"

	// gRPC core package
	"google.golang.org/grpc"
	// Reflection untuk debugging/testing (seperti Postman untuk gRPC)
	"google.golang.org/grpc/reflection"
)

func main() {
	// 1. CREATE TCP LISTENER
	// Listen di port 50051 untuk menerima koneksi gRPC
	// Format: ":port" berarti listen di semua network interfaces
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("❌ Failed to listen: %v", err)
	}
	
	log.Println("🎧 Listening on :50051")

	// 2. CREATE gRPC SERVER
	// grpc.NewServer() membuat server dengan default configuration
	// Bisa tambahkan options seperti:
	// - grpc.MaxRecvMsgSize() untuk limit ukuran message
	// - grpc.UnaryInterceptor() untuk middleware/logging
	// - grpc.Creds() untuk TLS/SSL
	grpcServer := grpc.NewServer()
	
	log.Println("🔧 gRPC Server created")

	// 3. CREATE BUSINESS LOGIC SERVER
	// Ini adalah struct kita yang implements gRPC service methods
	userServer := server.NewUserServer()
	
	log.Println("👤 User Server initialized")

	// 4. REGISTER SERVICE
	// Register service implementation ke gRPC server
	// Function ini di-generate otomatis dari proto
	// Connects: proto definition ↔ actual implementation
	pb.RegisterUserServiceServer(grpcServer, userServer)
	
	log.Println("📝 UserService registered")

	// 5. ENABLE REFLECTION (Optional, untuk development)
	// Reflection memungkinkan tools seperti grpcurl untuk:
	// - Discover services yang tersedia
	// - Melihat method definitions
	// - Testing tanpa perlu generate client code
	// CATATAN: Disable di production untuk security
	reflection.Register(grpcServer)
	
	log.Println("🔍 gRPC Reflection enabled")

	// 6. START SERVER
	// Serve() adalah blocking call - program akan wait di sini
	// Menerima dan handle incoming gRPC requests
	log.Println("🚀 User Service running on :50051")
	log.Println("✅ Ready to receive gRPC requests...")
	log.Println("⏳ Press Ctrl+C to stop")
	
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ Failed to serve: %v", err)
	}
}

/*
📚 FLOW DIAGRAM:

Client Request
      ↓
TCP Listener (:50051)
      ↓
gRPC Server (deserialize binary)
      ↓
Route ke Method (berdasarkan service name + method name)
      ↓
UserServer.CreateUser() / GetUser() / ListUsers()
      ↓
Business Logic (validation, database, dll)
      ↓
Return Response
      ↓
gRPC Server (serialize ke binary)
      ↓
Send ke Client

🔧 gRPC Server Options (advanced):

grpc.NewServer(
	grpc.MaxRecvMsgSize(1024 * 1024 * 10), // Max 10MB
	grpc.UnaryInterceptor(loggingInterceptor), // Middleware
	grpc.StreamInterceptor(streamInterceptor), // Middleware untuk streaming
	grpc.Creds(credentials.NewServerTLSFromFile("cert.pem", "key.pem")), // TLS
)

🧪 Testing dengan grpcurl (jika reflection enabled):

# List services
grpcurl -plaintext localhost:50051 list

# List methods
grpcurl -plaintext localhost:50051 list user.UserService

# Call method
grpcurl -plaintext -d '{"name":"Test","email":"test@test.com","age":25}' \
  localhost:50051 user.UserService/CreateUser
*/