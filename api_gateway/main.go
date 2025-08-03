package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	pb "github.com/ZnayMed/znaymed-backend/pb"
	"google.golang.org/grpc"
)

func main() {

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		conn, err := grpc.Dial("localhost:50053", grpc.WithInsecure())
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

		conn, err := grpc.Dial("localhost:50053", grpc.WithInsecure())
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

		conn, err := grpc.Dial("localhost:50054", grpc.WithInsecure()) // порт auth-сервиса
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
			json.NewEncoder(w).Encode(map[string]bool{"success": false})
			return
		}

		json.NewEncoder(w).Encode(map[string]bool{"success": resp.Success})
	})
	http.HandleFunc("/sections", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TgID string `json:"tgid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		conn, err := grpc.Dial("localhost:50053", grpc.WithInsecure())
		if err != nil {
			http.Error(w, "gRPC connect failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := pb.NewCourseServiceClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		resp, err := client.GetUserSections(ctx, &pb.UserRequest{
			Tgid: req.TgID,
		})
		if err != nil {
			log.Println("Ошибка в gRPC GetUserSections:", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get sections",
			})
			return
		}

		json.NewEncoder(w).Encode(map[string][]string{
			"sections": resp.TopicTitles,
		})
	})

	log.Println("API Gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))

}
