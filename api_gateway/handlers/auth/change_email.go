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

type changeEmailReq struct {
	TgID     string `json:"tgid"`
	NewEmail string `json:"new_email"`
}

func ChangeEmail(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodPut {
			common.BadRequest(w, "method not allowed")
			return
		}

		var req changeEmailReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.BadRequest(w, "invalid JSON")
			return
		}
		if req.TgID == "" || req.NewEmail == "" {
			common.BadRequest(w, "tgid and new_email are required")
			return
		}
		if _, err := mail.ParseAddress(req.NewEmail); err != nil {
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

		resp, err := client.UpdateEmail(ctx, &pb.UpdateEmailRequest{
			Tgid:     req.TgID,
			NewEmail: req.NewEmail,
		})
		if err != nil {
			common.Internal(w, "update email failed", err)
			return
		}

		status := http.StatusOK
		if !resp.Success {
			status = http.StatusBadRequest
		}
		common.JSON(w, status, map[string]any{
			"success": resp.Success,
			"message": resp.Message,
		})
	}
}
