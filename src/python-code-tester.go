package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"time"
)

type TestCase struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

type TestRequest struct {
	TestCases []TestCase `json:"test_cases"`
	Code      string
}

type TestResponse struct {
	Passed []int `json:passed`
}

func runCode(code string, input string, expected string, index int, c chan int) {
	result := -1

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python", "python-exec", code)
	//cmd := exec.CommandContext(ctx, "ls")

	stdin := bytes.Buffer{}
	stdin.Write([]byte(input))
	cmd.Stdin = &stdin

	out, err := cmd.CombinedOutput()
	if ctx.Err() != context.DeadlineExceeded {
		c <- -1
		return
	}
	if err != nil {
		c <- -1
		return
	}

	if string(out) == expected {
		c <- index
		return
	}

	c <- result
}

func testCode(w http.ResponseWriter, r *http.Request) {
	var request TestRequest

	// Try to decode the request body into the struct. If there is an error,
	// respond to the client with the error message and a 400 status code.
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c := make(chan int)
	for index, test_case := range request.TestCases {
		go runCode(request.Code, test_case.Input, test_case.Output, index, c)
	}

	response := TestResponse{make([]int, 0, len(request.TestCases))}
	for range request.TestCases {
		index := <-c
		if index == -1 {
			continue
		}
		response.Passed = append(response.Passed, index)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/test_code", testCode)

	err := http.ListenAndServe(":4000", mux)
	log.Fatal(err)
}
