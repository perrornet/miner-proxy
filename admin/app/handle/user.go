package handle

import (
	"miner-proxy/admin/app/models"
	"miner-proxy/admin/app/models/common"
	"miner-proxy/admin/app/response"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ListUser(c *gin.Context) {
	user := GetUser(c)
	pageIndex, pageSize := GetPage(c)
	if !user.IsSuperUser {
		response.PageOK(c, nil, 0, pageIndex, pageSize, "ok")
		return
	}
	var result []models.User
	var total int64
	if err := common.GetPage(new(models.User), new(models.User), &result, pageIndex, pageSize, &total); err != nil {
		response.Error(c, http.StatusInternalServerError, err, "server error")
		return
	}
	response.PageOK(c, result, total, pageIndex, pageSize, "ok")
	return
}

func DeleteUser(c *gin.Context) {
	user := GetUser(c)
	if !user.IsSuperUser {
		response.OK(c, nil, "ok")
		return
	}
	uId := c.GetInt("id")
	if uId <= 0 {
		response.OK(c, nil, "ok")
		return
	}
	if _, err := common.DeleteByID(new(models.User), uint64(uId)); err != nil {
		response.Error(c, http.StatusInternalServerError, err, "server error")
		return
	}
	response.OK(c, nil, "ok")
	return
}

func UpdateUser(c *gin.Context) {
	user := GetUser(c)
	var updateUser = new(models.User)
	if err := c.Bind(updateUser); err != nil {
		response.Error(c, http.StatusBadRequest, nil, "params error")
		return
	}

	var existUser models.User
	if err := common.FindById(&existUser, updateUser.ID); err != nil {
		response.Error(c, http.StatusBadRequest, err, "")
		return
	}

	if existUser.ID != user.ID && !user.IsSuperUser {
		response.OK(c, nil, "ok")
		return
	}

	if !user.IsSuperUser {
		updateUser.IsSuperUser = false
	}

	if err := common.Updates(&models.User{Model: gorm.Model{ID: updateUser.ID}}, updateUser); err != nil {
		response.Error(c, http.StatusBadRequest, err, "")
		return
	}
	response.OK(c, nil, "ok")
	return
}

func CreateUser(c *gin.Context) {
	user := GetUser(c)
	if !user.IsSuperUser {
		response.OK(c, nil, "ok")
		return
	}
	var createUser = new(models.User)
	if err := c.Bind(createUser); err != nil {
		response.Error(c, http.StatusBadRequest, nil, "params error")
		return
	}
	var existUser models.User
	if err := common.First(&models.User{Name: createUser.Name}, &existUser); err == nil {
		response.Error(c, http.StatusBadRequest, nil, "用户名已经存在")
		return
	}
	createUser.IsSuperUser = false

	if err := common.Create(createUser); err != nil {
		response.Error(c, http.StatusBadRequest, err, "")
		return
	}
	response.OK(c, nil, "ok")
	return
}
