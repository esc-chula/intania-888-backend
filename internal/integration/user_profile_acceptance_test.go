//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	userdomain "github.com/esc-chula/intania-888-backend/internal/domain/user"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/testutil"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

const profileTestActorID = "profile-self-user"

func TestOwnProfileHTTPEndpointsAgainstPostgres(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedProfileUsers(t, postgres)

	repository := userdomain.NewGORMRepository(postgres.DB)
	service := userdomain.NewService(repository, zap.NewNop())
	app := newProfileTestApp(t, service, profileTestActorID)

	status, body := patchProfileRequest(t, app, "/api/v1/users/me", `{"name":"Updated self"}`)
	if status != http.StatusOK {
		t.Fatalf("PATCH /users/me status = %d, want %d; body=%v", status, http.StatusOK, body)
	}
	if body["name"] != "Updated self" || body["nick_name"] != "Before" ||
		body["group_id"] != "profile-test-group" || body["remaining_coin"] != "123.45" {
		t.Fatalf("omitted fields or account balance changed: body=%v", body)
	}

	status, body = patchProfileRequest(t, app, "/api/v1/users/me", `{"nick_name":null,"group_id":null}`)
	if status != http.StatusOK {
		t.Fatalf("PATCH /users/me null clear status = %d, want %d; body=%v", status, http.StatusOK, body)
	}
	if body["name"] != "Updated self" ||
		body["nick_name"] != nil ||
		body["group_id"] != nil ||
		body["remaining_coin"] != "123.45" {
		t.Fatalf("explicit null fields were not cleared independently: body=%v", body)
	}

	status, body = patchProfileRequest(t, app, "/api/v1/users/"+profileTestActorID, `{"name":"Updated through legacy route"}`)
	if status != http.StatusOK ||
		body["name"] != "Updated through legacy route" ||
		body["remaining_coin"] != "123.45" {
		t.Fatalf("matching-ID legacy route did not delegate to self-profile update: status=%d body=%v", status, body)
	}

	status, body = patchProfileRequest(t, app, "/api/v1/users/profile-other-user", `{"name":"Must be rejected"}`)
	if status != http.StatusForbidden || body["code"] != "FORBIDDEN" {
		t.Fatalf("mismatched-ID legacy route = %d %v, want 403 FORBIDDEN", status, body)
	}

	status, body = patchProfileRequest(t, app, "/api/v1/users/me", `{"remaining_coin":"0.00"}`)
	if status != http.StatusBadRequest || body["code"] != "INVALID_REQUEST" {
		t.Fatalf("balance field update = %d %v, want 400 INVALID_REQUEST", status, body)
	}

	var otherName string
	if err := postgres.SQL.QueryRow(`SELECT name FROM users WHERE id = $1`, "profile-other-user").Scan(&otherName); err != nil {
		t.Fatal(err)
	}
	if otherName != "Other user" {
		t.Fatalf("other user's name = %q, want unchanged", otherName)
	}
}

func TestOwnProfilePatchPreservesInterleavedBalanceChange(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "profile-balance-user", 100_00)

	repository := userdomain.NewGORMRepository(postgres.DB)
	balanceService := userdomain.NewService(repository, zap.NewNop())
	interleavedRepository := profilePatchInterleavingRepository{
		Repository: repository,
		beforePatch: func(ctx context.Context) error {
			_, err := balanceService.DeductCoin(ctx, "profile-balance-user", value.MustMoneyFromMinor(12_34))

			return err
		},
	}
	service := userdomain.NewService(interleavedRepository, zap.NewNop())
	app := newProfileTestApp(t, service, "profile-balance-user")

	status, body := patchProfileRequest(t, app, "/api/v1/users/me", `{"name":"Balance-safe update"}`)
	if status != http.StatusOK {
		t.Fatalf("PATCH /users/me status = %d, want %d; body=%v", status, http.StatusOK, body)
	}
	if body["name"] != "Balance-safe update" || body["remaining_coin"] != "87.66" {
		t.Fatalf("profile update lost the interleaved balance change: body=%v", body)
	}
}

type profilePatchInterleavingRepository struct {
	userdomain.Repository
	beforePatch func(context.Context) error
}

func (r profilePatchInterleavingRepository) PatchProfile(ctx context.Context, actorID string, patch userdomain.ProfilePatch) error {
	if err := r.beforePatch(ctx); err != nil {
		return err
	}

	return r.Repository.PatchProfile(ctx, actorID, patch)
}

func newProfileTestApp(t *testing.T, service userdomain.ServicePort, actorID string) *fiber.App {
	t.Helper()

	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
	authenticate := func(c *fiber.Ctx) error {
		httpidentity.SetProfile(c, &identity.Profile{
			ID:     actorID,
			RoleID: "USER",
		})

		return c.Next()
	}

	userdomain.NewHTTPHandler(service).RegisterRoutes(app.Group("/api/v1"), authenticate, authenticate)

	return app
}

func patchProfileRequest(t *testing.T, app *fiber.App, path, body string) (int, map[string]any) {
	t.Helper()

	request := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
	request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("PATCH %s: %v", path, err)
	}

	responseBody, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		t.Fatalf("read PATCH %s response: %v", path, readErr)
	}
	if closeErr != nil {
		t.Fatalf("close PATCH %s response: %v", path, closeErr)
	}

	var decoded map[string]any
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		t.Fatalf("decode PATCH %s response %q: %v", path, responseBody, err)
	}

	return response.StatusCode, decoded
}

func seedProfileUsers(t *testing.T, postgres *testutil.Postgres) {
	t.Helper()

	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO colors(id, title) VALUES($1, $2)`, []any{"profile-test-color", "Profile test color"}},
		{`INSERT INTO intania_groups(id, color_id) VALUES($1, $2)`, []any{"profile-test-group", "profile-test-color"}},
		{
			`INSERT INTO users(id, email, name, nick_name, role_id, group_id, remaining_coin) VALUES
				($1, $2, $3, $4, 'USER', $5, $6),
				($7, $8, $9, NULL, 'USER', NULL, $10)`,
			[]any{profileTestActorID, "self@example.test", "Self user", "Before", "profile-test-group", 123_45,
				"profile-other-user", "other@example.test", "Other user", 50_00},
		},
	}

	for _, statement := range statements {
		if _, err := postgres.SQL.Exec(statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
}
