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

		conn, err := grpc.Dial(cfg.CourseAddr, grpc.WithInsecure())
		if err != nil {
			common.Internal(w, "gRPC connect failed", err)
			return
		}
		defer conn.Close()
		client := pb.NewCourseServiceClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.GetTopicsBySectionTitle(ctx, &pb.SectionTitleRequest{
			SectionTitle: req.SectionTitle,
		})
		if err != nil {
			common.Internal(w, "GetTopicsBySectionTitle failed", err)
			return
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
	}
}
