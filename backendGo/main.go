package main

import (
	"fmt"
	"net/http"
)

type Account struct {
	acc_id   uint64
	userName string
	email    string
}

type character struct {
	char_id  uint64
	acc_id   uint64
	class_id uint8
}

type scores struct {
	score_id     uint64
	char_id      uint64
	reward_score uint64
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World!")
}

func main() {
	http.HandleFunc("/", handler)
	fmt.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Error starting server:", err)
	}
}
