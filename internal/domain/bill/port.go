package bill

import "github.com/esc-chula/intania-888-backend/internal/model"

type BillRepository interface {
	GetById(billID, userID string) (*model.BillHead, error)
	GetAll(userID string) ([]*model.BillHead, error)
	GetAllAdmin() ([]*model.BillHead, error)
}

type BillService interface {
	CreateBill(userID string, req *model.CreateBillRequest) (*model.BillHeadDto, error)
	GetBill(billID, userID string) (*model.BillHeadDto, error)
	GetAllBills(userID string) ([]*model.BillHeadDto, error)
	GetAllBillsAdmin() ([]*model.BillHeadDto, error)
	VoidBill(billID, actorID, reason string) (*model.BillHeadDto, error)
}
