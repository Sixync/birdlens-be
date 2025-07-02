// path: birdlens-be/cmd/api/payments_payos.go
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
)

// createPayOSPaymentRequest is the struct for the request body sent from our Android client.
type createPayOSPaymentRequest struct {
	Items []item `json:"items"` // Reuses the same item struct from payments.go
}

// PayOSRequestData is the struct we build and send to the PayOS API.
type PayOSRequestData struct {
	OrderCode   int64  `json:"orderCode"`
	Amount      int64  `json:"amount"`
	Description string `json:"description"`
	BuyerName   string `json:"buyerName"`
	BuyerEmail  string `json:"buyerEmail"`
	BuyerPhone  string `json:"buyerPhone"`
	ReturnUrl   string `json:"returnUrl"`
	CancelUrl   string `json:"cancelUrl"`
	Signature   string `json:"signature"`
	ExpiredAt   int64  `json:"expiredAt,omitempty"`
}

// PayOSResponseData is the struct for the successful response from the PayOS API.
type PayOSResponseData struct {
	Code string `json:"code"`
	Desc string `json:"desc"`
	Data *struct {
		Bin           string `json:"bin"`
		AccountNumber string `json:"accountNumber"`
		AccountName   string `json:"accountName"`
		Amount        int64  `json:"amount"`
		Description   string `json:"description"`
		OrderCode     int64  `json:"orderCode"`
		PaymentLinkID string `json:"paymentLinkId"`
		CheckoutURL   string `json:"checkoutUrl"`
		QRCode        string `json:"qrCode"`
	} `json:"data"`
	Signature string `json:"signature"`
}

// PayOSWebhookData is the struct for the data part of an incoming webhook from PayOS.
type PayOSWebhookData struct {
	Code string `json:"code"`
	Desc string `json:"desc"`
	Data *struct {
		OrderCode     int64  `json:"orderCode"`
		Amount        int64  `json:"amount"`
		Description   string `json:"description"`
		AccountNumber string `json:"accountNumber"`
		Reference     string `json:"reference"`
		TransactionTs string `json:"transactionDateTime"`
	} `json:"data"`
	Signature string `json:"signature"`
}

func (app *application) createPayOSPaymentLinkHandler(w http.ResponseWriter, r *http.Request) {
	user := app.getUserFromFirebaseClaimsCtx(r)
	if user == nil {
		app.unauthorized(w, r)
		return
	}

	var req createPayOSPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	exBirdSubscription, err := app.store.Users.GetSubscriptionByName(r.Context(), "ExBird")
	if err != nil {
		app.serverError(w, r, fmt.Errorf("could not find ExBird subscription plan: %w", err))
		return
	}

	// For PayOS, we use a fixed amount in VND.
	// 20,000 VND for the ExBird subscription.
	orderAmount := int64(20000)

	// Create a unique order code for PayOS (must be an integer).
	orderCode := time.Now().UnixNano() / int64(time.Millisecond)

	// 1. Create a new order in our database with status "PENDING"
	newOrder := &store.Order{
		UserID:         user.Id,
		SubscriptionID: exBirdSubscription.ID,
		PaymentGateway: "payos",
		GatewayOrderID: strconv.FormatInt(orderCode, 10),
		Amount:         orderAmount,
		Currency:       "VND",
		Status:         store.OrderStatusPending,
	}

	err = app.store.Orders.Create(r.Context(), newOrder)
	if err != nil {
		app.logger.Error("Failed to create pending order in DB for PayOS", "error", err, "userID", user.Id)
		app.serverError(w, r, errors.New("failed to initialize payment"))
		return
	}

	// 2. Prepare the request to send to PayOS API
	payOSReq := &PayOSRequestData{
		OrderCode:   orderCode,
		Amount:      orderAmount,
		Description: fmt.Sprintf("Birdlens ExBird Subscription for %s", user.Email),
		BuyerName:   fmt.Sprintf("%s %s", user.FirstName, user.LastName),
		BuyerEmail:  user.Email,
		BuyerPhone:  "0931137913", // Placeholder phone number
		ReturnUrl:   "app://birdlens/payment-success",
		CancelUrl:   "app://birdlens/payment-cancel",
		ExpiredAt:   time.Now().Add(15 * time.Minute).Unix(),
	}

	// 3. Create the signature for the request
	signatureData := fmt.Sprintf("amount=%d&cancelUrl=%s&description=%s&orderCode=%d&returnUrl=%s",
		payOSReq.Amount, payOSReq.CancelUrl, payOSReq.Description, payOSReq.OrderCode, payOSReq.ReturnUrl)
	payOSReq.Signature = createPayOSSignature(signatureData, app.config.payos.checksumKey)

	// 4. Send the request to PayOS
	reqBodyBytes, _ := json.Marshal(payOSReq)
	payOSApiUrl := "https://api-merchant.payos.vn/v2/payment-requests"

	reqHttp, err := http.NewRequest("POST", payOSApiUrl, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	reqHttp.Header.Set("Content-Type", "application/json")
	reqHttp.Header.Set("x-client-id", app.config.payos.clientID)
	reqHttp.Header.Set("x-api-key", app.config.payos.apiKey)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(reqHttp)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		app.logger.Error("PayOS API error", "status", resp.Status, "body", string(bodyBytes))
		app.errorMessage(w, r, http.StatusBadGateway, "Payment provider service is unavailable", nil)
		return
	}

	var payOSResponse PayOSResponseData
	if err := json.Unmarshal(bodyBytes, &payOSResponse); err != nil {
		app.serverError(w, r, fmt.Errorf("failed to decode payos response: %w", err))
		return
	}

	if payOSResponse.Code != "00" || payOSResponse.Data == nil {
		app.logger.Error("PayOS did not create payment link successfully", "code", payOSResponse.Code, "desc", payOSResponse.Desc)
		app.serverError(w, r, errors.New("failed to create payment link"))
		return
	}

	// 5. Return the checkoutUrl to the Android client
	response.JSON(w, http.StatusOK, map[string]string{"checkoutUrl": payOSResponse.Data.CheckoutURL}, false, "Payment link created")
}

