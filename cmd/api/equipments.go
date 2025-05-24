package main

type EquipmentResponse struct {
	EquipmentName string `json:"equipment_name"`
	Quantity      int64  `json:"quantity"`
	Price         int64  `json:"price"`
	ImageUrl      string `json:"image_url"`
}

func (e *EquipmentResponse) toResponse() EquipmentResponse {
	return EquipmentResponse{
		EquipmentName: e.EquipmentName,
		Quantity:      e.Quantity,
		Price:         e.Price,
		ImageUrl:      e.ImageUrl,
	}
}
