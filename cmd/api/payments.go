package main

import (
	"context" 
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sixync/birdlens-be/internal/response" 
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/paymentintent"
)

type createPaymentIntentRequest struct {
	Items []item `json:"items"`
}

type item struct {
	ID     string `json:"id"`
	Amount *int64 `json:"amount"` // Changed to pointer to allow it to be optional
}

func calculateOrderAmountFromServer(app *application, items []item, ctx context.Context) (int64, error) {
	total := int64(0)
	exBirdSub, err := app.store.Users.GetSubscriptionByName(ctx, "ExBird")
	if err != nil {
		return 0, fmt.Errorf("ExBird subscription plan not found: %w", err)
	}

	for _, i := range items {
		if i.ID == "sub_premium" { // "sub_premium" matches your Android client
			expectedAmount := int64(exBirdSub.Price * 100) // Price from DB, in cents
			if i.Amount != nil && *i.Amount != expectedAmount { // If client sent an amount, log if it's different
				app.logger.Warn("Client-sent amount for ExBird subscription does not match server price. Using server price.",
					"clientAmount", *i.Amount, "serverAmount", expectedAmount)
			}
			// Always use the server-defined price for this known subscription
			total += expectedAmount
		} else {
			return 0, fmt.Errorf("unknown item ID in cart: %s", i.ID)
		}
	}
	if total == 0 && len(items) > 0 {
		return 0, errors.New("order amount cannot be zero for valid items")
	}
	return total, nil
}

func (app *application) handleCreatePaymentIntent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		app.methodNotAllowed(w, r)
		return
	}

	user := app.getUserFromFirebaseClaimsCtx(r)
	if user == nil {
		app.unauthorized(w, r)
		return
	}

	dbUser, err := app.store.Users.GetById(r.Context(), user.Id)
	if err != nil {
		app.serverError(w, r, fmt.Errorf("failed to retrieve user details: %w", err))
		return
	}

	if dbUser.SubscriptionId != nil {
		currentSub, err := app.store.Subscriptions.GetUserSubscriptionByEmail(r.Context(), dbUser.Email)
		if err == nil && currentSub != nil && currentSub.Name == "ExBird" {
			if dbUser.StripeSubscriptionStatus != nil && *dbUser.StripeSubscriptionStatus == "active" {
				if dbUser.StripeSubscriptionPeriodEnd != nil && dbUser.StripeSubscriptionPeriodEnd.After(time.Now()) {
					app.logger.Info("User already has an active ExBird subscription", "userID", dbUser.Id)
					response.JSON(w, http.StatusConflict, nil, true, "You already have an active ExBird subscription.")
					return
				}
			}
		}
	}

	var req createPaymentIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.logger.Error("Failed to decode request body for payment intent", "error", err)
		app.badRequest(w, r, err)
		return
	}

	if len(req.Items) == 0 {
		app.badRequest(w, r, errors.New("items list cannot be empty"))
		return
	}

	exBirdSubscription, err := app.store.Users.GetSubscriptionByName(r.Context(), "ExBird")
	if err != nil {
		app.serverError(w, r, fmt.Errorf("could not find ExBird subscription plan: %w", err))
		return
	}

	orderAmount, err := calculateOrderAmountFromServer(app, req.Items, r.Context())
	if err != nil {
		app.badRequest(w, r, err)
		return
	}
	if orderAmount <= 0 {
		app.badRequest(w, r, errors.New("order amount must be positive"))
		return
	}

	stripeCustomerID := ""
	if dbUser.StripeCustomerID != nil && *dbUser.StripeCustomerID != "" {
		stripeCustomerID = *dbUser.StripeCustomerID
	} else {
		customerParams := &stripe.CustomerParams{
			Email: stripe.String(dbUser.Email),
			Name:  stripe.String(dbUser.FirstName + " " + dbUser.LastName),
			Metadata: map[string]string{
				"user_id": strconv.FormatInt(dbUser.Id, 10),
			},
		}
		newCustomer, err := customer.New(customerParams)
		if err != nil {
			app.logger.Error("Failed to create Stripe customer", "error", err, "userID", dbUser.Id)
			app.serverError(w, r, errors.New("could not set up payment customer"))
			return
		}
		stripeCustomerID = newCustomer.ID
		dbUser.StripeCustomerID = &stripeCustomerID
		err = app.store.Users.UpdateUserSubscription(r.Context(), dbUser.Id, 0, stripeCustomerID, "", "", "", time.Time{})
		if err != nil {
			app.logger.Error("Failed to save Stripe customer ID to user", "error", err, "userID", dbUser.Id)
		}
	}

	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(orderAmount),
		Currency: stripe.String(string(stripe.CurrencyEUR)), 
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
		Customer: stripe.String(stripeCustomerID),
		Metadata: map[string]string{
			"user_id":            strconv.FormatInt(dbUser.Id, 10),
			"subscription_db_id": strconv.FormatInt(exBirdSubscription.ID, 10),
			"subscription_name":  exBirdSubscription.Name,
		},
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		app.logger.Error("Failed to create PaymentIntent", "error", err)
		app.serverError(w, r, errors.New("could not process payment, please try again later"))
		return
	}

	app.logger.Info("Created PaymentIntent for User ID", "userID", dbUser.Id, "clientSecretStart", pi.ClientSecret[:10])

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(struct {
		ClientSecret string `json:"clientSecret"`
	}{
		ClientSecret: pi.ClientSecret,
	}); err != nil {
		app.logger.Error("Failed to encode client secret response", "error", err)
		app.serverError(w, r, err)
	}
}