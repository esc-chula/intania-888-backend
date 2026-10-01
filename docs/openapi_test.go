package docs

import (
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenAPIDocumentMatchesPublishedRouteInventory(t *testing.T) {
	contents, err := ReadOpenAPI()
	if err != nil {
		t.Fatalf("ReadOpenAPI() error = %v", err)
	}

	var document map[string]any
	if err := yaml.Unmarshal(contents, &document); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if got := document["swagger"]; got != "2.0" {
		t.Fatalf("swagger version = %v, want 2.0", got)
	}

	paths, ok := document["paths"].(map[string]any)
	if !ok {
		t.Fatalf("paths has type %T, want map[string]any", document["paths"])
	}

	wantOperations := map[string]struct{}{}
	for _, operation := range []string{
		"GET /auth/callback",
		"GET /auth/login",
		"GET /auth/authorize",
		"POST /auth/token",
		"POST /auth/revoke",
		"POST /auth/logout",
		"GET /auth/me",
		"GET /external/me",
		"POST /external/deduct-coin",
		"GET /auth/policies",
		"POST /auth/policies",
		"DELETE /auth/policies/{id}",
		"PATCH /auth/policies/{id}",
		"GET /bills",
		"POST /bills",
		"GET /bills/{id}",
		"PUT /bills/admin/{id}/void",
		"GET /bills/admin/all",
		"GET /colors/group-stage",
		"GET /colors/leaderboards",
		"GET /events/daily-rewards",
		"DELETE /events/daily-rewards/{date}",
		"PUT /events/daily-rewards/{date}",
		"GET /events/redeem/daily",
		"POST /events/spin/slot",
		"POST /events/use-steal-token",
		"GET /locations",
		"POST /locations/admin",
		"GET /locations/{id}",
		"PATCH /locations/admin/{id}",
		"DELETE /locations/admin/{id}",
		"GET /matches",
		"POST /matches",
		"DELETE /matches/{id}",
		"GET /matches/{id}",
		"PUT /matches/{id}",
		"PUT /matches/{id}/result",
		"PATCH /matches/{id}/score",
		"GET /matches/current/time",
		"GET /mines/{id}",
		"POST /mines/{id}/cashout",
		"POST /mines/{id}/reveal",
		"GET /mines/active",
		"POST /mines/create",
		"GET /mines/history",
		"GET /mines/stats",
		"GET /sport-types",
		"GET /sport-types/{id}",
		"POST /sport-types/admin",
		"DELETE /sport-types/admin/{id}",
		"PATCH /sport-types/admin/{id}",
		"GET /users",
		"GET /users/{id}",
		"PATCH /users/{id}",
		"PATCH /users/admin/{id}",
		"PATCH /users/me",
	} {
		wantOperations[operation] = struct{}{}
	}

	gotOperations := map[string]struct{}{}
	for path, rawOperations := range paths {
		operations, ok := rawOperations.(map[string]any)
		if !ok {
			t.Fatalf("operations for %q have type %T", path, rawOperations)
		}
		for method, rawOperation := range operations {
			operation, ok := rawOperation.(map[string]any)
			if !ok {
				t.Fatalf("%s %s has type %T", method, path, rawOperation)
			}
			key := strings.ToUpper(method) + " " + path
			gotOperations[key] = struct{}{}
			if strings.TrimSpace(fmt.Sprint(operation["summary"])) == "" ||
				strings.TrimSpace(fmt.Sprint(operation["description"])) == "" {
				t.Errorf("%s has no summary or description", key)
			}
			if _, ok := operation["responses"].(map[string]any); !ok {
				t.Errorf("%s has no response definitions", key)
			}
			parameters, _ := operation["parameters"].([]any)
			for _, rawParameter := range parameters {
				parameter, ok := rawParameter.(map[string]any)
				if !ok {
					t.Errorf("%s has a malformed parameter", key)
					continue
				}
				description, _ := parameter["description"].(string)
				if strings.TrimSpace(description) == "" {
					t.Errorf("%s parameter %q has no description", key, parameter["name"])
				}
				if parameter["in"] == "path" && parameter["required"] != true {
					t.Errorf("%s path parameter %q is not required", key, parameter["name"])
				}
			}

			responses, _ := operation["responses"].(map[string]any)
			for code, rawResponse := range responses {
				response, ok := rawResponse.(map[string]any)
				if !ok {
					t.Errorf("%s response %s is malformed", key, code)
					continue
				}
				headers, _ := response["headers"].(map[string]any)
				if _, ok := headers["X-Request-ID"]; !ok {
					t.Errorf("%s response %s does not document X-Request-ID", key, code)
				}
			}

			if method == "post" || method == "put" || method == "patch" || method == "delete" {
				security, _ := operation["security"].([]any)
				requiresCSRF := key != "POST /auth/logout" && key != "POST /auth/token" && key != "POST /auth/revoke" && !strings.HasPrefix(path, "/external/")
				if len(security) > 0 && requiresCSRF && !hasRequiredHeader(operation, "X-CSRF-Token") {
					t.Errorf("%s does not document its required session CSRF header", key)
				}
			}
		}
	}

	for operation := range wantOperations {
		if _, ok := gotOperations[operation]; !ok {
			t.Errorf("documented operation %q is missing", operation)
		}
	}
	for operation := range gotOperations {
		if _, ok := wantOperations[operation]; !ok {
			t.Errorf("unexpected operation %q", operation)
		}
	}
	if len(gotOperations) != 56 {
		t.Errorf("documented operation count = %d, want 56", len(gotOperations))
	}

	definitions, ok := document["definitions"].(map[string]any)
	if !ok {
		t.Fatalf("definitions has type %T", document["definitions"])
	}
	if err := checkReferences(document, definitions); err != nil {
		t.Error(err)
	}
	if _, ok := definitions["model.LocationDto"]; !ok {
		t.Error("LocationDto definition is missing")
	}
	if createMatch, ok := definitions["model.CreateMatchRequest"].(map[string]any); ok {
		if !hasRequiredField(createMatch, "location_id") {
			t.Error("CreateMatchRequest must require location_id")
		}
	} else {
		t.Error("CreateMatchRequest definition is missing or malformed")
	}
	if match, ok := definitions["model.MatchDto"].(map[string]any); ok {
		properties, _ := match["properties"].(map[string]any)
		location, _ := properties["location"].(map[string]any)
		if location["$ref"] != "#/definitions/model.LocationDto" {
			t.Error("MatchDto.location must reference LocationDto")
		}
	} else {
		t.Error("MatchDto definition is missing or malformed")
	}

	securityDefinitions, _ := document["securityDefinitions"].(map[string]any)
	if _, ok := securityDefinitions["CookieSession"]; !ok {
		t.Error("CookieSession security definition is missing")
	}
	for _, operation := range []string{
		"PATCH /users/{id}",
	} {
		parts := strings.SplitN(operation, " ", 2)
		path, _ := paths[parts[1]].(map[string]any)
		item, _ := path[strings.ToLower(parts[0])].(map[string]any)
		if item["deprecated"] != true {
			t.Errorf("%s must remain marked deprecated", operation)
		}
	}
	matchParameters, _ := paths["/matches"].(map[string]any)
	matchList, _ := matchParameters["get"].(map[string]any)
	if !hasParameter(matchList, "query", "typeId") {
		t.Error("GET /matches must document the typeId query parameter exactly as the handler reads it")
	}

	for _, operation := range []string{
		"GET /locations",
		"GET /locations/{id}",
		"GET /sport-types",
		"GET /sport-types/{id}",
		"GET /matches",
		"GET /matches/{id}",
		"GET /matches/current/time",
		"GET /colors/leaderboards",
		"GET /colors/group-stage",
	} {
		parts := strings.SplitN(operation, " ", 2)
		path, _ := paths[parts[1]].(map[string]any)
		item, _ := path[strings.ToLower(parts[0])].(map[string]any)
		if _, secured := item["security"]; secured {
			t.Errorf("%s must not require CookieSession", operation)
		}
		responses, _ := item["responses"].(map[string]any)
		if _, ok := responses["401"]; ok {
			t.Errorf("%s must not document authentication failure", operation)
		}
		if _, ok := responses["403"]; !ok {
			t.Errorf("%s must document rejection by the configured origin policy", operation)
		}
		if _, ok := responses["429"]; !ok {
			t.Errorf("%s must document the shared IP rate limit", operation)
		}
	}
}

func hasRequiredField(schema map[string]any, name string) bool {
	required, _ := schema["required"].([]any)
	for _, field := range required {
		if field == name {
			return true
		}
	}

	return false
}

func hasRequiredHeader(operation map[string]any, name string) bool {
	parameters, _ := operation["parameters"].([]any)
	for _, rawParameter := range parameters {
		parameter, _ := rawParameter.(map[string]any)
		if parameter["in"] == "header" && parameter["name"] == name && parameter["required"] == true {
			return true
		}
	}

	return false
}

func hasParameter(operation map[string]any, location, name string) bool {
	parameters, _ := operation["parameters"].([]any)
	for _, rawParameter := range parameters {
		parameter, _ := rawParameter.(map[string]any)
		if parameter["in"] == location && parameter["name"] == name {
			return true
		}
	}

	return false
}

func checkReferences(value any, definitions map[string]any) error {
	switch typed := value.(type) {
	case map[string]any:
		if reference, ok := typed["$ref"].(string); ok {
			const prefix = "#/definitions/"
			if !strings.HasPrefix(reference, prefix) {
				return fmt.Errorf("unsupported reference %q", reference)
			}
			name := strings.TrimPrefix(reference, prefix)
			if _, ok := definitions[name]; !ok {
				return fmt.Errorf("unresolved schema reference %q", reference)
			}
		}
		for key, item := range typed {
			if err := checkReferences(item, definitions); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		}
	case []any:
		for index, item := range typed {
			if err := checkReferences(item, definitions); err != nil {
				return fmt.Errorf("item %d: %w", index, err)
			}
		}
	}

	return nil
}
