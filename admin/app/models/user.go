package models

import (
	"crypto/md5"
	"fmt"
	"strings"

	"github.com/sethvargo/go-password/password"
	"gorm.io/gorm"
)

type Password string

func (p Password) Encryption() Password {
	if strings.HasPrefix(string(p), "miner-proxy-Encryption") {
		return p
	}
	return Password(fmt.Sprintf("miner-proxy-Encryption%x", md5.Sum([]byte(fmt.Sprintf("%sminer-proxy%s-pass", p, p)))))
}

func (p Password) MarshalJSON() (data []byte, err error) {
	return []byte(`""`), nil
}

type User struct {
	gorm.Model
	Pass        Password `json:"pass"`
	Name        string   `json:"name" gorm:"uniqueIndex:port_index"`
	IsSuperUser bool     `json:"is_super_user"`
	IsActive    bool     `json:"is_active"`
}

func (u User) TableName() string {
	return "sys_user"
}

func (u *User) EncryptionPassword(pass string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%sminer-proxy%s-pass", u.Pass, u.Pass))))
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	u.IsActive = true
	u.Pass = u.Pass.Encryption()
	println(u.Pass)
	return
}

func (u *User) BeforeUpdate(tx *gorm.DB) (err error) {
	if u.Pass != "" {
		u.Pass = u.Pass.Encryption()
	}
	return
}

func GeneratePassword() string {
	pass, _ := password.Generate(32, 8, 8, false, false)
	return pass
}
