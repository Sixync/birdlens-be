// birdlens-be/cmd/api/webhooks.go
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

func (app *application) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	const MaxBodyBytes = int64(65536) // 64KB
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		app.logger.Error("Error reading Stripe webhook request body", "error", err)
		http.Error(w, "Error reading request body", http.StatusServiceUnavailable)
		return
	}

	endpointSecret := app.config.stripe.webhookSecret // Load this from env
	if endpointSecret == "" {
		app.logger.Error("Stripe webhook secret is not configured")
		http.Error(w, "Webhook secret configuration error", http.StatusInternalServerError)
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), endpointSecret)
	if err != nil {
		app.logger.Error("Error constructing Stripe webhook event", "error", err)
		http.Error(w, "Webhook signature verification failed", http.StatusBadRequest)
		return
	}

	app.logger.Info("Received Stripe webhook event", "type", event.Type, "id", event.ID)

	switch event.Type {
	case "payment_intent.succeeded":
		var paymentIntent stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &paymentIntent)
		if err != nil {
			app.logger.Error("Error unmarshaling payment_intent.succeeded", "error", err, "eventID", event.ID)
			http.Error(w, "Error processing webhook", http.StatusInternalServerError)
			return
		}
		app.logger.Info("PaymentIntent Succeeded", "paymentIntentID", paymentIntent.ID, "amount", paymentIntent.Amount)

		// Extract metadata
		userIDStr, okUserID := paymentIntent.Metadata["user_id"]
		subscriptionDbIDStr, okSubDbID := paymentIntent.Metadata["subscription_db_id"]

		if !okUserID || !okSubDbID {
			app.logger.Error("Missing user_id or subscription_db_id in PaymentIntent metadata", "paymentIntentID", paymentIntent.ID)
			// Still return 200 to Stripe to acknowledge receipt, but log error for investigation.
			w.WriteHeader(http.StatusOK)
			return
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			app.logger.Error("Invalid user_id in PaymentIntent metadata", "userID", userIDStr, "paymentIntentID", paymentIntent.ID)
			w.WriteHeader(http.StatusOK)
			return
		}
		subscriptionDbID, err := strconv.ParseInt(subscriptionDbIDStr, 10, 64)
		if err != nil {
			app.logger.Error("Invalid subscription_db_id in PaymentIntent metadata", "subscriptionDbID", subscriptionDbIDStr, "paymentIntentID", paymentIntent.ID)
			w.WriteHeader(http.StatusOK)
			return
		}

		// TODO: Fetch subscription details (e.g., duration) based on subscriptionDbID if needed for period_end
		// For simplicity, let's assume ExBird is always 30 days for this example.
		// In a real app, fetch the subscription from app.store.Subscriptions.GetByID(subscriptionDbID)
		// and use its DurationDays.
		exBirdSub, err := app.store.Users.GetSubscriptionByName(r.Context(), "ExBird")
		if err != nil {
			app.logger.Error("Could not find ExBird subscription details in DB for webhook", "error", err)
			w.WriteHeader(http.StatusOK)
			return
		}
		durationDays := exBirdSub.DurationDays
		periodEnd := time.Now().AddDate(0, 0, durationDays)

		// Update user's subscription status in your database
		err = app.store.Users.UpdateUserSubscription(
			r.Context(),
			userID,
			subscriptionDbID,
			paymentIntent.Customer.ID, // Stripe Customer ID from PaymentIntent
			"",                          // Stripe Subscription ID (empty if not using Stripe Subscriptions API directly for this PI)
			"",                          // Stripe Price ID (empty for same reason)
			"active",                    // Status
			periodEnd,                   // Period End
		)
		if err != nil {
			app.logger.Error("Failed to update user subscription after webhook", "userID", userID, "error", err)
			// Decide if you want to retry or send 500 to Stripe. For critical fulfillment, 500 might prompt Stripe to retry.
			// http.Error(w, "Failed to update user record", http.StatusInternalServerError)
			// For now, acknowledge to Stripe and log for manual intervention.
			w.WriteHeader(http.StatusOK)
			return
		}
		app.logger.Info("User subscription updated successfully via webhook", "userID", userID, "subscriptionDbID", subscriptionDbID)

	// Handle other event types as needed (e.g., payment_intent.payment_failed, invoice.paid for recurring)
	// case "invoice.paid":
	// ...
	default:
		app.logger.Info("Unhandled Stripe event type", "type", event.Type)
	}

	w.WriteHeader(http.StatusOK)
}