package course

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

type subjectTotalReq struct {
	TgID    string `json:"tgid"`
	Subject string `json:"subject"`
}

func SubjectTotal(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			common.BadRequest(w, "method not allowed")
			return
		}

		var req subjectTotalReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TgID == "" || req.Subject == "" {
			common.BadRequest(w, "invalid json: need tgid and subject")
			return
		}

		conn, err := grpc.Dial(cfg.CourseAddr, grpc.WithInsecure())
		if err != nil {
			common.Internal(w, "gRPC connect failed", err)
			return
		}
		defer conn.Close()
		client := pb.NewCourseServiceClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.SubjectMissingTotal(ctx, &pb.SubjectMissingTotalRequest{
			Tgid:    req.TgID,
			Subject: req.Subject,
		})
		if err != nil {
			common.Internal(w, "SubjectMissingTotal failed", err)
			return
		}

		common.JSON(w, http.StatusOK, map[string]any{
			"subject":      resp.Subject,
			"total_kopeck": resp.TotalKopeck,
			"currency":     resp.Currency,
		})
	}
}
