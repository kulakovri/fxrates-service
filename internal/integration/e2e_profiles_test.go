//go:build e2e
// +build e2e

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	baseURL            = "http://localhost:8080"
	expectedFakePrice  = 1.2345
	composeUpTimeout   = 3 * time.Minute
	composeDownTimeout = 1 * time.Minute
	readyTimeout       = 30 * time.Second
	readyPollInterval  = 250 * time.Millisecond
	statusPollTimeout  = 30 * time.Second
	statusPollInterval = 250 * time.Millisecond
	requestContentType = "application/json"
	idempotencyHeader  = "X-Idempotency-Key"
)

type quoteUpdateResponse struct {
	UpdateID string `json:"update_id"`
}

type quoteUpdateDetails struct {
	UpdateID  string    `json:"update_id"`
	Pair      string    `json:"pair"`
	Status    string    `json:"status"`
	Price     *float32  `json:"price"`
	UpdatedAt time.Time `json:"updated_at"`
	Error     *string   `json:"error"`
}

type lastQuoteResponse struct {
	Pair      string    `json:"pair"`
	Price     *float32  `json:"price"`
	UpdatedAt time.Time `json:"updated_at"`
}

func TestE2E_ChanProfile(t *testing.T) {
	if !isE2EEnabled(t) {
		t.Skip("E2E_PROFILES not enabled or docker compose unavailable")
	}
	cleanup := startProfile(t, "chan")
	defer cleanup()

	waitForReady(t, baseURL)
	updateID := postUpdate(t, baseURL, "EUR/USD", "e2e-chan")
	waitForDone(t, baseURL, updateID)
	price := getLastQuote(t, baseURL, "EUR/USD")
	assertApproxEqual(t, price, expectedFakePrice, 1e-4)
}

func TestE2E_DBProfile(t *testing.T) {
	if !isE2EEnabled(t) {
		t.Skip("E2E_PROFILES not enabled or docker compose unavailable")
	}
	cleanup := startProfile(t, "db")
	defer cleanup()

	waitForReady(t, baseURL)
	updateID := postUpdate(t, baseURL, "EUR/USD", "e2e-db")
	waitForDone(t, baseURL, updateID)
	price := getLastQuote(t, baseURL, "EUR/USD")
	assertApproxEqual(t, price, expectedFakePrice, 1e-4)
}

func TestE2E_GRPCProfile(t *testing.T) {
	if !isE2EEnabled(t) {
		t.Skip("E2E_PROFILES not enabled or docker compose unavailable")
	}
	cleanup := startProfile(t, "grpc")
	defer cleanup()

	waitForReady(t, baseURL)
	updateID := postUpdate(t, baseURL, "EUR/USD", "e2e-grpc")
	waitForDone(t, baseURL, updateID)
	price := getLastQuote(t, baseURL, "EUR/USD")
	assertApproxEqual(t, price, expectedFakePrice, 1e-4)
}

func isE2EEnabled(t *testing.T) bool {
	t.Helper()
	if os.Getenv("E2E_PROFILES") != "1" {
		return false
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	// Check docker compose (v2)
	cmd := exec.Command("docker", "compose", "-v")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func startProfile(t *testing.T, profile string) func() {
	t.Helper()
	composeFile := repoPath(t, "ops", "docker", "docker-compose.yml")

	// Ensure clean slate (best-effort)
	_ = runCompose(t, composeDownTimeout, "down -v", profile, composeFile)

	// Up with build
	if err := runCompose(t, composeUpTimeout, "up -d --build", profile, composeFile); err != nil {
		t.Fatalf("failed to start profile %q: %v", profile, err)
	}
	return func() {
		_ = runCompose(t, composeDownTimeout, "down -v", profile, composeFile)
	}
}

func runCompose(t *testing.T, timeout time.Duration, action, profile, composeFile string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	args := []string{"compose", "-f", composeFile, "--profile", profile}
	args = append(args, strings.Split(action, " ")...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	// Force fake provider for deterministic behavior
	cmd.Env = append(os.Environ(), "PROVIDER=fake")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker %s failed: %w\nOutput:\n%s", strings.Join(args, " "), err, string(out))
	}
	return nil
}

func waitForReady(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(readyTimeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/readyz")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			return
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(readyPollInterval)
	}
	t.Fatalf("API did not become ready within %s", readyTimeout)
}

func postUpdate(t *testing.T, baseURL, pair, idem string) string {
	t.Helper()
	body := map[string]string{"pair": pair}
	data, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/quotes/updates", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", requestContentType)
	req.Header.Set(idempotencyHeader, idem)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /quotes/updates failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected status for POST /quotes/updates: got %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
	var out quoteUpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("failed to decode response: %v")
	}
	if out.UpdateID == "" {
		t.Fatalf("missing update_id in response")
	}
	return out.UpdateID
}

func waitForDone(t *testing.T, baseURL, updateID string) {
	t.Helper()
	deadline := time.Now().Add(statusPollTimeout)
	client := &http.Client{Timeout: 5 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/quotes/updates/" + updateID)
		if err != nil {
			time.Sleep(statusPollInterval)
			continue
		}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return
			}
			var det quoteUpdateDetails
			if err := json.NewDecoder(resp.Body).Decode(&det); err != nil {
				return
			}
			switch det.Status {
			case "done":
				return
			case "failed":
				t.Fatalf("update %s failed", updateID)
			}
		}()
		// Check if already done by re-fetching status in a non-blocking way
		ok, err := isDone(baseURL, updateID)
		if err == nil && ok {
			return
		}
		time.Sleep(statusPollInterval)
	}
	t.Fatalf("update %s did not reach done within %s", updateID, statusPollTimeout)
}

func isDone(baseURL, updateID string) (bool, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/quotes/updates/" + updateID)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, errors.New("bad status")
	}
	var det quoteUpdateDetails
	if err := json.NewDecoder(resp.Body).Decode(&det); err != nil {
		return false, err
	}
	return det.Status == "done", nil
}

func getLastQuote(t *testing.T, baseURL, pair string) float64 {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/quotes/last?pair=" + pair)
	if err != nil {
		t.Fatalf("GET /quotes/last failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status for GET /quotes/last: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out lastQuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("failed to decode last quote: %v", err)
	}
	if out.Price == nil {
		t.Fatalf("missing price in last quote response")
	}
	return float64(*out.Price)
}

func assertApproxEqual(t *testing.T, got, want, tol float64) {
	t.Helper()
	if got > want+tol || got < want-tol {
		t.Fatalf("unexpected price: got %.6f, want %.6f (±%.6f)", got, want, tol)
	}
}

func repoPath(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to determine caller")
	}
	// internal/integration -> internal -> repo root
	dir := filepath.Dir(file)
	parent := filepath.Dir(dir)
	root := filepath.Dir(parent)
	full := filepath.Join(root, filepath.Join(parts...))
	return full
}
