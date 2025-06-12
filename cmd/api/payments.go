// birdlens-be/cmd/api/payments.go
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/paymentintent"
)

type createPaymentIntentRequest struct {
	Items []item `json:"items"`
	// Add other fields like currency if you want it to be dynamic from client
	// For now, currency is hardcoded to EUR in the handler.
}

type item struct {
	ID     string `json:"id"` // ID of the item (e.g., "tour-123", "subscription-premium")
	Amount int64  `json:"amount"` // Amount for this single item in the smallest currency unit (e.g., cents)
	// Quantity could also be added here if items can have quantities
}

// calculateOrderAmount calculates the total order amount from a list of items.
// IMPORTANT: In a real application, you should fetch item prices from your database
// based on the item IDs to prevent price manipulation from the client.
// For this example, we'll assume the client sends the correct 'amount' for each item.
// A more robust server-side calculation would look up item.ID in DB to get its price.
func calculateOrderAmount(items []item) int64 {
	total := int64(0)
	for _, i := range items {
		// Basic example: Summing amounts sent by client.
		// Production: Look up i.ID in database to get its price, then multiply by quantity if applicable.
		// For instance:
		// priceFromDB, err := getItemPriceFromDB(i.ID)
		// if err != nil { /* handle error */ }
		// total += priceFromDB * i.Quantity
		total += i.Amount
	}
	return total
}

func (app *application) handleCreatePaymentIntent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		app.methodNotAllowed(w, r)
		return
	}

	var req createPaymentIntentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.logger.Error("Failed to decode request body for payment intent", "error", err)
		app.badRequest(w, r, err)
		return
	}

	if len(req.Items) == 0 {
		app.badRequest(w, r, NewError("items list cannot be empty"))
		return
	}

	orderAmount := calculateOrderAmount(req.Items)
	if orderAmount <= 0 { // Stripe requires a positive amount
		app.badRequest(w, r, NewError("order amount must be positive"))
		return
	}

	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(orderAmount),
		Currency: stripe.String(string(stripe.CurrencyEUR)), // Or make this configurable/part of request
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
		// You might want to add metadata here, like user ID, order ID, etc.
		// Metadata: map[string]string{
		//  "order_id": "your_internal_order_id",
		//  "user_id": "user_from_context_if_authenticated",
		// },
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		app.logger.Error("Failed to create PaymentIntent", "error", err)
		// It's good practice to not expose raw Stripe errors directly to client in production.
		// Map to a generic server error.
		app.serverError(w, r, NewError("could not process payment, please try again later"))
		return
	}

	log.Printf("Created PaymentIntent with ID: %s, ClientSecret: %s...", pi.ID, pi.ClientSecret[:10])

	// The response from the backend endpoint is just the client secret
	// and not wrapped in the standard JsonResponse struct for this specific example.
	// If you prefer to wrap it, adjust the writeJSON in the Stripe example or response.JSON here.
	// To match the Stripe example closely:
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(struct {
		ClientSecret string `json:"clientSecret"`
	}{
		ClientSecret: pi.ClientSecret,
	}); err != nil {
		app.logger.Error("Failed to encode client secret response", "error", err)
		app.serverError(w, r, err) // This uses your app.serverError which wraps it.
	}
}

// Helper to create error instances easily, can be moved to a common place
type Error struct {
	Message string
}
func (e *Error) Error() string {
	return e.Message
}
func NewError(message string) error {
	return &Error{Message: message}
}