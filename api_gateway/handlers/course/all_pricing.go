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

type allPricingReq struct {
	TgID string `json:"tgid"`
}

func AllSubjectsPricing(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			common.BadRequest(w, "method not allowed")
			return
		}

		var req allPricingReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TgID == "" {
			common.BadRequest(w, "invalid json: need tgid")
			return
		}

		err := courseutil.WithClient(cfg, 4*time.Second, func(c pb.CourseServiceClient) error {
			ctx, cancel := grpcx.Context(4 * time.Second)
			defer cancel()

			resp, err := c.AllSubjectsPricing(ctx, &pb.AllSubjectsPricingRequest{Tgid: req.TgID})
			if err != nil {
				return err
			}

			common.JSON(w, http.StatusOK, map[string]any{
				"currency": resp.Currency,
				"subjects": resp.Subjects,
				"total": map[string]any{
					"missing_count":     resp.TotalMissingCount,
					"subtotal_kopeck":   resp.TotalSubtotalKopeck,
					"discounted_kopeck": resp.TotalDiscountedKopeck,
					"applied_rule":      resp.TotalAppliedRule,
				},
			})
			return nil
		})
		if err != nil {
			common.Internal(w, "AllSubjectsPricing failed", err)
		}
	}
}
