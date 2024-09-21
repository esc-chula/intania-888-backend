package bill

import "github.com/wiraphatys/intania888/internal/model"

type BillRepository interface {
	Create(bill *model.BillHead) error
	GetById(id string) (*model.BillHead, error)
	GetAll() ([]*model.BillHead, error)
	Update(bill *model.BillHead) error
	Delete(id string) error
}

type BillService interface {
	CreateBill(billDto *model.BillHeadDto) error
	GetBill(id string) (*model.BillHeadDto, error)
	GetAllBills() ([]*model.BillHeadDto, error)
	UpdateBill(billDto *model.BillHeadDto) error
	DeleteBill(id string) error
}
