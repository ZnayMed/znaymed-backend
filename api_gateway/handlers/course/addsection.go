package course

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ZnayMed/znaymed-backend/api_gateway/config"
	"github.com/ZnayMed/znaymed-backend/api_gateway/handlers/common"
	"github.com/ZnayMed/znaymed-backend/api_gateway/utils/courseutil"
	"github.com/ZnayMed/znaymed-backend/api_gateway/utils/grpcx"
	pb "github.com/ZnayMed/znaymed-backend/pb"
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

		err := courseutil.WithClient(cfg, 3*time.Second, func(c pb.CourseServiceClient) error {
			ctx, cancel := grpcx.Context(3 * time.Second)
			defer cancel()

			resp, err := c.AddSection(ctx, &pb.SaveSectionRequest{
				Tgid:  req.TgID,
				Title: req.Title,
			})
			if err != nil {
				return err
			}

			common.JSON(w, http.StatusOK, map[string]bool{"success": resp.Success})
			return nil
		})
		if err != nil {
			common.Internal(w, "AddSection failed", err)
		}
	}
}
