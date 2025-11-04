package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	// Import proto yang sudah di-generate
	// pb = protocol buffer (naming convention umum)
	pb "user-service/proto/user"

	"github.com/google/uuid"
)

// UserServer adalah struct yang mengimplementasikan gRPC service
// Struct ini harus meng-embed UnimplementedUserServiceServer untuk forward compatibility
// Artinya: jika di masa depan ada method baru di proto, code ini tidak akan break
type UserServer struct {
	pb.UnimplementedUserServiceServer // Embedded untuk safety
	users map[string]*pb.User          // In-memory storage (dalam produksi pakai database)
	mu    sync.RWMutex                 // Mutex untuk thread-safety (concurrent access)
}

// NewUserServer adalah constructor function untuk membuat instance UserServer
// Pattern ini umum digunakan di Go untuk inisialisasi struct
func NewUserServer() *UserServer {
	return &UserServer{
		users: make(map[string]*pb.User), // Initialize map
	}
}

// CreateUser mengimplementasikan RPC method CreateUser dari proto
// Signature method ini HARUS sesuai dengan yang di-generate dari proto:
// - Parameter 1: context.Context (untuk timeout, cancellation, metadata)
// - Parameter 2: Request message (*pb.CreateUserRequest)
// - Return 1: Response message (*pb.CreateUserResponse)
// - Return 2: error
func (s *UserServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	log.Printf("📝 Creating user: %s", req.Name)

	// Lock untuk write operation (thread-safe)
	// Penting jika ada multiple concurrent requests
	s.mu.Lock()
	defer s.mu.Unlock() // Unlock otomatis saat function selesai

	// Validasi input
	// Best practice: selalu validasi data dari client
	if req.Name == "" || req.Email == "" {
		// Return response dengan success=false DAN error
		// Client bisa cek dari status code atau dari response.Success
		return &pb.CreateUserResponse{
			Success: false,
			Message: "Name and email are required",
		}, fmt.Errorf("validation error")
	}

	// Buat user baru
	// Perhatikan: kita membuat struct sesuai dengan message User di proto
	user := &pb.User{
		Id:        uuid.New().String(),           // Generate unique ID
		Name:      req.Name,                      // Ambil dari request
		Email:     req.Email,                     // Ambil dari request
		Age:       req.Age,                       // Ambil dari request
		CreatedAt: time.Now().Format(time.RFC3339), // Timestamp
	}

	// Simpan ke "database" (map)
	s.users[user.Id] = user

	// Return response yang sukses
	// Response ini akan di-serialize menjadi binary oleh gRPC
	return &pb.CreateUserResponse{
		User:    user,                         // Embedded user object
		Message: "User created successfully",  // Status message
		Success: true,                         // Flag sukses
	}, nil // nil error = sukses
}

// GetUser mengimplementasikan RPC method GetUser (Unary RPC)
// Unary = simple request-response (seperti HTTP request biasa)
func (s *UserServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	log.Printf("🔍 Getting user: %s", req.Id)

	// RLock untuk read operation (multiple readers bisa akses bersamaan)
	// Lebih efisien daripada Lock() untuk read-only operation
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Cari user di map
	user, exists := s.users[req.Id]
	if !exists {
		// Return nil response DAN error
		// gRPC akan convert error ini menjadi status code
		return nil, fmt.Errorf("user with id %s not found", req.Id)
	}

	// Return response dengan user yang ditemukan
	return &pb.GetUserResponse{
		User: user,
	}, nil
}

// ListUsers mengimplementasikan RPC method ListUsers (Server Streaming RPC)
// Server Streaming = server mengirim multiple messages ke client
// Signature berbeda: parameter ke-2 adalah stream object, bukan request biasa
// stream = channel untuk mengirim data bertahap
func (s *UserServer) ListUsers(req *pb.ListUsersRequest, stream pb.UserService_ListUsersServer) error {
	log.Printf("📋 Listing users with limit: %d", req.Limit)

	// Lock untuk read operation
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := int32(0)
	
	// Iterate semua users
	for _, user := range s.users {
		// Cek limit (jika 0, berarti unlimited)
		if req.Limit > 0 && count >= req.Limit {
			break // Stop jika sudah mencapai limit
		}

		// Send user satu per satu melalui stream
		// stream.Send() adalah blocking call sampai data terkirim
		if err := stream.Send(&pb.UserResponse{User: user}); err != nil {
			return err // Return error jika gagal send
		}
		count++

		// Simulasi delay untuk demo streaming
		// Dalam produksi, ini biasanya dari database query yang pelan
		// time.Sleep(100 * time.Millisecond)
	}

	log.Printf("✅ Sent %d users", count)
	
	// Return nil = stream selesai dengan sukses
	// Client akan menerima EOF (End of File) signal
	return nil
}

/*
📚 CATATAN PENTING tentang RPC Types:

1. Unary RPC (CreateUser, GetUser):
   - Client send 1 request → Server send 1 response
   - Seperti HTTP request biasa
   
2. Server Streaming RPC (ListUsers):
   - Client send 1 request → Server send MULTIPLE responses
   - Berguna untuk: list data besar, real-time updates, progress tracking
   
3. Client Streaming RPC (tidak ada di contoh ini):
   - Client send MULTIPLE requests → Server send 1 response
   - Berguna untuk: upload file besar, batch insert
   
4. Bidirectional Streaming RPC (tidak ada di contoh ini):
   - Client dan Server send MULTIPLE messages bolak-balik
   - Berguna untuk: chat, real-time collaboration

🔐 Thread Safety:
- sync.RWMutex digunakan karena map di Go TIDAK thread-safe
- Lock() untuk write (Create)
- RLock() untuk read (Get, List)
- Dalam produksi dengan database, biasanya tidak perlu mutex manual

🎯 Error Handling:
- Return error untuk invalid input atau server error
- gRPC akan convert ke status codes (seperti HTTP 404, 500, dll)
- Client bisa handle error dengan checking status code
*/