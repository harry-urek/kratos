package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/harry-urek/urek/v/internal/logger"
	"github.com/harry-urek/urek/v/internal/models"
	"github.com/harry-urek/urek/v/internal/session"
	"github.com/harry-urek/urek/v/proto/sessionpb"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Server represents the gRPC server for session management
type Server struct {
	sessionpb.UnimplementedSessionServiceServer
	grpcServer *grpc.Server
	manager    *session.Manager
	log        *zap.Logger
}

// NewServer creates a new gRPC server
func NewServer(manager *session.Manager) *Server {
	s := &Server{
		manager: manager,
		log:     logger.Named("grpc-server"),
	}

	// Create gRPC server with interceptors
	s.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(s.loggingInterceptor),
	)
	sessionpb.RegisterSessionServiceServer(s.grpcServer, s)

	// Enable reflection for tools like grpcurl
	reflection.Register(s.grpcServer)

	return s
}

// Start starts the gRPC server
func (s *Server) Start(address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		s.log.Error("Failed to listen", zap.Error(err), zap.String("address", address))
		return err
	}

	s.log.Info("gRPC server starting", zap.String("address", address))
	return s.grpcServer.Serve(lis)
}

// Stop stops the gRPC server
func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
	s.log.Info("gRPC server stopped")
}

// ValidateSession implements the ValidateSession RPC method
func (s *Server) ValidateSession(ctx context.Context, req *sessionpb.SessionRequest) (*sessionpb.SessionResponse, error) {
	start := time.Now()
	s.log.Debug("ValidateSession request received", zap.String("session_id", req.SessionId))

	session, err := s.manager.ValidateSession(ctx, req.SessionId)

	response := &sessionpb.SessionResponse{
		Valid: false,
	}

	if err != nil {
		s.log.Debug("Session validation failed",
			zap.Error(err),
			zap.String("session_id", req.SessionId),
			zap.Duration("elapsed", time.Since(start)))
		return response, nil
	}

	// Convert claims map
	claims := make(map[string]string)
	for k, v := range session.Claims {
		claims[k] = v
	}

	response.Valid = true
	response.SessionId = session.ID
	response.Claims = claims

	s.log.Debug("Session validated successfully",
		zap.String("session_id", req.SessionId),
		zap.Duration("elapsed", time.Since(start)))

	return response, nil
}

// CreateSession implements the CreateSession RPC method
func (s *Server) CreateSession(ctx context.Context, req *sessionpb.CreateSessionRequest) (*sessionpb.SessionResponse, error) {
	start := time.Now()
	s.log.Debug("CreateSession request received", zap.String("client_id", req.ClientId))

	// Convert request to internal model
	sessionReq := &models.CreateSessionRequest{
		UserID:   req.ClientId, // In this simplified version, we use client_id as user_id
		ClientID: req.ClientId,
		Claims:   req.Claims,
	}

	// Handle expiry if provided
	if req.Expiry > 0 {
		sessionReq.Duration = time.Duration(req.Expiry) * time.Second
	}

	session, err := s.manager.CreateSession(ctx, sessionReq)
	if err != nil {
		s.log.Error("Failed to create session",
			zap.Error(err),
			zap.String("client_id", req.ClientId),
			zap.Duration("elapsed", time.Since(start)))
		return &sessionpb.SessionResponse{Valid: false}, fmt.Errorf("failed to create session: %w", err)
	}

	// Convert claims map
	claims := make(map[string]string)
	for k, v := range session.Claims {
		claims[k] = v
	}

	response := &sessionpb.SessionResponse{
		Valid:     true,
		SessionId: session.ID,
		Claims:    claims,
	}

	s.log.Debug("Session created successfully",
		zap.String("session_id", session.ID),
		zap.String("client_id", req.ClientId),
		zap.Duration("elapsed", time.Since(start)))

	return response, nil
}

// loggingInterceptor is a gRPC interceptor that logs requests
func (s *Server) loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()

	// Execute the handler
	resp, err := handler(ctx, req)

	// Log the request
	logger.LogGRPCRequest(info.FullMethod, time.Since(start), err)

	return resp, err
}
