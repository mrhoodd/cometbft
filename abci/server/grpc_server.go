package server

import (
	"context"
	"net"

	"google.golang.org/grpc"

	"github.com/cometbft/cometbft/v2/abci/types"
	cmtnet "github.com/cometbft/cometbft/v2/internal/net"
	"github.com/cometbft/cometbft/v2/libs/service"
)

type GRPCServer struct {
	service.BaseService

	proto    string
	addr     string
	listener net.Listener
	server   *grpc.Server

	app types.Application
}

// NewGRPCServer returns a new gRPC ABCI server.
func NewGRPCServer(protoAddr string, app types.Application) service.Service {
	proto, addr := cmtnet.ProtocolAndAddress(protoAddr)
	s := &GRPCServer{
		proto:    proto,
		addr:     addr,
		listener: nil,
		app:      app,
	}
	s.BaseService = *service.NewBaseService(nil, "ABCIServer", s)
	return s
}

// OnStart starts the gRPC service.
func (s *GRPCServer) OnStart() error {
	ln, err := net.Listen(s.proto, s.addr)
	if err != nil {
		return err
	}

	s.listener = ln
	s.server = grpc.NewServer(grpc.MaxConcurrentStreams(100)) // Limit to 100 streams per connection
	types.RegisterABCIServer(s.server, &gRPCApplication{s.app})

	s.Logger.Info("Listening", "proto", s.proto, "addr", s.addr)
	go func() {
		if err := s.server.Serve(s.listener); err != nil {
			s.Logger.Error("Error serving gRPC server", "err", err)
		}
	}()
	return nil
}

// OnStop stops the gRPC server.
func (s *GRPCServer) OnStop() {
	s.server.Stop()
}

// -------------------------------------------------------

// gRPCApplication is a gRPC shim for Application.
type gRPCApplication struct {
	types.Application
}

func (*gRPCApplication) Echo(_ context.Context, req *types.EchoRequest) (*types.EchoResponse, error) {
	return &types.EchoResponse{Message: req.Message}, nil
}

func (*gRPCApplication) Flush(context.Context, *types.FlushRequest) (*types.FlushResponse, error) {
	return &types.FlushResponse{}, nil
}
