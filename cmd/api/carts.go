package main

// type CartResponse struct {
// 	CartItems []CartItemResponse `json:"cart_items"`
// }
//
// type CartItemResponse struct {
// 	TourName         string              `json:"tour_name"`
// 	TourThumbnailUrl *string             `json:"tour_thumbnail_url"`
// 	ParticipantsNo   int64               `json:"participants_no"`
// 	Equipments       []EquipmentResponse `json:"equipments"`
// }
//
// func (app *application) getCartHandler(w http.ResponseWriter, r *http.Request) {
// 	userClaims := getUserClaimsFromCtx(r)
// 	ctx := r.Context()
//
// 	var cartResponse CartResponse
//
// 	cartItems, err := app.store.Carts.GetCartItemByCartId(ctx, userClaims.ID)
//
// 	var CartItemResponses []CartItemResponse
// 	for _, item := range cartItems {
//     itemResponse := CartItemResponse{
//       TourName: item.Tour.Name,
//       TourThumbnailUrl: item.Tour.ThumbnailUrl,
//       ParticipantsNo: item.Tour.
//     }
// 	}
// 	if err != nil {
// 		app.serverError(w, r, err)
// 		return
// 	}
//
// 	if len(cartItems) == 0 {
// 		response.JSON(w, http.StatusOK, cartResponse, false, "get successful")
// 		return
// 	}
//
// 	for _, item := range cartItems {
// 		cartItemResponse := item.toResponse()
// 		cartResponse.CartItems = append(cartResponse.CartItems, cartItemResponse)
// 	}
//
// 	response.JSON(w, http.StatusOK, cartItems, false, "get successful")
// }
