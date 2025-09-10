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

type sectionsTotalReq struct {
	TgID     string   `json:"tgid"`
	Sections []string `json:"sections"`
}

func SectionsTotal(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			common.BadRequest(w, "method not allowed")
			return
		}

		var req sectionsTotalReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TgID == "" || len(req.Sections) == 0 {
			common.BadRequest(w, "invalid json: need tgid and sections")
			return
		}

		conn, err := grpc.Dial(cfg.CourseAddr, grpc.WithInsecure())
		if err != nil {
			common.Internal(w, "gRPC connect to course failed", err)
			return
		}
		defer conn.Close()

		client := pb.NewCourseServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.PriceMissingFromList(ctx, &pb.PriceMissingRequest{
			Tgid:     req.TgID,
			Sections: req.Sections,
		})
		if err != nil {
			common.Internal(w, "price calculation failed", err)
			return
		}

		common.JSON(w, http.StatusOK, map[string]any{
			"missing_sections": resp.MissingSections,
			"total_kopeck":     resp.TotalKopeck,
			"currency":         resp.Currency,
		})
	}
}
