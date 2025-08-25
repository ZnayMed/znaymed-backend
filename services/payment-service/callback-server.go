package main

import (
	"log"
	"net/http"
)

func StartCallbackHTTPServer(server *paymentServer) {
	http.HandleFunc("/psp-callback", server.PSPCallback)
	http.HandleFunc("/psp-callback-test", server.PSPCallbackTest)

	go func() {
		log.Println("📡 HTTP Callback server listening on :8081")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Fatalf("callback server failed: %v", err)
		}
	}()
}
