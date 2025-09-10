package handlers

import (
	"net/http"

	"github.com/ZnayMed/znaymed-backend/api_gateway/config"
	"github.com/ZnayMed/znaymed-backend/api_gateway/handlers/auth"
	"github.com/ZnayMed/znaymed-backend/api_gateway/handlers/course"
	"github.com/ZnayMed/znaymed-backend/api_gateway/handlers/payment"
)

func RegisterRoutes(mux *http.ServeMux, cfg config.Config) {
	// auth
	mux.HandleFunc("/register", auth.Register(cfg))
	mux.HandleFunc("/is_admin", auth.IsAdmin(cfg))
	mux.HandleFunc("/check_user", auth.CheckUser(cfg))
	mux.HandleFunc("/login", auth.Login(cfg))
	mux.HandleFunc("/verify", auth.Verify(cfg))

	// course
	mux.HandleFunc("/listsubjects", course.ListSubjects(cfg))
	mux.HandleFunc("/addsection", course.AddSection(cfg))
	mux.HandleFunc("/sections/total", course.SectionsTotal(cfg))
	mux.HandleFunc("/subject_total", course.SubjectTotal(cfg))
	mux.HandleFunc("/sectiontopics", course.SectionTopics(cfg))
	mux.HandleFunc("/subjectsections", course.SubjectSections(cfg))

	// payment
	mux.HandleFunc("/createpayment", payment.CreatePayment(cfg))
	mux.HandleFunc("/createpayment_miss_sections", payment.CreatePaymentMissSections(cfg))
}
