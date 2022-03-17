package database

import (
	"fmt"
	"time"

	sqliteEncrypt "github.com/jackfr0st13/gorm-sqlite-cipher"
	"github.com/patrickmn/go-cache"
	"gorm.io/gorm"
)

var (
	db    *gorm.DB
	Cache = cache.New(time.Minute, time.Second*10)
)

func GetDb() *gorm.DB {
	return db
}

func InitDb(dbPath, key string) (err error) {
	dbnameWithDSN := dbPath + fmt.Sprintf("?_pragma_key=%s&_pragma_cipher_page_size=4096", key)
	// Logger: logger.Discard
	db, err = gorm.Open(sqliteEncrypt.Open(dbnameWithDSN), &gorm.Config{})
	return
}
