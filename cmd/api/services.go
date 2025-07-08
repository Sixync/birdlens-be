package main

import (
	"log/slog"
	"net/http"

	"github.com/sixync/birdlens-be/internal/response"
)

// handleSendNewsletter is triggered by an authenticated admin.
// It compiles unprocessed updates and sends them to all users.
func (app *application) handleSendNewsletter(w http.ResponseWriter, r *http.Request) {
	// Admin access is already verified by the adminOnlyMiddleware.
	ctx := r.Context()

	// Fetch all unprocessed updates from the database.
	updates, err := app.store.NewsletterUpdates.GetUnprocessed(ctx)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if len(updates) == 0 {
		slog.Info("Admin triggered newsletter, but no new updates to send.")
		response.JSON(w, http.StatusOK, nil, false, "No new updates to send.")
		return
	}

	// Fetch all verified user emails.
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

	// Prepare and queue the emails using the background worker pool.
	emailData := map[string]interface{}{
		"Updates": updates,
	}

	slog.Info("Sending weekly newsletter via admin trigger", "update_count", len(updates), "user_count", len(userEmails))

	for _, email := range userEmails {
		JobQueue <- EmailJob{
			Recipient: email,
			Data:      emailData,
			Patterns:  []string{"weekly_newsletter.tmpl"},
		}
	}

	// Mark the updates as processed to avoid re-sending.
	updateIDs := make([]int64, len(updates))
	for i, u := range updates {
		updateIDs[i] = u.ID
	}

	err = app.store.NewsletterUpdates.MarkAsProcessed(ctx, updateIDs)
	if err != nil {
		app.logger.Error("CRITICAL: Failed to mark newsletter updates as processed.", "error", err)
		// We still return a success to the admin dashboard but log the critical error.
		response.JSON(w, http.StatusAccepted, nil, false, "Newsletter sending initiated, but failed to mark updates as processed.")
		return
	}

	response.JSON(w, http.StatusAccepted, nil, false, "Newsletter sending process initiated.")
}