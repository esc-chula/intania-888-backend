package bill

import "github.com/esc-chula/intania-888-backend/internal/model"

type BillRepository interface {
	Create(bill *model.BillHead) error
	GetById(billId, userId string) (*model.BillHead, error)
	GetAll(userId string) ([]*model.BillHead, error)
	Update(bill *model.BillHead) error
	Delete(id string) error
}

type BillService interface {
	CreateBill(userProfile *model.UserDto, billDto *model.BillHeadDto) error
	GetBill(billId, userId string) (*model.BillHeadDto, error)
	GetAllBills(userId string) ([]*model.BillHeadDto, error)
	UpdateBill(billDto *model.BillHeadDto) error
	DeleteBill(id string) error
}
