package courseutil

import (
	"time"

	"github.com/ZnayMed/znaymed-backend/api_gateway/config"
	"github.com/ZnayMed/znaymed-backend/api_gateway/utils/grpcx"
	pb "github.com/ZnayMed/znaymed-backend/pb"
)

func WithClient(cfg config.Config, timeout time.Duration, fn func(pb.CourseServiceClient) error) error {
	conn, err := grpcx.Dial(cfg.CourseAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewCourseServiceClient(conn)
	return fn(client)
}
