// Command docxlint validates .docx files against the OOXML schema.
//
// It delegates to whichever validator backend is available:
//  1. OOXMLValidatorCLI binary on PATH (fastest, no Docker)
//  2. Docker image docx-go-validator built from tools/validator/
//
// Usage:
//
//	docxlint file1.docx [file2.docx ...]
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: docxlint file1.docx [file2.docx ...]")
		os.Exit(2)
	}
	files := os.Args[1:]

	results, err := validate(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docxlint: %v\n", err)
		os.Exit(2)
	}

	hasErrors := false
	for _, r := range results {
		for _, e := range r.Errors {
			fmt.Printf("ERROR %s: %s\n", r.File, e)
			hasErrors = true
		}
		for _, w := range r.Warnings {
			fmt.Printf("WARN  %s: %s\n", r.File, w)
		}
	}

	if hasErrors {
		os.Exit(1)
	}
	fmt.Printf("OK — %d file(s) validated\n", len(results))
}

type validationResult struct {
	File     string   `json:"file"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

func validate(files []string) ([]validationResult, error) {
	// Try OOXMLValidatorCLI on PATH first.
	if path, err := exec.LookPath("OOXMLValidatorCLI"); err == nil {
		return runCLI(path, files)
	}
	// Fall back to Docker.
	if _, err := exec.LookPath("docker"); err == nil {
		return runDocker(files)
	}
	return nil, fmt.Errorf("no validator backend found: install OOXMLValidatorCLI or Docker")
}

func runCLI(bin string, files []string) ([]validationResult, error) {
	args := append([]string{}, files...)
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Exit 1 = validation errors found; parse the output.
		} else {
			return nil, fmt.Errorf("OOXMLValidatorCLI: %w", err)
		}
	}
	return parseJSONLines(out)
}

func runDocker(files []string) ([]validationResult, error) {
	args := []string{"run", "--rm"}
	// Use a counter-based container path so files from different directories
	// with the same basename do not collide on the same mount point.
	containerPaths := make([]string, len(files))
	for i, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			return nil, fmt.Errorf("abs path %s: %w", f, err)
		}
		containerPaths[i] = fmt.Sprintf("/files/input_%d.docx", i)
		args = append(args, "-v", abs+":"+containerPaths[i]+":ro")
	}
	args = append(args, "docx-go-validator")
	args = append(args, containerPaths...)
	out, err := exec.Command("docker", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Exit 1 = validation errors; parse output.
		} else {
			return nil, fmt.Errorf("docker run: %w", err)
		}
	}
	return parseJSONLines(out)
}

func parseJSONLines(data []byte) ([]validationResult, error) {
	var results []validationResult
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var r validationResult
		if err := dec.Decode(&r); err != nil {
			return nil, fmt.Errorf("parse validator output: %w", err)
		}
		results = append(results, r)
	}
	return results, nil
}
