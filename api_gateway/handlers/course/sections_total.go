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

		err := courseutil.WithClient(cfg, 3*time.Second, func(c pb.CourseServiceClient) error {
			ctx, cancel := grpcx.Context(3 * time.Second)
			defer cancel()

			resp, err := c.PriceMissingFromList(ctx, &pb.PriceMissingRequest{
				Tgid:     req.TgID,
				Sections: req.Sections,
			})
			if err != nil {
				return err
			}
			common.JSON(w, http.StatusOK, map[string]any{
				"missing_sections": resp.MissingSections,
				"total_kopeck":     resp.TotalKopeck,
				"currency":         resp.Currency,
			})
			return nil
		})
		if err != nil {
			common.Internal(w, "price calculation failed", err)
		}
	}
}
