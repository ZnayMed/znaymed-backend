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

type addSectionReq struct {
	TgID  string `json:"tgid"`
	Title string `json:"title"`
}

func AddSection(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req addSectionReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.BadRequest(w, "invalid JSON")
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

		resp, err := client.AddSection(ctx, &pb.SaveSectionRequest{
			Tgid:  req.TgID,
			Title: req.Title,
		})
		if err != nil {
			common.Internal(w, "AddSection failed", err)
			return
		}
		common.JSON(w, http.StatusOK, map[string]bool{"success": resp.Success})
	}
}
