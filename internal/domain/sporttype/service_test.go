package sporttype

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"
)

type memoryRepository struct {
	rows  map[string]*SportType
	inUse map[string]bool
	err   error
	ctx   context.Context
	calls int
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		rows: map[string]*SportType{
			"S": {
				ID:    "S",
				Title: "Sport",
			},
			"USED": {
				ID:    "USED",
				Title: "Used sport",
			},
			"admin": {
				ID:    "admin",
				Title: "Admin is a valid ID",
			},
		},
		inUse: map[string]bool{"USED": true},
	}
}

func (r *memoryRepository) record(ctx context.Context) error {
	r.ctx = ctx
	r.calls++

	return r.err
}

func (r *memoryRepository) GetAllSportTypes(ctx context.Context) ([]*SportType, error) {
	if err := r.record(ctx); err != nil {
		return nil, err
	}
	rows := make([]*SportType, 0, len(r.rows))
	for _, row := range r.rows {
		rows = append(rows, row)
	}

	return rows, nil
}

func (r *memoryRepository) GetSportType(ctx context.Context, id string) (*SportType, error) {
	if err := r.record(ctx); err != nil {
		return nil, err
	}
	row, ok := r.rows[id]
	if !ok {
		return nil, ErrSportTypeNotFound
	}

	return &SportType{
		ID:    row.ID,
		Title: row.Title,
	}, nil
}

func (r *memoryRepository) CreateSportType(ctx context.Context, input SportType) (*SportType, error) {
	if err := r.record(ctx); err != nil {
		return nil, err
	}
	if _, exists := r.rows[input.ID]; exists {
		return nil, ErrSportTypeConflict
	}
	r.rows[input.ID] = &input

	return &input, nil
}

func (r *memoryRepository) UpdateSportType(ctx context.Context, id, title string) (*SportType, error) {
	if err := r.record(ctx); err != nil {
		return nil, err
	}
	if _, exists := r.rows[id]; !exists {
		return nil, ErrSportTypeNotFound
	}
	row := &SportType{
		ID:    id,
		Title: title,
	}
	r.rows[id] = row

	return row, nil
}

func (r *memoryRepository) DeleteSportType(ctx context.Context, id string) error {
	if err := r.record(ctx); err != nil {
		return err
	}
	if _, exists := r.rows[id]; !exists {
		return ErrSportTypeNotFound
	}
	if r.inUse[id] {
		return ErrSportTypeInUse
	}
	delete(r.rows, id)

	return nil
}

func TestSportTypeValidationAndNormalization(t *testing.T) {
	tests := []struct {
		name, id, title string
		valid           bool
	}{
		{"ASCII ID", "Badminton_1-all", " Badminton ", true},
		{"ID boundary", strings.Repeat("A", 100), "Sport", true},
		{"Unicode title boundary", "THAI", strings.Repeat("ก", 100), true},
		{"empty ID", "", "Sport", false},
		{"long ID", strings.Repeat("A", 101), "Sport", false},
		{"spaces in ID", " S ", "Sport", false},
		{"Unicode ID", "กีฬา", "Sport", false},
		{"punctuation ID", "sport.type", "Sport", false},
		{"empty title", "S2", "", false},
		{"blank title", "S2", " \t\n ", false},
		{"long title", "S2", strings.Repeat("ก", 101), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newMemoryRepository()
			service := NewService(repo, zap.NewNop())
			row, err := service.CreateSportType(context.Background(), SportType{
				ID:    test.id,
				Title: test.title,
			})
			if !test.valid {
				if !errors.Is(err, ErrInvalidSportType) || repo.calls != 0 {
					t.Fatalf("invalid input reached storage: row=%+v err=%v calls=%d", row, err, repo.calls)
				}

				return
			}
			if err != nil || row.ID != test.id || row.Title != strings.TrimSpace(test.title) {
				t.Fatalf("normalized entry = %+v; err=%v", row, err)
			}
		})
	}
}

func TestSportTypeCRUDAndErrorPropagation(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, err := service.CreateSportType(ctx, SportType{
		ID:    "NEW",
		Title: "Sport",
	}); err != nil {
		t.Fatalf("duplicate titles must be allowed: %v", err)
	}
	if _, err := service.CreateSportType(ctx, SportType{
		ID:    "NEW",
		Title: "Other",
	}); !errors.Is(err, ErrSportTypeConflict) {
		t.Fatalf("duplicate ID error = %v", err)
	}
	row, err := service.UpdateSportType(ctx, "NEW", " Renamed ")
	if err != nil || row.ID != "NEW" || row.Title != "Renamed" || repo.ctx != ctx {
		t.Fatalf("rename/context = %+v %v %v", row, err, repo.ctx)
	}
	if err := service.DeleteSportType(ctx, "USED"); !errors.Is(err, ErrSportTypeInUse) {
		t.Fatalf("referenced delete = %v", err)
	}
	if err := service.DeleteSportType(ctx, "NEW"); err != nil {
		t.Fatal(err)
	}

	operations := []func() error{
		func() error {
			_, err := service.GetSportType(ctx, "NEW")

			return err
		},

		func() error {
			_, err := service.UpdateSportType(ctx, "NEW", "Sport")

			return err
		},
		func() error {
			return service.DeleteSportType(ctx, "NEW")
		},
	}
	for _, operation := range operations {
		if err := operation(); !errors.Is(err, ErrSportTypeNotFound) {
			t.Fatalf("missing entry error = %v", err)
		}
	}

	repo.err = errors.New("storage unavailable")
	operations = append(operations,
		func() error {
			_, err := service.GetAllSportTypes(ctx)

			return err
		},

		func() error {
			_, err := service.CreateSportType(ctx, SportType{
				ID:    "NEW",
				Title: "Sport",
			})

			return err
		},
	)
	for _, operation := range operations {
		if err := operation(); !errors.Is(err, repo.err) || repo.ctx != ctx {
			t.Fatalf("storage error/context lost: %v", err)
		}
	}
}

func TestSportTypeInvalidOperationsDoNotReachStorage(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, nil)
	ctx := context.Background()
	operations := []func() error{
		func() error {
			_, err := service.GetSportType(ctx, "bad.id")

			return err
		},

		func() error {
			_, err := service.UpdateSportType(ctx, "bad.id", "Sport")

			return err
		},

		func() error {
			_, err := service.UpdateSportType(ctx, "S", " ")

			return err
		},
		func() error {
			return service.DeleteSportType(ctx, "bad.id")
		},
	}
	for _, operation := range operations {
		if err := operation(); !errors.Is(err, ErrInvalidSportType) || repo.calls != 0 {
			t.Fatalf("invalid input reached storage: %v calls=%d", err, repo.calls)
		}
	}
}
