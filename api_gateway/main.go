package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	pb "github.com/ZnayMed/znaymed-backend/pb"
	"google.golang.org/grpc"
)

func main() {
	addrAuth := os.Getenv("AUTH_SERVICE_ADDR")
	addrCourse := os.Getenv("COURSE_SERVICE_ADDR")
	addrPayment := os.Getenv("PAYMENT_SERVICE_ADDR")

	http.HandleFunc("/createpayment_miss_sections", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			TgID     string   `json:"tgid"`
			Subjects []string `json:"subjects"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TgID == "" || len(req.Subjects) == 0 {
			http.Error(w, "invalid json: need tgid and subjects", http.StatusBadRequest)
			return
		}

		cConn, err := grpc.Dial(addrCourse, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect to course failed", http.StatusInternalServerError)
			return
		}
		defer cConn.Close()
		cClient := pb.NewCourseServiceClient(cConn)

		ctxCourse, cancelCourse := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelCourse()

		cResp, err := cClient.MissingSectionsBySubjects(ctxCourse, &pb.MissingSectionsRequest{
			Tgid: req.TgID, Subjects: req.Subjects,
		})
		if err != nil {
			http.Error(w, "MissingSectionsBySubjects failed", http.StatusInternalServerError)
			return
		}

		bySubject := make(map[string][]string, len(cResp.Result))
		seen := make(map[string]struct{})
		var missingSections []string
		for subj, pack := range cResp.Result {
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
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message":      "nothing to buy: all sections already owned",
				"sections":     []string{},
				"by_subject":   bySubject,
				"total_kopeck": 0,
				"currency":     "RUB",
			})
			return
		}

		priceResp, err := cClient.PriceMissingFromList(ctxCourse, &pb.PriceMissingRequest{
			Tgid: req.TgID, Sections: missingSections,
		})
		if err != nil {
			http.Error(w, "PriceMissingFromList failed", http.StatusInternalServerError)
			return
		}

		missingForPayment := priceResp.MissingSections
		total := priceResp.TotalKopeck
		if len(missingForPayment) == 0 || total <= 0 {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message":      "nothing to buy after price check",
				"sections":     []string{},
				"by_subject":   bySubject,
				"total_kopeck": 0,
				"currency":     "RUB",
			})
			return
		}

		pConn, err := grpc.Dial(addrPayment, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect to payment failed", http.StatusInternalServerError)
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
		})
		if err != nil {
			log.Println("CreatePayment RPC failed:", err)
			http.Error(w, "payment create failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"payment_id":   pResp.PaymentId,
			"payment_url":  pResp.PaymentUrl,
			"status":       pResp.Status,
			"sections":     missingForPayment,
			"by_subject":   bySubject,
			"total_kopeck": total,
			"currency":     priceResp.Currency,
		})
	})

	http.HandleFunc("/subject_total", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			TgID    string `json:"tgid"`
			Subject string `json:"subject"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TgID == "" || req.Subject == "" {
			http.Error(w, "invalid json: need tgid and subject", http.StatusBadRequest)
			return
		}

		conn, err := grpc.Dial(addrCourse, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := pb.NewCourseServiceClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.SubjectMissingTotal(ctx, &pb.SubjectMissingTotalRequest{
			Tgid:    req.TgID,
			Subject: req.Subject,
		})
		if err != nil {
			http.Error(w, "SubjectMissingTotal failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"subject":      resp.Subject,
			"total_kopeck": resp.TotalKopeck,
			"currency":     resp.Currency,
		})
	})

	http.HandleFunc("/sectiontopics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var reqBody struct {
			SectionTitle string `json:"section_title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			log.Println("Ошибка декодирования JSON:", err)
			return
		}
		if reqBody.SectionTitle == "" {
			http.Error(w, "section_title is required", http.StatusBadRequest)
			return
		}

		conn, err := grpc.Dial(addrCourse, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect failed", http.StatusInternalServerError)
			log.Println("Ошибка подключения к gRPC:", err)
			return
		}
		defer conn.Close()

		client := pb.NewCourseServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.GetTopicsBySectionTitle(ctx, &pb.SectionTitleRequest{
			SectionTitle: reqBody.SectionTitle,
		})
		if err != nil {
			http.Error(w, "gRPC call failed", http.StatusInternalServerError)
			log.Println("Ошибка вызова GetTopicsBySectionTitle:", err)
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

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"section_title": reqBody.SectionTitle,
			"topics":        out,
		})
	})

	http.HandleFunc("/check_user", func(w http.ResponseWriter, r *http.Request) {
		tgid := r.URL.Query().Get("tgid")
		if tgid == "" {
			http.Error(w, "missing tgid", http.StatusBadRequest)
			return
		}

		conn, err := grpc.Dial(addrAuth, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := pb.NewAuthServiceClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.CheckUser(ctx, &pb.UserRequest{Tgid: tgid})
		if err != nil {
			http.Error(w, "CheckUser failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"exists": resp.Exists})
	})

	http.HandleFunc("/is_admin", func(w http.ResponseWriter, r *http.Request) {
		tgid := r.URL.Query().Get("tgid")
		if strings.TrimSpace(tgid) == "" {
			http.Error(w, "missing tgid", http.StatusBadRequest)
			return
		}

		conn, err := grpc.Dial(addrAuth, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := pb.NewAuthServiceClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.IsAdmin(ctx, &pb.UserRequest{Tgid: tgid})
		if err != nil {
			http.Error(w, "IsAdmin failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{
			"is_admin": resp.IsAdmin,
		})
	})

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		conn, err := grpc.Dial(addrAuth, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := pb.NewAuthServiceClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		resp, err := client.Login(ctx, &pb.LoginRequest{
			Username: req.Username,
			Password: req.Password,
		})
		if err != nil {
			http.Error(w, "Auth failed", http.StatusUnauthorized)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"token": resp.Token})
	})

	http.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		conn, err := grpc.Dial(addrAuth, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := pb.NewAuthServiceClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		resp, err := client.VerifyToken(ctx, &pb.VerifyRequest{Token: req.Token})
		if err != nil {
			http.Error(w, "Verification failed", http.StatusUnauthorized)
			return
		}

		json.NewEncoder(w).Encode(map[string]bool{"valid": resp.Valid})
	})

	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name  string `json:"name"`
			TgID  string `json:"tgid"`
			Email string `json:"email"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Name == "" || req.TgID == "" || req.Email == "" {
			http.Error(w, "name, tgid and email are required", http.StatusBadRequest)
			return
		}
		if _, err := mail.ParseAddress(req.Email); err != nil {
			http.Error(w, "invalid email", http.StatusBadRequest)
			return
		}

		conn, err := grpc.Dial(addrAuth, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := pb.NewAuthServiceClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.Register(ctx, &pb.SaveUserRequest{
			Name:  req.Name,
			Tgid:  req.TgID,
			Email: req.Email,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "internal error: " + err.Error(),
			})
			return
		}

		if !resp.Success {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": resp.Message,
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": resp.Message,
		})
	})

	http.HandleFunc("/listsubjects", func(w http.ResponseWriter, r *http.Request) {
		conn, err := grpc.Dial(addrCourse, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect failed", http.StatusInternalServerError)
			log.Println("Ошибка подключения к gRPC:", err)
			return
		}
		defer conn.Close()

		client := pb.NewCourseServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.GetListSubjects(ctx, &pb.ListSubjectsRequest{})
		if err != nil {
			http.Error(w, "gRPC call failed", http.StatusInternalServerError)
			log.Println("Ошибка вызова ListSubjects:", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"subjects": resp.Titles,
		})
	})

	http.HandleFunc("/addsection", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TgID  string `json:"tgid"`
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		conn, err := grpc.Dial(addrCourse, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := pb.NewCourseServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.AddSection(ctx, &pb.SaveSectionRequest{
			Tgid:  req.TgID,
			Title: req.Title,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]bool{"success": false})
			return
		}

		json.NewEncoder(w).Encode(map[string]bool{"success": resp.Success})
	})

	http.HandleFunc("/sections/total", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			TgID     string   `json:"tgid"`
			Sections []string `json:"sections"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TgID == "" || len(req.Sections) == 0 {
			http.Error(w, "invalid json: need tgid and sections", http.StatusBadRequest)
			return
		}

		conn, err := grpc.Dial(addrCourse, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect to course failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()
		client := pb.NewCourseServiceClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.PriceMissingFromList(ctx, &pb.PriceMissingRequest{
			Tgid:     req.TgID,
			Sections: req.Sections,
		})
		if err != nil {
			log.Println("PriceMissingFromList RPC failed:", err)
			http.Error(w, "price calculation failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"missing_sections": resp.MissingSections,
			"total_kopeck":     resp.TotalKopeck,
			"currency":         resp.Currency,
		})
	})

	http.HandleFunc("/createpayment", func(w http.ResponseWriter, r *http.Request) {
		type createReq struct {
			TgID     string   `json:"tgid"`
			Sections []string `json:"sections"`
			Section  string   `json:"section"`
		}

		var req createReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		courseIDs := req.Sections
		if len(courseIDs) == 0 && strings.TrimSpace(req.Section) != "" {
			courseIDs = []string{strings.TrimSpace(req.Section)}
		}
		if req.TgID == "" || len(courseIDs) == 0 {
			http.Error(w, "missing tgid or sections", http.StatusBadRequest)
			return
		}

		cConn, err := grpc.Dial(addrCourse, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect to course failed", http.StatusInternalServerError)
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
			log.Println("PriceMissingFromList RPC failed:", err)
			http.Error(w, "price calculation failed", http.StatusInternalServerError)
			return
		}

		missing := priceResp.MissingSections
		total := priceResp.TotalKopeck
		currency := priceResp.Currency

		if len(missing) == 0 || total <= 0 {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message":          "nothing to buy: all provided sections already owned",
				"missing_sections": []string{},
				"total_kopeck":     0,
				"currency":         "RUB",
			})
			return
		}

		pConn, err := grpc.Dial(addrPayment, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect to payment failed", http.StatusInternalServerError)
			return
		}
		defer pConn.Close()
		pClient := pb.NewPaymentServiceClient(pConn)

		ctxPay, cancelPay := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelPay()

		pResp, err := pClient.CreatePayment(ctxPay, &pb.CreatePaymentRequest{
			Tgid:         req.TgID,
			CourseIds:    missing,
			AmountKopeck: total,
		})
		if err != nil {
			log.Println("CreatePayment RPC failed:", err)
			http.Error(w, "payment create failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"payment_id":       pResp.PaymentId,
			"payment_url":      pResp.PaymentUrl,
			"status":           pResp.Status,
			"missing_sections": missing,
			"total_kopeck":     total,
			"currency":         currency,
		})
	})

	http.HandleFunc("/subjectsections", func(w http.ResponseWriter, r *http.Request) {

		var reqBody struct {
			TgID    string `json:"tgid"`
			Subject string `json:"subject"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		conn, err := grpc.Dial(addrCourse, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := pb.NewCourseServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.GetSubjectSections(ctx, &pb.SubjectSectionsRequest{
			Tgid:    reqBody.TgID,
			Subject: reqBody.Subject,
		})
		if err != nil {
			http.Error(w, "gRPC call failed", http.StatusInternalServerError)
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

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
	log.Println("API Gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
