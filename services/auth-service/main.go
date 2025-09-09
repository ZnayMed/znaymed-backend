package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	pb "github.com/ZnayMed/znaymed-backend/pb"
	"github.com/ZnayMed/znaymed-backend/services/auth-service/db"
	redisauth "github.com/ZnayMed/znaymed-backend/services/auth-service/redis"
	goredis "github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"log"
	"net"
	"time"
)

const userTTL = 3 * time.Hour

type authServer struct {
	pb.UnimplementedAuthServiceServer
	db  *db.Database
	rdb *goredis.Client
}

func (s *authServer) GetUser(ctx context.Context, req *pb.UserRequest) (*pb.UserInfo, error) {
	hashTgid := hashTGID(req.Tgid)
	u, err := s.db.GetUserByTGIDHash(hashTgid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.UserInfo{}, nil
		}
		return nil, status.Errorf(codes.Internal, "db error: %v", err)
	}
	return &pb.UserInfo{
		Name:    u.Name,
		Tgid:    req.Tgid, // возвращаем исходный, не хэш
		Email:   u.Email,
		IsAdmin: u.IsAdmin,
	}, nil
}

func (s *authServer) CheckUser(ctx context.Context, req *pb.UserRequest) (*pb.CheckUserResponse, error) {
	tgid := req.Tgid
	hashTgid := hashTGID(tgid)

	log.Printf("CheckUser: tgid=%s, hash=%s", tgid, hashTgid)

	if s.rdb != nil {
		existsKey := "user:" + tgid + ":exists"
		sectionsKey := "user:" + tgid + ":sections"

		log.Printf("Проверка Redis: ключи %q и %q", existsKey, sectionsKey)

		if n, err := s.rdb.Exists(ctx, existsKey, sectionsKey).Result(); err == nil {
			if n > 0 {
				log.Printf("Redis hit: найдено %d ключ(ей), возвращаем Exists=true", n)
				return &pb.CheckUserResponse{Exists: true}, nil
			}
			log.Printf("ℹRedis miss: ключи не найдены")
		} else {
			log.Printf("⚠Redis error: %v", err)
		}
	}

	log.Printf("Проверка в БД...")
	exists, err := s.db.UserExists(hashTgid)
	if err != nil {
		log.Printf("DB error: %v", err)
		return nil, status.Errorf(codes.Internal, "db error: %v", err)
	}
	if !exists {
		log.Printf("Пользователь не найден в БД")
		return &pb.CheckUserResponse{Exists: false}, nil
	}
	log.Printf("DB hit: пользователь найден")

	if s.rdb != nil {
		log.Printf("Сохраняем маркер существования в Redis на TTL=%s", userTTL)
		_ = redisauth.SetUserExistMarker(ctx, s.rdb, tgid, userTTL)

		if titles, err := s.db.GetAccessibleSectionTitlesByTGIDHash(hashTgid); err == nil {
			log.Printf("Сохраняем %d секций пользователя в Redis", len(titles))
			_ = redisauth.SaveUserSectionsByTitles(ctx, s.rdb, tgid, titles, userTTL)
		} else {
			log.Printf("⚠Ошибка получения секций из БД: %v", err)
		}
	}

	log.Printf("Возвращаем Exists=true")
	return &pb.CheckUserResponse{Exists: true}, nil
}

func (s *authServer) IsAdmin(ctx context.Context, req *pb.UserRequest) (*pb.IsAdminResponse, error) {
	tgid := req.Tgid
	hashTgid := hashTGID(tgid)

	// Быстро пробуем отдать из Redis (опционально — если хочешь кэшировать):
	// key := "user:" + tgid + ":admin"
	// if s.rdb != nil {
	//     if val, err := s.rdb.Get(ctx, key).Result(); err == nil && (val == "1" || val == "true") {
	//         return &pb.IsAdminResponse{IsAdmin: true}, nil
	//     }
	// }

	isAdmin, err := s.db.IsAdminByTGIDHash(hashTgid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "db error: %v", err)
	}

	// Кэшируем (опционально):
	// if s.rdb != nil {
	//     v := "0"
	//     if isAdmin { v = "1" }
	//     _ = s.rdb.SetEx(ctx, key, v, userTTL).Err()
	// }

	return &pb.IsAdminResponse{IsAdmin: isAdmin}, nil
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
	log.Printf("Пытаемся сохранить: name=%s, hashTgid=%s, email=%s", req.Name, hashName, req.Email)

	if err := s.db.SaveUser(req.Name, hashName, req.Email); err != nil {
		log.Println("Ошибка при сохранении:", err)
		return &pb.SaveUserResponse{Success: false, Message: err.Error()}, err
	}

	if s.rdb != nil {
		key := "user:" + req.Tgid + ":exists"
		if err := s.rdb.SetEx(ctx, key, "1", userTTL).Err(); err != nil {
			log.Printf("Redis: не удалось установить маркер %s: %v", key, err)
		} else {
			log.Printf("Redis: установлен маркер %s на %v", key, userTTL)
		}
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

	rdb := redisauth.New()

	lis, err := net.Listen("tcp", ":50054")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterAuthServiceServer(grpcServer, &authServer{
		db:  database,
		rdb: rdb,
	})

	log.Println("AuthService listening on :50054")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
