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

type subjectSectionsReq struct {
	TgID    string `json:"tgid"`
	Subject string `json:"subject"`
}

func SubjectSections(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req subjectSectionsReq
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

		resp, err := client.GetSubjectSections(ctx, &pb.SubjectSectionsRequest{
			Tgid:    req.TgID,
			Subject: req.Subject,
		})
		if err != nil {
			common.Internal(w, "GetSubjectSections failed", err)
			return
		}

		type sectionJSON struct {
			Title      string `json:"title"`
			Accessible bool   `json:"accessible"`
		}
		out := struct {
			Sections []sectionJSON `json:"sections"`
		}{Sections: make([]sectionJSON, 0, len(resp.Sections))}

		for _, s := range resp.Sections {
			out.Sections = append(out.Sections, sectionJSON{
				Title:      s.Title,
				Accessible: s.Accessible,
			})
		}
		common.JSON(w, http.StatusOK, out)
	}
}
