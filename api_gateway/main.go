package main

import (
	"log"
	"net/http"
	"os"

	"github.com/ZnayMed/znaymed-backend/api_gateway/config"
	"github.com/ZnayMed/znaymed-backend/api_gateway/handlers"
)

func main() {
	cfg := config.Config{
		AuthAddr:    os.Getenv("AUTH_SERVICE_ADDR"),
		CourseAddr:  os.Getenv("COURSE_SERVICE_ADDR"),
		PaymentAddr: os.Getenv("PAYMENT_SERVICE_ADDR"),
	}
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux, cfg)
	log.Println("API Gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
