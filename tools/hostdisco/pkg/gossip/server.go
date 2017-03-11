package gossip

import (
	"net"
	"google.golang.org/grpc"
	"fmt"
	pb "k8s.io/kops/tools/hostdisco/pkg/proto"
)

type Server struct {
	listenAddress string
}
func NewServer(listenAddress string) (*Server) {
	return &Server{
		listenAddress: listenAddress,
	}
}

func (s*Server) Run() error {
	lis, err := net.Listen("tcp", s.listenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on %q: %v", s.listenAddress, err)
	}
	s := grpc.NewServer()
	pb.RegisterMyServiceServer(s, &server{})
	s.Serve(lis)
}