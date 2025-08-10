package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	pb "github.com/ZnayMed/znaymed-backend/pb"
	"github.com/ZnayMed/znaymed-backend/services/auth-service/db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"net"
)

var secret = []byte("пока_ничего")

type authServer struct {
	pb.UnimplementedAuthServiceServer
	db *db.Database
}

func (s *authServer) CheckUser(ctx context.Context, req *pb.UserRequest) (*pb.CheckUserResponse, error) {
	hashTgid := hashTGID(req.Tgid)
	exists, err := s.db.UserExists(hashTgid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "db error: %v", err)
	}
	return &pb.CheckUserResponse{Exists: exists}, nil
}

//func (s *authServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
//	if req.Username != "admin" || req.Password != "password" {
//		return nil, grpc.Errorf(401, "invalid credentials")
//	}
//
//	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
//		"username": req.Username,
//		"exp":      time.Now().Add(time.Hour * 24).Unix(),
//	})
//
//	tokenString, err := token.SignedString(secret)
//	if err != nil {
//		return nil, err
//	}
//
//	return &pb.LoginResponse{Token: tokenString}, nil
//}
//
//func (s *authServer) VerifyToken(ctx context.Context, req *pb.VerifyRequest) (*pb.VerifyResponse, error) {
//	_, err := jwt.Parse(req.Token, func(token *jwt.Token) (interface{}, error) {
//		return secret, nil
//	})
//
//	return &pb.VerifyResponse{Valid: err == nil}, nil
//}

func (s *authServer) Register(ctx context.Context, req *pb.SaveUserRequest) (*pb.SaveUserResponse, error) {
	hashName := hashTGID(req.Tgid)

	log.Printf("Пытаемся сохранить: name=%s, hashTgid=%s, birthdate=%s", req.Name, hashName, req.Birthdate)

	err := s.db.SaveUser(req.Name, hashName, req.Birthdate)
	if err != nil {
		log.Println("Ошибка при сохранении:", err)
		return &pb.SaveUserResponse{Success: false, Message: err.Error()}, err
	}
	return &pb.SaveUserResponse{Success: true, Message: "Пользователь успешно добавлен"}, nil
}

func hashTGID(tgid string) string {
	hash := sha256.Sum256([]byte(tgid))
	return hex.EncodeToString(hash[:])
}

func main() {
	database, err := db.NewDatabase()
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
	lis, err := net.Listen("tcp", ":50054")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterAuthServiceServer(grpcServer, &authServer{db: database})

	log.Println("AuthService listening on :50054")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
