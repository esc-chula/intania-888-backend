package location

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

type memoryRepository struct {
	rows       map[string]Location
	references map[string]bool
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		rows: map[string]Location{"USED": {
			ID:    "USED",
			Title: "Used venue",
		}},
		references: map[string]bool{"USED": true},
	}
}

func (r *memoryRepository) GetAllLocations(context.Context) ([]*Location, error) {
	rows := make([]*Location, 0, len(r.rows))
	for _, row := range r.rows {
		copy := row
		rows = append(rows, &copy)
	}

	return rows, nil
}

func (r *memoryRepository) GetLocation(_ context.Context, id string) (*Location, error) {
	row, ok := r.rows[id]
	if !ok {
		return nil, ErrLocationNotFound
	}

	return &row, nil
}

func (r *memoryRepository) CreateLocation(_ context.Context, input Location) (*Location, error) {
	if _, ok := r.rows[input.ID]; ok {
		return nil, ErrLocationConflict
	}
	r.rows[input.ID] = input

	return &input, nil
}

func (r *memoryRepository) UpdateLocation(_ context.Context, id, title string) (*Location, error) {
	row, ok := r.rows[id]
	if !ok {
		return nil, ErrLocationNotFound
	}
	row.Title = title
	r.rows[id] = row

	return &row, nil
}

func (r *memoryRepository) DeleteLocation(_ context.Context, id string) error {
	if _, ok := r.rows[id]; !ok {
		return ErrLocationNotFound
	}
	if r.references[id] {
		return ErrLocationInUse
	}
	delete(r.rows, id)

	return nil
}

func locationTestApp(repo *memoryRepository) *fiber.App {
	return locationTestAppWithGuards(
		repo,
		func(c *fiber.Ctx) error {
			return c.Next()
		},
		func(c *fiber.Ctx) error {
			return c.Next()
		},
	)
}

func locationTestAppWithAuth(repo *memoryRepository, authenticate fiber.Handler) *fiber.App {
	return locationTestAppWithGuards(repo, authenticate, func(c *fiber.Ctx) error {
		return c.Next()
	})
}

func locationTestAppWithGuards(repo *memoryRepository, authenticate, admin fiber.Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
	NewHTTPHandler(NewService(repo, nil)).RegisterRoutes(app, authenticate, admin)

	return app
}

func locationRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	return request
}

func locationResponse(t *testing.T, app *fiber.App, request *http.Request) (int, []byte) {
	t.Helper()

	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	return response.StatusCode, body
}

func TestLocationHTTPCRUDAndReferencedDeletion(t *testing.T) {
	repo := newMemoryRepository()
	app := locationTestApp(repo)
	tests := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{"POST", "/locations/admin", `{"id":"new_ID-1","title":" Venue "}`, 201, `{"id":"new_ID-1","title":"Venue"}`},
		{"GET", "/locations/new_ID-1", "", 200, `{"id":"new_ID-1","title":"Venue"}`},
		{"PATCH", "/locations/admin/new_ID-1", `{"title":" New name "}`, 200, `{"id":"new_ID-1","title":"New name"}`},
		{"DELETE", "/locations/admin/new_ID-1", "", 204, ""},
		{"DELETE", "/locations/admin/USED", "", 409, "LOCATION_IN_USE"},
		{"GET", "/locations/MISSING", "", 404, "RESOURCE_NOT_FOUND"},
	}
	for _, test := range tests {
		t.Run(test.method+test.path, func(t *testing.T) {
			status, body := locationResponse(t, app, locationRequest(test.method, test.path, test.body))
			if status != test.status {
				t.Fatalf("status = %d; want %d; body=%s", status, test.status, body)
			}
			if status < 400 {
				if string(body) != test.want {
					t.Fatalf("body = %s; want %s", body, test.want)
				}

				return
			}

			var failure apierror.Response
			if err := json.Unmarshal(body, &failure); err != nil {
				t.Fatal(err)
			}
			if failure.Code != test.want || failure.RequestID == "" {
				t.Fatalf("failure = %+v", failure)
			}
		})
	}
}

