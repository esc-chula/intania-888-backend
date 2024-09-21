package model

import "time"

type User struct {
	Id        string    `gorm:"primaryKey;type:varchar(100)"`
	Email     string    `gorm:"type:varchar(100);not null"`
	Name      string    `gorm:"type:varchar(100);not null"`
	RoleId    string    `gorm:"type:varchar(100);not null"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Role Role `gorm:"foreignKey:RoleId"`
}

type Role struct {
	ID        string    `gorm:"primaryKey;type:varchar(100)"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Users []User `gorm:"foreignKey:RoleId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
