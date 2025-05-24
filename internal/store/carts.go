package store

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Cart struct {
	ID        int64       `json:"id"`
	CartItems []*CartItem `json:"cart_items"`
}

type CartItem struct {
	ID                 int64                `json:"id"`
	CartId             int64                `json:"cart_id"`
	TourId             int64                `json:"tour_id"`
	Tour               *Tour                `json:"tour"`
	ParticipantsNo     int64                `json:"participants_no"`
	CartItemEquipments []*CartItemEquipment `json:"cart_item_equipments"`
}

type CartItemEquipment struct {
	ID          int64      `json:"id"`
	CartItemId  int64      `json:"cart_item_id"`
	EquipmentId int64      `json:"equipment_id"`
	Equipment   *Equipment `json:"equipment"`
	Quantity    int64      `json:"quantity"`
}

type CartStore struct {
	db *sqlx.DB
}

type cartItemRow struct {
	CiID             int64  `db:"ci_id"`
	CiCartID         int64  `db:"ci_cart_id"`
	CiTourID         int64  `db:"ci_tour_id"`
	CiParticipantsNo int64  `db:"ci_participants_no"`
	TId              int64  `db:"t_id"`
	TName            string `db:"t_name"`
	CieID            int64  `db:"cie_id"`
	CieCartItemID    int64  `db:"cie_cart_item_id"`
	CieEquipmentID   int64  `db:"cie_equipment_id"`
	CieQuantity      int64  `db:"cie_quantity"`
	EId              int64  `db:"e_id"`
	EName            string `db:"e_name"`
}

func (s *CartStore) GetDetailedCartByID(ctx context.Context, id int64) (*Cart, error) {
	query := `
		SELECT
			ci.id AS ci_id,
			ci.cart_id AS ci_cart_id,
			ci.tour_id AS ci_tour_id,
			ci.participants_no AS ci_participants_no,
			t.id AS t_id,
			t.name AS t_name,
			cie.id AS cie_id,
			cie.cart_item_id AS cie_cart_item_id,
			cie.equipment_id AS cie_equipment_id,
			cie.quantity AS cie_quantity,
			e.id AS e_id,
			e.name AS e_name
		FROM cart_items ci
		LEFT JOIN tours t ON ci.tour_id = t.id
		LEFT JOIN cart_item_equipments cie ON cie.cart_item_id = ci.id
		LEFT JOIN equipment e ON cie.equipment_id = e.id
		WHERE ci.cart_id = $1
	`

	// Execute the query with context
	rows, err := s.db.QueryxContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Use a map to track cart items by ID and a slice to maintain order
	cartItemsMap := make(map[int64]*CartItem)
	var cartItems []*CartItem

	// Process each row
	for rows.Next() {
		var row cartItemRow
		if err := rows.StructScan(&row); err != nil {
			return nil, err
		}

		// Check if the cart item already exists in the map
		ci, exists := cartItemsMap[row.CiID]
		if !exists {
			// Create a new CartItem
			ci = &CartItem{
				ID:             row.CiID,
				CartId:         row.CiCartID,
				TourId:         row.CiTourID,
				ParticipantsNo: row.CiParticipantsNo,
				Tour: &Tour{
					ID:   row.TId,
					Name: row.TName,
					// Add other Tour fields here if present in the query
				},
				CartItemEquipments: []*CartItemEquipment{},
			}
			cartItemsMap[row.CiID] = ci
			cartItems = append(cartItems, ci)
		}

		// Add equipment if present (CieID != 0 indicates equipment exists)
		if row.CieID != 0 {
			cie := &CartItemEquipment{
				ID:          row.CieID,
				CartItemId:  row.CieCartItemID,
				EquipmentId: row.CieEquipmentID,
				Quantity:    row.CieQuantity,
				Equipment: &Equipment{
					ID:   row.EId,
					Name: row.EName,
					// Add other Equipment fields here if present in the query
				},
			}
			ci.CartItemEquipments = append(ci.CartItemEquipments, cie)
		}
	}

	// Check for errors during row iteration
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Construct the Cart object
	cart := &Cart{
		ID:        id,
		CartItems: cartItems,
	}

	return cart, nil
}

func (s *CartStore) GetCartItemByCartId(ctx context.Context, id int64) ([]*CartItem, error) {
	var cartItems []*CartItem
	query := `SELECT id, cart_id, tour_id, participants_no FROM cart_items WHERE cart_id = $1`
	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		cartItem := &CartItem{}
		err := rows.Scan(
			&cartItem.ID,
			&cartItem.CartId,
			&cartItem.TourId,
			&cartItem.ParticipantsNo,
		)
		if err != nil {
			return nil, err
		}
		cartItems = append(cartItems, cartItem)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cartItems, nil
}
