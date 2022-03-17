package models

import (
	"fmt"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

func InitModels(db *gorm.DB) error {
	_ = db.AutoMigrate(
		new(Client),
		new(Forward),
		new(User),
	)
	// 检查是否存在超级用户
	var user User
	db.Where("name = 'admin'").First(&user)
	if user.ID == 0 {
		// 生成password
		pass := GeneratePassword()
		if err := db.Create(&User{
			Pass:        Password(pass),
			Name:        "admin",
			IsSuperUser: true,
			IsActive:    true,
		}).Error; err != nil {
			return errors.Wrap(err, "初始化admin用户失败")
		}
		fmt.Printf("您的用户名为:admin\n密码为:%s\n", pass)
	}
	return nil
}
