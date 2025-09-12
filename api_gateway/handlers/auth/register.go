package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/mail"
	"time"

	"github.com/ZnayMed/znaymed-backend/api_gateway/config"
	"github.com/ZnayMed/znaymed-backend/api_gateway/handlers/common"
	pb "github.com/ZnayMed/znaymed-backend/pb"
	"google.golang.org/grpc"
)

type registerReq struct {
	Name  string `json:"name"`
	TgID  string `json:"tgid"`
	Email string `json:"email"`
}

func Register(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			common.BadRequest(w, "method not allowed")
			return
		}

		var req registerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.BadRequest(w, "invalid JSON")
			return
		}
		if req.Name == "" || req.TgID == "" || req.Email == "" {
			common.BadRequest(w, "name, tgid and email are required")
			return
		}
		if _, err := mail.ParseAddress(req.Email); err != nil {
			common.BadRequest(w, "invalid email")
			return
		}

		authConn, err := grpc.Dial(cfg.AuthAddr, grpc.WithInsecure())
		if err != nil {
			common.Internal(w, "gRPC connect to AuthService failed", err)
			return
		}
		defer authConn.Close()

		authClient := pb.NewAuthServiceClient(authConn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		authResp, err := authClient.Register(ctx, &pb.SaveUserRequest{
			Name:  req.Name,
			Tgid:  req.TgID,
			Email: req.Email,
		})
		if err != nil {
			common.Internal(w, "register failed", err)
			return
		}

		if !authResp.Success {
			common.JSON(w, http.StatusBadRequest, map[string]any{
				"success": false,
				"message": authResp.Message,
			})
			return
		}

		var grantOK bool
		var grantMsg string

		const freeSectionTitle = "Остеология"

		courseConn, err := grpc.Dial(cfg.CourseAddr, grpc.WithInsecure())
		if err == nil {
			defer courseConn.Close()
			courseClient := pb.NewCourseServiceClient(courseConn)

			cctx, ccancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer ccancel()

			grantResp, gerr := courseClient.GrantFreeSection(cctx, &pb.GrantFreeSectionRequest{
				Tgid:         req.TgID,
				SectionTitle: freeSectionTitle,
			})
			if gerr == nil {
				grantOK = grantResp.GetSuccess()
				grantMsg = grantResp.GetMessage()
			} else {
				grantOK = false
				grantMsg = "failed to grant free section"
			}
		} else {
			grantOK = false
			grantMsg = "connect to CourseService failed"
		}

		common.JSON(w, http.StatusOK, map[string]any{
			"success":         true,
			"message":         authResp.Message,
			"granted_section": freeSectionTitle,
			"grant_success":   grantOK,
			"grant_message":   grantMsg,
		})

	}
}
