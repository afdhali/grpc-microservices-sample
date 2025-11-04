package main

import (
	"log"
	"net"

	pb "user-service/proto/user"
	"user-service/server"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("❌ Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	userServer := server.NewUserServer()
	pb.RegisterUserServiceServer(grpcServer, userServer)

	reflection.Register(grpcServer)

	log.Println("🚀 User Service running on :50051")
	log.Println("✅ Ready to receive gRPC requests...")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ Failed to serve: %v", err)
	}
}