package course

import (
	"net/http"
	"time"

	"github.com/ZnayMed/znaymed-backend/api_gateway/config"
	"github.com/ZnayMed/znaymed-backend/api_gateway/handlers/common"
	"github.com/ZnayMed/znaymed-backend/api_gateway/utils/courseutil"
	"github.com/ZnayMed/znaymed-backend/api_gateway/utils/grpcx"
	pb "github.com/ZnayMed/znaymed-backend/pb"
)

func ListSubjects(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := courseutil.WithClient(cfg, 3*time.Second, func(c pb.CourseServiceClient) error {
			ctx, cancel := grpcx.Context(3 * time.Second)
			defer cancel()

			resp, err := c.GetListSubjects(ctx, &pb.ListSubjectsRequest{})
			if err != nil {
				return err
			}

			type subj struct {
				Title       string `json:"title"`
				Description string `json:"description"`
			}
			outSubjects := make([]subj, 0, len(resp.GetSubjects()))
			for _, s := range resp.GetSubjects() {
				outSubjects = append(outSubjects, subj{
					Title:       s.GetTitle(),
					Description: s.GetDescription(),
				})
			}
			common.JSON(w, http.StatusOK, map[string]any{
				"subjects": outSubjects,
				"titles":   resp.GetTitles(),
			})
			return nil
		})
		if err != nil {
			common.Internal(w, "ListSubjects failed", err)
		}
	}
}
