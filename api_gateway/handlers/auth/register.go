package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/mail"
	"time"

	"github.com/ZnayMed/znaymed-backend/api_gateway/config"
	"github.com/ZnayMed/znaymed-backend/api_gateway/handlers/common"
	pb "github.com/ZnayMed/znaymed-backend/pb"
	"google.golang.org/grpc"
)

type registerReq struct {
	Name  string `json:"name"`
	TgID  string `json:"tgid"`
	Email string `json:"email"`
}

func Register(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			common.BadRequest(w, "method not allowed")
			return
		}

		var req registerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.BadRequest(w, "invalid JSON")
			return
		}
		if req.Name == "" || req.TgID == "" || req.Email == "" {
			common.BadRequest(w, "name, tgid and email are required")
			return
		}
		if _, err := mail.ParseAddress(req.Email); err != nil {
			common.BadRequest(w, "invalid email")
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

		resp, err := client.Register(ctx, &pb.SaveUserRequest{
			Name:  req.Name,
			Tgid:  req.TgID,
			Email: req.Email,
		})
		if err != nil {
			common.Internal(w, "register failed", err)
			return
		}
		code := http.StatusOK
		if !resp.Success {
			code = http.StatusBadRequest
		}
		common.JSON(w, code, map[string]any{
			"success": resp.Success,
			"message": resp.Message,
		})
	}
}
