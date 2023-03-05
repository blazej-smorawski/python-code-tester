package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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
	Passed  []int           `json:"passed"`
	Results []RunCodeResult `json:"results"`
}

type RunCodeResult struct {
	TestCase int    `json:"test_case"`
	Output   string `json:"output"`
}

func runCode(code string, input string, test_case int, c chan RunCodeResult) {
	// Create temp file for users code:
	file, err := ioutil.TempFile("temp", "prefix")
	if err != nil {
		c <- RunCodeResult{TestCase: test_case, Output: "Could not create file for users code -> " + err.Error()}
		return
	}
	defer os.Remove(file.Name())

	_, err = file.WriteString(code)
	if err != nil {
		c <- RunCodeResult{TestCase: test_case, Output: "Could not write users code to temporary file"}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python", file.Name())

	stdin := bytes.Buffer{}
	stdin.Write([]byte(input))
	cmd.Stdin = &stdin

	out, err := cmd.CombinedOutput()
	if err != nil {
		c <- RunCodeResult{TestCase: test_case, Output: string(out)}
		return
	}

	c <- RunCodeResult{TestCase: test_case, Output: string(out)}
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

	c := make(chan RunCodeResult)
	for index, test_case := range request.TestCases {
		go runCode(request.Code, test_case.Input, index, c)
	}

	response := TestResponse{make([]int, 0, len(request.TestCases)), make([]RunCodeResult, 0, len(request.TestCases))}
	for range request.TestCases {
		result := <-c
		response.Results = append(response.Results, result)
		if result.Output == request.TestCases[result.TestCase].Output {
			response.Passed = append(response.Passed, result.TestCase)
		}
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
