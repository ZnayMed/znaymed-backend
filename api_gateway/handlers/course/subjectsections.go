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

type subjectSectionsReq struct {
	TgID    string `json:"tgid"`
	Subject string `json:"subject"`
}

func SubjectSections(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			common.BadRequest(w, "method not allowed")
			return
		}

		var req subjectSectionsReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.BadRequest(w, "invalid JSON")
			return
		}

		err := courseutil.WithClient(cfg, 3*time.Second, func(c pb.CourseServiceClient) error {
			ctx, cancel := grpcx.Context(3 * time.Second)
			defer cancel()

			resp, err := c.GetSubjectSections(ctx, &pb.SubjectSectionsRequest{
				Tgid:    req.TgID,
				Subject: req.Subject,
			})
			if err != nil {
				return err
			}

			type sectionJSON struct {
				Title       string `json:"title"`
				Accessible  bool   `json:"accessible"`
				Description string `json:"description"`
			}
			out := struct {
				Sections []sectionJSON `json:"sections"`
			}{Sections: make([]sectionJSON, 0, len(resp.Sections))}

			for _, s := range resp.Sections {
				out.Sections = append(out.Sections, sectionJSON{
					Title:       s.Title,
					Accessible:  s.Accessible,
					Description: s.Description,
				})
			}

			common.JSON(w, http.StatusOK, out)
			return nil
		})
		if err != nil {
			common.Internal(w, "GetSubjectSections failed", err)
		}
	}
}
