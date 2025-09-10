package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ZnayMed/znaymed-backend/api_gateway/config"
	"github.com/ZnayMed/znaymed-backend/api_gateway/handlers/common"
	pb "github.com/ZnayMed/znaymed-backend/pb"
	"google.golang.org/grpc"
)

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Login(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.BadRequest(w, "invalid JSON")
			return
		}

		conn, err := grpc.Dial(cfg.AuthAddr, grpc.WithInsecure())
		if err != nil {
			common.Internal(w, "gRPC connect failed", err)
			return
		}
		defer conn.Close()
		client := pb.NewAuthServiceClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.Login(ctx, &pb.LoginRequest{
			Username: req.Username,
			Password: req.Password,
		})
		if err != nil {
			http.Error(w, "Auth failed", http.StatusUnauthorized)
			return
		}
		common.JSON(w, http.StatusOK, map[string]string{"token": resp.Token})
	}
}
