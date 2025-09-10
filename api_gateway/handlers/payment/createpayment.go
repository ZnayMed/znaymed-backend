package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ZnayMed/znaymed-backend/api_gateway/config"
	"github.com/ZnayMed/znaymed-backend/api_gateway/handlers/common"
	"github.com/ZnayMed/znaymed-backend/api_gateway/utils/authutil"
	pb "github.com/ZnayMed/znaymed-backend/pb"
	"google.golang.org/grpc"
)

type createReq struct {
	TgID     string   `json:"tgid"`
	Sections []string `json:"sections"`
	Section  string   `json:"section"`
}

func CreatePayment(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.BadRequest(w, "invalid JSON")
			return
		}

		courseIDs := req.Sections
		if len(courseIDs) == 0 && strings.TrimSpace(req.Section) != "" {
			courseIDs = []string{strings.TrimSpace(req.Section)}
		}
		if req.TgID == "" || len(courseIDs) == 0 {
			common.BadRequest(w, "missing tgid or sections")
			return
		}

		cConn, err := grpc.Dial(cfg.CourseAddr, grpc.WithInsecure())
		if err != nil {
			common.Internal(w, "gRPC connect to course failed", err)
			return
		}
		defer cConn.Close()
		cClient := pb.NewCourseServiceClient(cConn)

		ctxCourse, cancelCourse := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelCourse()

		priceResp, err := cClient.PriceMissingFromList(ctxCourse, &pb.PriceMissingRequest{
			Tgid:     req.TgID,
			Sections: courseIDs,
		})
		if err != nil {
			common.Internal(w, "price calculation failed", err)
			return
		}
		if len(priceResp.MissingSections) == 0 || priceResp.TotalKopeck <= 0 {
			common.JSON(w, http.StatusOK, map[string]any{
				"message":          "nothing to buy: all provided sections already owned",
				"missing_sections": []string{},
				"total_kopeck":     0,
				"currency":         "RUB",
			})
			return
		}

		email, err := authutil.GetUserEmail(cfg, req.TgID)

		pConn, err := grpc.Dial(cfg.PaymentAddr, grpc.WithInsecure())
		if err != nil {
			common.Internal(w, "gRPC connect to payment failed", err)
			return
		}
		defer pConn.Close()
		pClient := pb.NewPaymentServiceClient(pConn)

		ctxPay, cancelPay := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelPay()

		pResp, err := pClient.CreatePayment(ctxPay, &pb.CreatePaymentRequest{
			Tgid:         req.TgID,
			CourseIds:    priceResp.MissingSections,
			AmountKopeck: priceResp.TotalKopeck,
			Email:        email,
		})
		if err != nil {
			common.Internal(w, "payment create failed", err)
			return
		}

		common.JSON(w, http.StatusOK, map[string]any{
			"payment_id":       pResp.PaymentId,
			"payment_url":      pResp.PaymentUrl,
			"status":           pResp.Status,
			"missing_sections": priceResp.MissingSections,
			"total_kopeck":     priceResp.TotalKopeck,
			"currency":         priceResp.Currency,
		})
	}
}
