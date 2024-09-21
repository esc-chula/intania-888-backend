package model

import "time"

type UserDto struct {
	Id        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	RoleId    string    `json:"role_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CredentialDto struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int32  `json:"expires_in"`
}

type OAuthCodeDto struct {
	Code string `json:"code"`
}

type RefreshTokenDto struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshCacheDto struct {
	UserId string
	Role   string
}
