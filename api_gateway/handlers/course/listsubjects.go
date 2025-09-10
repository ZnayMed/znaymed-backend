package course

import (
	"context"
	"net/http"
	"time"

	"github.com/ZnayMed/znaymed-backend/api_gateway/config"
	"github.com/ZnayMed/znaymed-backend/api_gateway/handlers/common"
	pb "github.com/ZnayMed/znaymed-backend/pb"
	"google.golang.org/grpc"
)

func ListSubjects(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := grpc.Dial(cfg.CourseAddr, grpc.WithInsecure())
		if err != nil {
			common.Internal(w, "gRPC connect failed", err)
			return
		}
		defer conn.Close()
		client := pb.NewCourseServiceClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.GetListSubjects(ctx, &pb.ListSubjectsRequest{})
		if err != nil {
			common.Internal(w, "ListSubjects failed", err)
			return
		}
		common.JSON(w, http.StatusOK, map[string]any{"subjects": resp.Titles})
	}
}
