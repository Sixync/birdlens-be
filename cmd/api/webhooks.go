package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sixync/birdlens-be/internal/env"
	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
)

// GitHubCommit represents the structure of a single commit in the webhook payload.
type GitHubCommit struct {
	ID        string `json:"id"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Author    struct {
		Name string `json:"name"`
	} `json:"author"`
}

// GitHubWebhookPayload represents the structure of the JSON payload from GitHub.
type GitHubWebhookPayload struct {
	Ref     string         `json:"ref"`
	Commits []GitHubCommit `json:"commits"`
}

// handleGitHubWebhook is the handler for incoming GitHub webhooks.
func (app *application) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	// Verify the signature to ensure the request is genuinely from GitHub.
	githubSignature := r.Header.Get("X-Hub-Signature-256")
	webhookSecret := env.GetString("GITHUB_WEBHOOK_SECRET", "")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		app.badRequest(w, r, errors.New("cannot read request body"))
		return
	}

	if webhookSecret == "" {
		app.logger.Error("CRITICAL: GITHUB_WEBHOOK_SECRET is not set. Webhook cannot be verified.")
		app.serverError(w, r, errors.New("internal server configuration error"))
		return
	}

	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write(body)
	expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(githubSignature), []byte(expectedSignature)) {
		app.logger.Warn("Invalid GitHub webhook signature received.")
		app.errorMessage(w, r, http.StatusUnauthorized, "Invalid signature", nil)
		return
	}

	var payload GitHubWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		app.badRequest(w, r, errors.New("invalid JSON payload"))
		return
	}

	// Process only commits to main or master branches.
	if !strings.HasSuffix(payload.Ref, "/main") && !strings.HasSuffix(payload.Ref, "/master") {
		response.JSON(w, http.StatusOK, nil, false, "Webhook received, but not for main/master branch. No action taken.")
		return
	}

	var updatesAdded int
	for _, commit := range payload.Commits {
		// Only include commits with specific user-facing prefixes.
		if strings.HasPrefix(commit.Message, "feat:") ||
			strings.HasPrefix(commit.Message, "fix:") ||
			strings.HasPrefix(commit.Message, "update:") {

			commitTimestamp, _ := time.Parse(time.RFC3339, commit.Timestamp)

			update := &store.NewsletterUpdate{
				CommitHash:  commit.ID,
				Message:     strings.TrimSpace(strings.SplitN(commit.Message, "\n", 2)[0]),
				Author:      commit.Author.Name,
				CommittedAt: commitTimestamp,
			}

			err := app.store.NewsletterUpdates.Create(r.Context(), update)
			if err == nil {
				updatesAdded++
			} else {
				app.logger.Warn("Failed to save newsletter update for commit", "hash", commit.ID, "error", err)
			}
		}
	}

	slog.Info("GitHub webhook processed.", "updates_added", updatesAdded)
	response.JSON(w, http.StatusOK, nil, false, "Webhook processed successfully")
}