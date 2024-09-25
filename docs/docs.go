// Package docs Code generated by swaggo/swag. DO NOT EDIT
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/auth/login": {
            "get": {
                "description": "Retrieves the OAuth login URL",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Auth"
                ],
                "summary": "Login URL",
                "responses": {
                    "200": {
                        "description": "url",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "500": {
                        "description": "internal server error",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        },
        "/auth/login/callback": {
            "post": {
                "description": "Verifies the OAuth login and returns credentials",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Auth"
                ],
                "summary": "OAuth Login Callback",
                "parameters": [
                    {
                        "description": "OAuth Code",
                        "name": "oauthCode",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/model.OAuthCodeDto"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "credential",
                        "schema": {
                            "type": "object",
                            "additionalProperties": true
                        }
                    },
                    "400": {
                        "description": "cannot parse body",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "500": {
                        "description": "internal server error",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        },
        "/auth/me": {
            "get": {
                "description": "Retrieves user profile data",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Auth"
                ],
                "summary": "GetMe",
                "responses": {
                    "200": {
                        "description": "profile",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "400": {
                        "description": "bad request error",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        },
        "/auth/refresh": {
            "post": {
                "description": "Refreshes the access token using the refresh token",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Auth"
                ],
                "summary": "Refresh Token",
                "parameters": [
                    {
                        "description": "Refresh Token",
                        "name": "refreshToken",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/model.RefreshTokenDto"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "credential",
                        "schema": {
                            "type": "object",
                            "additionalProperties": true
                        }
                    },
                    "400": {
                        "description": "cannot parse body",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "500": {
                        "description": "internal server error",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        },
        "/bills": {
            "get": {
                "description": "Get all bills",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Bill"
                ],
                "summary": "Get all bills",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/model.BillHeadDto"
                            }
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/bill.ErrorResponse"
                        }
                    }
                }
            },
            "post": {
                "description": "Create a new bill with the input payload",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Bill"
                ],
                "summary": "Create a new bill",
                "parameters": [
                    {
                        "description": "Create bill",
                        "name": "bill",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/model.BillHeadDto"
                        }
                    }
                ],
                "responses": {
                    "201": {
                        "description": "Created",
                        "schema": {
                            "$ref": "#/definitions/model.BillHeadDto"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/bill.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/bill.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/bills/{id}": {
            "get": {
                "description": "Get a bill by its ID",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Bill"
                ],
                "summary": "Get a bill by ID",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Bill ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/model.BillHeadDto"
                        }
                    },
                    "404": {
                        "description": "Not Found",
                        "schema": {
                            "$ref": "#/definitions/bill.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/bill.ErrorResponse"
                        }
                    }
                }
            },
            "put": {
                "description": "Update a bill with the input payload",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Bill"
                ],
                "summary": "Update a bill",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Bill ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "description": "Update bill",
                        "name": "bill",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/model.BillHeadDto"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/model.BillHeadDto"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/bill.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "Not Found",
                        "schema": {
                            "$ref": "#/definitions/bill.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/bill.ErrorResponse"
                        }
                    }
                }
            },
            "delete": {
                "description": "Delete a bill by its ID",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Bill"
                ],
                "summary": "Delete a bill",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Bill ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "204": {
                        "description": "No Content"
                    },
                    "404": {
                        "description": "Not Found",
                        "schema": {
                            "$ref": "#/definitions/bill.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/bill.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/matches": {
            "get": {
                "description": "Retrieves a list of matches, optionally filtered by type and schedule",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Match"
                ],
                "summary": "Retrieves a list of matches, optionally filtered by type and schedule",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Filter by sport type ID",
                        "name": "typeId",
                        "in": "query"
                    },
                    {
                        "type": "string",
                        "description": "Filter by schedule (schedule or result)",
                        "name": "schedule",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "List of matches grouped by date and sport type",
                        "schema": {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/model.MatchesByDate"
                            }
                        }
                    },
                    "400": {
                        "description": "Invalid schedule parameter",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "500": {
                        "description": "Failed to fetch matches",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            },
            "post": {
                "description": "Creates a new match and stores it in the system",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Match"
                ],
                "summary": "Creates a new match",
                "parameters": [
                    {
                        "description": "Match information",
                        "name": "match",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/model.MatchDto"
                        }
                    }
                ],
                "responses": {
                    "201": {
                        "description": "Created match successful",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "400": {
                        "description": "Invalid request payload",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "500": {
                        "description": "Failed to create match",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        },
        "/matches/{id}": {
            "get": {
                "description": "Retrieves a single match by its ID",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Match"
                ],
                "summary": "Retrieves a single match by its ID",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Match ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/model.MatchDto"
                        }
                    },
                    "404": {
                        "description": "Match not found",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            },
            "delete": {
                "description": "Deletes a match by its ID",
                "tags": [
                    "Match"
                ],
                "summary": "Deletes a match by its ID",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Match ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "Deleted match successful",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "500": {
                        "description": "Failed to delete match",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        },
        "/matches/{id}/score": {
            "patch": {
                "description": "Updates the score of a match",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Match"
                ],
                "summary": "Updates the score of a match",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Match ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "description": "Score information",
                        "name": "score",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/model.ScoreDto"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "Updated match score successfully",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "400": {
                        "description": "Invalid request payload",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "500": {
                        "description": "Failed to update match score",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        },
        "/matches/{id}/winner/{winner_id}": {
            "patch": {
                "description": "Updates the winner of a match",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Match"
                ],
                "summary": "Updates the winner of a match",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Match ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "description": "Winner Team ID",
                        "name": "winner_id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "Updated match winner successfully",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "500": {
                        "description": "Failed to update match winner",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        },
        "/users": {
            "get": {
                "description": "Retrieves a list of all users",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "User"
                ],
                "summary": "Get all users",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/model.UserDto"
                            }
                        }
                    },
                    "500": {
                        "description": "internal server error",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            },
            "post": {
                "description": "Creates a new user and stores it in the system",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "User"
                ],
                "summary": "Create a new user",
                "parameters": [
                    {
                        "description": "User information",
                        "name": "user",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/model.UserDto"
                        }
                    }
                ],
                "responses": {
                    "201": {
                        "description": "Created",
                        "schema": {
                            "$ref": "#/definitions/model.UserDto"
                        }
                    },
                    "400": {
                        "description": "cannot parse body",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "500": {
                        "description": "internal server error",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        },
        "/users/{id}": {
            "get": {
                "description": "Retrieves a single user by their ID",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "User"
                ],
                "summary": "Get user by ID",
                "parameters": [
                    {
                        "type": "string",
                        "description": "User ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/model.UserDto"
                        }
                    },
                    "404": {
                        "description": "user not found",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            },
            "delete": {
                "description": "Deletes a user by their ID",
                "tags": [
                    "User"
                ],
                "summary": "Delete user",
                "parameters": [
                    {
                        "type": "string",
                        "description": "User ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "204": {
                        "description": "No Content"
                    },
                    "500": {
                        "description": "internal server error",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            },
            "patch": {
                "description": "Updates an existing user",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "User"
                ],
                "summary": "Update user",
                "parameters": [
                    {
                        "type": "string",
                        "description": "User ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "description": "Updated user information",
                        "name": "user",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/model.UserDto"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/model.UserDto"
                        }
                    },
                    "400": {
                        "description": "cannot parse body",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "500": {
                        "description": "internal server error",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "bill.ErrorResponse": {
            "type": "object",
            "properties": {
                "message": {
                    "type": "string"
                }
            }
        },
        "model.BillHeadDto": {
            "type": "object",
            "properties": {
                "id": {
                    "type": "string"
                },
                "lines": {
                    "description": "Nested BillLine DTO",
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/model.BillLineDto"
                    }
                },
                "total": {
                    "type": "number"
                },
                "user_id": {
                    "type": "string"
                }
            }
        },
        "model.BillLineDto": {
            "type": "object",
            "properties": {
                "betting_on": {
                    "type": "string"
                },
                "bill_id": {
                    "type": "string"
                },
                "match": {
                    "$ref": "#/definitions/model.MatchDto"
                },
                "match_id": {
                    "type": "string"
                },
                "rate": {
                    "type": "number"
                }
            }
        },
        "model.MatchDto": {
            "type": "object",
            "properties": {
                "end_time": {
                    "type": "string"
                },
                "id": {
                    "type": "string"
                },
                "start_time": {
                    "type": "string"
                },
                "team_a": {
                    "type": "string"
                },
                "team_a_score": {
                    "type": "integer"
                },
                "team_b": {
                    "type": "string"
                },
                "team_b_score": {
                    "type": "integer"
                },
                "type": {
                    "type": "string"
                },
                "winner": {
                    "type": "string"
                }
            }
        },
        "model.MatchesByDate": {
            "type": "object",
            "properties": {
                "date": {
                    "type": "string"
                },
                "types": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/model.MatchesByType"
                    }
                }
            }
        },
        "model.MatchesByType": {
            "type": "object",
            "properties": {
                "matches": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/model.MatchDto"
                    }
                },
                "sportType": {
                    "type": "string"
                }
            }
        },
        "model.OAuthCodeDto": {
            "type": "object",
            "properties": {
                "code": {
                    "type": "string"
                }
            }
        },
        "model.RefreshTokenDto": {
            "type": "object",
            "properties": {
                "refresh_token": {
                    "type": "string"
                }
            }
        },
        "model.ScoreDto": {
            "type": "object",
            "properties": {
                "teamAScore": {
                    "type": "integer"
                },
                "teamBScore": {
                    "type": "integer"
                }
            }
        },
        "model.UserDto": {
            "type": "object",
            "properties": {
                "email": {
                    "type": "string"
                },
                "group_id": {
                    "type": "string"
                },
                "id": {
                    "type": "string"
                },
                "name": {
                    "type": "string"
                },
                "remaining_coin": {
                    "type": "number"
                },
                "role_id": {
                    "type": "string"
                }
            }
        }
    },
    "securityDefinitions": {
        "BearerAuth": {
            "description": "Type \"Bearer\" followed by a space and the token",
            "type": "apiKey",
            "name": "Authorization",
            "in": "header"
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "0.0.0",
	Host:             "localhost:8080",
	BasePath:         "/api/v1",
	Schemes:          []string{},
	Title:            "Intania888 Backend - API",
	Description:      "This is an Intania888 Backend API in Intania888 project.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
