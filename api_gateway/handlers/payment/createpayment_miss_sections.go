package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ZnayMed/znaymed-backend/api_gateway/config"
	"github.com/ZnayMed/znaymed-backend/api_gateway/handlers/common"
	"github.com/ZnayMed/znaymed-backend/api_gateway/utils/authutil"
	pb "github.com/ZnayMed/znaymed-backend/pb"
	"google.golang.org/grpc"
)

type createMissReq struct {
	TgID     string   `json:"tgid"`
	Subjects []string `json:"subjects"`
}

func CreatePaymentMissSections(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			common.BadRequest(w, "method not allowed")
			return
		}
		var req createMissReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TgID == "" || len(req.Subjects) == 0 {
			common.BadRequest(w, "invalid json: need tgid and subjects")
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

		missResp, err := cClient.MissingSectionsBySubjects(ctxCourse, &pb.MissingSectionsRequest{
			Tgid:     req.TgID,
			Subjects: req.Subjects,
		})
		if err != nil {
			common.Internal(w, "MissingSectionsBySubjects failed", err)
			return
		}

		seen := make(map[string]struct{})
		bySubject := make(map[string][]string, len(missResp.Result))
		var missingSections []string
		for subj, pack := range missResp.Result {
			if pack == nil {
				continue
			}
			for _, s := range pack.SectionIds {
				if _, ok := seen[s]; ok {
					continue
				}
				seen[s] = struct{}{}
				missingSections = append(missingSections, s)
				bySubject[subj] = append(bySubject[subj], s)
			}
		}

		if len(missingSections) == 0 {
			common.JSON(w, http.StatusOK, map[string]any{
				"message":      "nothing to buy: all sections already owned",
				"sections":     []string{},
				"by_subject":   bySubject,
				"total_kopeck": 0,
				"currency":     "RUB",
			})
			return
		}

		priceResp, err := cClient.PriceMissingFromList(ctxCourse, &pb.PriceMissingRequest{
			Tgid:     req.TgID,
			Sections: missingSections,
		})
		if err != nil {
			common.Internal(w, "PriceMissingFromList failed", err)
			return
		}

		missingForPayment := priceResp.MissingSections
		total := priceResp.TotalKopeck
		if len(missingForPayment) == 0 || total <= 0 {
			common.JSON(w, http.StatusOK, map[string]any{
				"message":      "nothing to buy after price check",
				"sections":     []string{},
				"by_subject":   bySubject,
				"total_kopeck": 0,
				"currency":     priceResp.Currency,
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
			CourseIds:    missingForPayment,
			AmountKopeck: total,
			Email:        email,
		})
		if err != nil {
			common.Internal(w, "payment create failed", err)
			return
		}

		common.JSON(w, http.StatusOK, map[string]any{
			"payment_id":   pResp.PaymentId,
			"payment_url":  pResp.PaymentUrl,
			"status":       pResp.Status,
			"sections":     missingForPayment,
			"by_subject":   bySubject,
			"total_kopeck": total,
			"currency":     priceResp.Currency,
		})
	}
}
