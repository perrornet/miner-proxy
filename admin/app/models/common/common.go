package common

import (
	jsoniter "encoding/json"
	"fmt"
	"miner-proxy/admin/app/database"
)

// Create
func Create(value interface{}) error {

	return database.GetDb().Create(value).Error
}

// Save
func Save(value interface{}) error {
	return database.GetDb().Save(value).Error
}

// Updates
func Updates(where interface{}, value interface{}) error {
	return database.GetDb().Model(where).Updates(value).Error
}

// Delete
func DeleteByModel(model interface{}) (count int64, err error) {
	db := database.GetDb().Delete(model)
	err = db.Error
	if err != nil {
		return
	}
	count = db.RowsAffected
	return
}

// Delete
func DeleteByWhere(model, where interface{}) (count int64, err error) {
	db := database.GetDb().Where(where).Delete(model)
	err = db.Error
	if err != nil {
		return
	}
	count = db.RowsAffected
	return
}

// Delete
func DeleteByID(model interface{}, id uint64) (count int64, err error) {
	db := database.GetDb().Where("id=?", id).Delete(model)
	err = db.Error
	if err != nil {
		return
	}
	count = db.RowsAffected
	return
}

// Delete
func DeleteByIDS(model interface{}, ids interface{}) (count int64, err error) {
	db := database.GetDb().Where("id in (?)", ids).Delete(model)
	err = db.Error
	if err != nil {
		return
	}
	count = db.RowsAffected
	return
}

func DeleteByFields(model interface{}, field string, value interface{}) (count int64, err error) {
	db := database.GetDb().Where(fmt.Sprintf("%s in (?)", field), value).Delete(model)
	err = db.Error
	if err != nil {
		return
	}
	count = db.RowsAffected
	return
}

// First
func FirstByID(out interface{}, id interface{}) error {
	return database.GetDb().First(out, id).Error
}

// First
func First(where interface{}, out interface{}) error {
	return database.GetDb().Where(where).First(out).Error
}

func ToJsonRemove(data interface{}, remove []string) (map[string]interface{}, error) {
	var result = make(map[string]interface{})
	temp, _ := jsoniter.Marshal(data)
	if err := jsoniter.Unmarshal(temp, &result); err != nil {
		return nil, err
	}
	for _, v := range remove {
		if _, ok := result[v]; ok {
			delete(result, v)
		}
	}
	return result, nil
}

// Find
func Find(where interface{}, out interface{}, orders ...string) error {
	db := database.GetDb().Where(where)
	if len(orders) > 0 {
		for _, order := range orders {
			db = db.Order(order)
		}
	}
	return db.Find(out).Error
}

func FindBySelect(where interface{}, out interface{}, selectFiled string, orders ...string) error {
	db := database.GetDb().Where(where)
	if len(orders) > 0 {
		for _, order := range orders {
			db = db.Order(order)
		}
	}
	return db.Select(selectFiled).Find(out).Error
}

// Find
func FindById(dest interface{}, id interface{}) error {
	return database.GetDb().Where("id = ?", id).Find(dest).Error
}

// Finds
func FindByIds(dest interface{}, ids ...interface{}) error {
	return database.GetDb().Where("id IN (?)", ids).Find(dest).Error
}

func FindInField(dest interface{}, field string, values ...interface{}) error {
	return database.GetDb().Where(fmt.Sprintf("%s IN (?)", field), values).Find(dest).Error
}

// Scan
func Scan(model, where interface{}, out interface{}) error {
	return database.GetDb().Model(model).Where(where).Scan(out).Error
}

// ScanList
func ScanList(model, where interface{}, out interface{}, orders ...string) error {
	db := database.GetDb().Model(model).Where(where)
	if len(orders) > 0 {
		for _, order := range orders {
			db = db.Order(order)
		}
	}
	return db.Scan(out).Error
}

// GetPage
func GetPage(model, where interface{}, out interface{}, pageIndex, pageSize int, totalCount *int64, whereOrder ...PageWhereOrder) error {
	if pageIndex == 0 {
		pageIndex = 1
	}
	db := database.GetDb().Model(model).Where(where)
	var hasSort bool
	if len(whereOrder) > 0 {
		for _, wo := range whereOrder {
			if wo.Order != "" {
				hasSort = true
				db = db.Order(wo.Order)
			}
			if wo.Where != "" {
				db = db.Where(wo.Where, wo.Value...)
			}
		}
	}
	if !hasSort {
		db = db.Order("id DESC")
	}
	err := db.Count(totalCount).Error
	if err != nil {
		return fmt.Errorf("count error: %v", err)
	}
	if *totalCount == 0 {
		return nil
	}
	if err := db.Offset((pageIndex - 1) * pageSize).Limit(pageSize).Find(out).Error; err != nil {
		return fmt.Errorf("get result error: %v", err)
	}
	return nil
}

type Count struct {
	Count uint64 `json:"count"`
}

// PluckList
func PluckList(model, where interface{}, out interface{}, fieldName string) error {
	return database.GetDb().Model(model).Where(where).Pluck(fieldName, out).Error
}
