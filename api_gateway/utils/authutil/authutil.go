package authutil

import (
	"strings"
	"time"

	"github.com/ZnayMed/znaymed-backend/api_gateway/config"
	"github.com/ZnayMed/znaymed-backend/api_gateway/utils/grpcx"
	pb "github.com/ZnayMed/znaymed-backend/pb"
)

func GetUserEmail(cfg config.Config, tgid string) (string, error) {
	conn, err := grpcx.Dial(cfg.AuthAddr)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	client := pb.NewAuthServiceClient(conn)
	ctx, cancel := grpcx.Context(3 * time.Second)
	defer cancel()

	info, err := client.GetUser(ctx, &pb.UserRequest{Tgid: tgid})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(info.GetEmail()), nil
}
