package main

import (
  "os"
	"log"
  "fmt"
  "net"

  "faild-agent/faild"

  "google.golang.org/grpc"
  
)

func main() {

  if len(os.Args) == 1 {
    log.Fatalf("Usage: %s iface", os.Args[0])
  }

	fmt.Println("Starting Faild agent")

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 9000))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := faild.Server{
    Iface: os.Args[1],
  }

	grpcServer := grpc.NewServer()

	faild.RegisterFaildServiceServer(grpcServer, &s)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}