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

		err := courseutil.WithClient(cfg, 3*time.Second, func(c pb.CourseServiceClient) error {
			ctx, cancel := grpcx.Context(3 * time.Second)
			defer cancel()

			resp, err := c.SubjectMissingTotal(ctx, &pb.SubjectMissingTotalRequest{
				Tgid:    req.TgID,
				Subject: req.Subject,
			})
			if err != nil {
				return err
			}

			common.JSON(w, http.StatusOK, map[string]any{
				"subject":      resp.Subject,
				"total_kopeck": resp.TotalKopeck,
				"currency":     resp.Currency,
			})
			return nil
		})
		if err != nil {
			common.Internal(w, "SubjectMissingTotal failed", err)
		}
	}
}