func TestLocationReadsArePublicAndMutationsStillAuthenticate(t *testing.T) {
	repo := newMemoryRepository()
	app := locationTestAppWithAuth(repo, func(c *fiber.Ctx) error {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	})

	for _, path := range []string{"/locations", "/locations/USED"} {
		status, body := locationResponse(t, app, locationRequest(http.MethodGet, path, ""))
		if status != fiber.StatusOK {
			t.Fatalf("public GET %s = %d %s", path, status, body)
		}
	}

	status, body := locationResponse(t, app, locationRequest(http.MethodPost, "/locations/admin", `{"id":"new_ID-1","title":"Venue"}`))
	if status != fiber.StatusUnauthorized || !strings.Contains(string(body), `"code":"UNAUTHORIZED"`) {
		t.Fatalf("protected POST = %d %s; want authentication failure", status, body)
	}

	app = locationTestAppWithGuards(
		repo,
		func(c *fiber.Ctx) error {
			return c.Next()
		},
		func(c *fiber.Ctx) error {
			return apierror.New(fiber.StatusForbidden, "FORBIDDEN", "Administrator access required")
		},
	)
	status, body = locationResponse(t, app, locationRequest(http.MethodPost, "/locations/admin", `{"id":"new_ID-1","title":"Venue"}`))
	if status != fiber.StatusForbidden || !strings.Contains(string(body), `"code":"FORBIDDEN"`) {
		t.Fatalf("admin-protected POST = %d %s; want administrator failure", status, body)
	}
}

func TestLocationHTTPRejectsInvalidRequests(t *testing.T) {
	for _, test := range []struct{ method, path, body string }{
		{"POST", "/locations/admin", `{}`},
		{"POST", "/locations/admin", `{"id":"กีฬา","title":"Venue"}`},
		{"POST", "/locations/admin", `{"id":" S ","title":"Valid title"}`},
		{"POST", "/locations/admin", `{"id":"a/b","title":"Valid title"}`},
		{"POST", "/locations/admin", `{"id":"a?b","title":"Valid title"}`},
		{"POST", "/locations/admin", `{"id":"a%b","title":"Valid title"}`},
		{"POST", "/locations/admin", `{"id":"a.b","title":"Valid title"}`},
		{"POST", "/locations/admin", `{"id":"a:b","title":"Valid title"}`},
		{"POST", "/locations/admin", `{"id":"a\u0000b","title":"Valid title"}`},
		{"POST", "/locations/admin", `{"id":"a\tb","title":"Valid title"}`},
		{"POST", "/locations/admin", `{"id":"a\u007fb","title":"Valid title"}`},
		{"POST", "/locations/admin", `{"id":"new_ID-1","title":"  "}`},
		{"PATCH", "/locations/admin/USED", `{}`},
		{"PATCH", "/locations/admin/USED", `{"title":"` + strings.Repeat("ก", 101) + `"}`},
		{"GET", "/locations/กีฬา", ""},
		{"GET", "/locations/%20S%20", ""},
		{"PATCH", "/locations/admin/%20S%20", `{"title":"Valid title"}`},
		{"DELETE", "/locations/admin/%20S%20", ""},
		{"GET", "/locations/a%2Fb", ""},
		{"PATCH", "/locations/admin/a%2Fb", `{"title":"Valid title"}`},
		{"DELETE", "/locations/admin/a%2Fb", ""},
		{"GET", "/locations/a%3Fb", ""},
		{"PATCH", "/locations/admin/a%3Fb", `{"title":"Valid title"}`},
		{"DELETE", "/locations/admin/a%3Fb", ""},
		{"GET", "/locations/a%25b", ""},
		{"PATCH", "/locations/admin/a%25b", `{"title":"Valid title"}`},
		{"DELETE", "/locations/admin/a%25b", ""},
		{"GET", "/locations/a%00b", ""},
		{"PATCH", "/locations/admin/a%00b", `{"title":"Valid title"}`},
		{"DELETE", "/locations/admin/a%00b", ""},
	} {
		t.Run(test.method+test.path+test.body, func(t *testing.T) {
			status, body := locationResponse(t, locationTestApp(newMemoryRepository()), locationRequest(test.method, test.path, test.body))
			if status != fiber.StatusBadRequest || !strings.Contains(string(body), `"code":"INVALID_REQUEST"`) {
				t.Fatalf("invalid request = %d %s", status, body)
			}
		})
	}
}
