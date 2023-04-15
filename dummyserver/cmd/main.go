package main

import (
	"log"
	"net/http"
)

func main() {
	err := http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//DENY all requests with forbidden
		w.WriteHeader(http.StatusForbidden)
		log.Default().Println("Recieved request")
	}))
	if err != nil {
		log.Fatal(err)
	}
}
