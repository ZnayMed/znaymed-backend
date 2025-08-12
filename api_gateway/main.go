package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	pb "github.com/ZnayMed/znaymed-backend/pb"
	"google.golang.org/grpc"
)

func main() {
	addrAuth := os.Getenv("AUTH_SERVICE_ADDR")
	addrCourse := os.Getenv("COURSE_SERVICE_ADDR")
	addrPayment := os.Getenv("PAYMENT_SERVICE_ADDR")

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
			Name      string `json:"name"`
			TgID      string `json:"tgid"`
			Birthdate string `json:"birthdate"`
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

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := client.Register(ctx, &pb.SaveUserRequest{
			Name:      req.Name,
			Tgid:      req.TgID,
			Birthdate: req.Birthdate,
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

	http.HandleFunc("/createpayment", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TgID    string `json:"tgid"`
			Section string `json:"section"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		conn, err := grpc.Dial(addrPayment, grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()
		client := pb.NewPaymentServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		resp, err := client.CreatePayment(ctx, &pb.CreatePaymentRequest{
			Tgid:     req.TgID,
			CourseId: req.Section,
		})
		if err != nil {
			log.Println("Ошибка в RPC CreatePayment:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"Payment_id":  resp.PaymentId,
			"Payment_url": resp.PaymentUrl,
			"Status":      resp.Status,
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
