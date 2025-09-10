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

type sectionTopicsReq struct {
	SectionTitle string `json:"section_title"`
}

func SectionTopics(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			common.BadRequest(w, "method not allowed")
			return
		}

		var req sectionTopicsReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SectionTitle == "" {
			common.BadRequest(w, "bad json: section_title is required")
			return
		}

		err := courseutil.WithClient(cfg, 3*time.Second, func(c pb.CourseServiceClient) error {
			ctx, cancel := grpcx.Context(3 * time.Second)
			defer cancel()

			resp, err := c.GetTopicsBySectionTitle(ctx, &pb.SectionTitleRequest{
				SectionTitle: req.SectionTitle,
			})
			if err != nil {
				return err
			}

			type topicJSON struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				TgId        string `json:"tg_id"`
				MindmapUrl  string `json:"mindmap_url"`
			}
			out := make([]topicJSON, 0, len(resp.Topics))
			for _, t := range resp.Topics {
				out = append(out, topicJSON{
					Title:       t.Title,
					Description: t.Description,
					TgId:        t.TgId,
					MindmapUrl:  t.MindmapUrl,
				})
			}

			common.JSON(w, http.StatusOK, map[string]any{
				"section_title": req.SectionTitle,
				"topics":        out,
			})
			return nil
		})
		if err != nil {
			common.Internal(w, "GetTopicsBySectionTitle failed", err)
		}
	}
}
