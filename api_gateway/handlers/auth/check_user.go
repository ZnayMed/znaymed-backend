package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ZnayMed/znaymed-backend/api_gateway/config"
	"github.com/ZnayMed/znaymed-backend/api_gateway/handlers/common"
	pb "github.com/ZnayMed/znaymed-backend/pb"
	"google.golang.org/grpc"
)

func CheckUser(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tgid := strings.TrimSpace(r.URL.Query().Get("tgid"))
		if tgid == "" {
			common.BadRequest(w, "missing tgid")
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

		resp, err := client.CheckUser(ctx, &pb.UserRequest{Tgid: tgid})
		if err != nil {
			common.Internal(w, "CheckUser failed", err)
			return
		}

		common.JSON(w, http.StatusOK, map[string]bool{"exists": resp.Exists})
	}
}
