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
