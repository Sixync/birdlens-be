// birdlens-be/cmd/api/services.go (New File or added to an existing services file)
package main

import (
	"log/slog"
	"net/http"

	"github.com/sixync/birdlens-be/internal/response"
)

// handleSendNewsletter is triggered by a scheduler (e.g., cron job).
// It compiles unprocessed updates and sends them to all users.
func (app *application) handleSendNewsletter(w http.ResponseWriter, r *http.Request) {
	// Optional: Add a secret header/token check to ensure this is only called by your scheduler.
	// For example: if r.Header.Get("X-Scheduler-Secret") != app.config.schedulerSecret { ... }

	ctx := r.Context()

	// 1. Fetch all unprocessed updates from the database.
	updates, err := app.store.NewsletterUpdates.GetUnprocessed(ctx)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if len(updates) == 0 {
		slog.Info("Newsletter job ran, but no new updates to send.")
		response.JSON(w, http.StatusOK, nil, false, "No new updates to send.")
		return
	}

	// 2. Fetch all user emails.
	userEmails, err := app.store.Users.GetAllUserEmails(ctx)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if len(userEmails) == 0 {
		slog.Info("Newsletter job ran, but no users to send email to.")
		response.JSON(w, http.StatusOK, nil, false, "No users to send to.")
		return
	}

	// 3. Prepare and queue the emails.
	emailData := map[string]interface{}{
		"Updates": updates,
	}

	slog.Info("Sending weekly newsletter", "update_count", len(updates), "user_count", len(userEmails))

	for _, email := range userEmails {
		// Use the existing background worker pool to send emails.
		JobQueue <- EmailJob{
			Recipient: email,
			Data:      emailData,
			Patterns:  []string{"weekly_newsletter.tmpl"},
		}
	}

	// 4. Mark the updates as processed to avoid re-sending.
	updateIDs := make([]int64, len(updates))
	for i, u := range updates {
		updateIDs[i] = u.ID
	}

	err = app.store.NewsletterUpdates.MarkAsProcessed(ctx, updateIDs)
	if err != nil {
		// Log this error critically, as it could cause duplicate emails.
		app.logger.Error("CRITICAL: Failed to mark newsletter updates as processed.", "error", err)
		app.serverError(w, r, err) // Still report success to the scheduler to prevent retries
		return
	}

	response.JSON(w, http.StatusOK, nil, false, "Newsletter sending process initiated.")
}