func (app *application) handlePayOSWebhook(w http.ResponseWriter, r *http.Request) {
	var webhookReq PayOSWebhookData
	if err := json.NewDecoder(r.Body).Decode(&webhookReq); err != nil {
		app.logger.Error("Failed to decode PayOS webhook body", "error", err)
		app.badRequest(w, r, errors.New("invalid webhook payload"))
		return
	}

	if webhookReq.Data == nil {
		app.logger.Error("PayOS webhook payload is missing 'data' object")
		app.badRequest(w, r, errors.New("invalid webhook payload: missing data"))
		return
	}

	// 1. Verify the signature to ensure the webhook is from PayOS
	if !verifyPayOSSignature(webhookReq, app.config.payos.checksumKey) {
		app.logger.Warn("Invalid PayOS webhook signature received", "orderCode", webhookReq.Data.OrderCode)
		app.errorMessage(w, r, http.StatusUnauthorized, "Invalid signature", nil)
		return
	}

	// 2. Process the webhook based on the payment code
	if webhookReq.Code == "00" { // "00" means PAID
		gatewayOrderID := strconv.FormatInt(webhookReq.Data.OrderCode, 10)
		slog.Info("Received successful PayOS payment webhook", "gateway_order_id", gatewayOrderID, "amount", webhookReq.Data.Amount)

		// 3. Find the order in our database
		order, err := app.store.Orders.GetByGatewayOrderID(r.Context(), gatewayOrderID)
		if err != nil {
			app.logger.Error("PayOS webhook: order not found in DB", "gateway_order_id", gatewayOrderID, "error", err)
			app.notFound(w, r)
			return
		}

		// 4. If the order is PENDING, update it to PAID and grant the subscription
		if order.Status == store.OrderStatusPending {
			err = app.store.Orders.UpdateStatus(r.Context(), order.ID, store.OrderStatusPaid)
			if err != nil {
				app.logger.Error("Failed to update order status to PAID", "orderID", order.ID, "error", err)
				app.serverError(w, r, err)
				return
			}

			// Grant the user their subscription
			err = app.store.Users.GrantSubscriptionForOrder(r.Context(), order.UserID, order.SubscriptionID)
			if err != nil {
				app.logger.Error("Failed to grant subscription after PAID webhook", "orderID", order.ID, "userID", order.UserID, "error", err)
				app.serverError(w, r, err)
				return
			}
			slog.Info("Subscription granted successfully via PayOS webhook", "userID", order.UserID, "orderID", order.ID)
		} else {
			slog.Warn("Received PayOS webhook for an order that is not PENDING", "orderID", order.ID, "currentStatus", order.Status)
		}
	} else {
		slog.Info("Received non-success PayOS webhook", "code", webhookReq.Code, "desc", webhookReq.Desc, "orderCode", webhookReq.Data.OrderCode)
	}

	w.WriteHeader(http.StatusOK)
}

// --- Helper Functions for PayOS ---

func createPayOSSignature(data string, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func verifyPayOSSignature(webhookData PayOSWebhookData, checksumKey string) bool {
	var dataAsMap map[string]interface{}
	dataBytes, err := json.Marshal(webhookData.Data)
	if err != nil {
		slog.Error("Failed to marshal webhook data for signature verification", "error", err)
		return false
	}
	json.Unmarshal(dataBytes, &dataAsMap)

	var keys []string
	for k := range dataAsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var dataBuilder strings.Builder
	for i, k := range keys {
		if i > 0 {
			dataBuilder.WriteString("&")
		}
		var valueStr string
		switch v := dataAsMap[k].(type) {
		case float64:
			valueStr = strconv.FormatInt(int64(v), 10)
		default:
			valueStr = fmt.Sprintf("%v", v)
		}
		dataBuilder.WriteString(fmt.Sprintf("%s=%s", k, valueStr))
	}
	dataString := dataBuilder.String()

	expectedSignature := createPayOSSignature(dataString, checksumKey)

	return hmac.Equal([]byte(webhookData.Signature), []byte(expectedSignature))
}