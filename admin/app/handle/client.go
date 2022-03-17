package handle

import (
	"miner-proxy/admin/app/database"
	"miner-proxy/admin/app/models"
	"miner-proxy/admin/app/models/common"
	"miner-proxy/admin/app/response"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

func ListClient(c *gin.Context) {
	user := GetUser(c)
	pageIndex, pageSize := GetPage(c)
	var (
		result []*models.Client
		total  int64
		wheres []common.PageWhereOrder
	)

	if !user.IsSuperUser {
		wheres = append(wheres, common.PageWhereOrder{
			Where: "created_by = ?",
			Value: []interface{}{user.ID},
		})
	}

	if id := cast.ToInt64(c.Query("id")); id != 0 {
		wheres = append(wheres, common.PageWhereOrder{
			Where: "id = ?",
			Value: []interface{}{id},
		})
	}

	if err := common.GetPage(new(models.Client), new(models.Client),
		&result, pageIndex, pageSize, &total, wheres...); err != nil {
		response.Error(c, http.StatusInternalServerError, err, "server error")
		return
	}
	for index, v := range result {
		if _, ok := database.Cache.Get(v.OnlineKey()); ok {
			result[index].IsOnline = true
		}

		for fIndex, f := range result[index].Forwards {

			if _, ok := database.Cache.Get(f.OnlineKey()); ok {
				(&result[index].Forwards[fIndex]).IsOnline = true
			}

			if v, ok := database.Cache.Get(f.ConnSizeKey()); ok {
				(&result[index].Forwards[fIndex]).ConnSize = cast.ToInt64(v)
			}

			if v, ok := database.Cache.Get(f.DataSizeKey()); ok {
				(&result[index].Forwards[fIndex]).DataSize = cast.ToInt64(v)
			}
		}

	}
	response.PageOK(c, result, total, pageIndex, pageSize, "ok")
}

func UpdateClient(c *gin.Context) {
	user := GetUser(c)

	var client models.Client
	if err := c.Bind(&client); err != nil {
		response.Error(c, http.StatusBadRequest, nil, "params error")
		return
	}
	var existClient models.Client
	if err := common.FindById(&existClient, client.ID); err != nil {
		response.Error(c, http.StatusBadRequest, err, "")
		return
	}

	if !user.IsSuperUser && existClient.CreatedBy != user.ID {
		response.OK(c, nil, "ok")
		return
	}

	if err := common.Updates(&models.Client{Model: gorm.Model{ID: client.ID}}, client); err != nil {
		response.Error(c, http.StatusBadRequest, err, "")
		return
	}
	response.OK(c, nil, "ok")
}

func DeleteClient(c *gin.Context) {
	user := GetUser(c)

	clientId := c.GetInt("id")
	if clientId <= 0 {
		response.OK(c, nil, "ok")
		return
	}
	var existClient models.Client
	if err := common.FindById(&existClient, uint64(clientId)); err != nil {
		response.OK(c, nil, "ok")
		return
	}
	if existClient.CreatedBy != user.ID && !user.IsSuperUser {
		response.OK(c, nil, "ok")
		return
	}
	response.OK(c, nil, "ok")
	return
}

func CreateClient(c *gin.Context) {
	user := GetUser(c)

	var client models.Client
	if err := c.Bind(&client); err != nil {
		response.Error(c, http.StatusBadRequest, nil, "params error")
		return
	}
	client.CreatedBy = user.ID
	if err := common.Create(client); err != nil {
		response.Error(c, http.StatusBadRequest, nil, "已经存在一个该端口或者其他错误")
		return
	}
	response.OK(c, nil, "ok")
	return
}

func CreateForward(c *gin.Context) {
	var forward models.Forward
	if err := c.Bind(&forward); err != nil {
		response.Error(c, http.StatusBadRequest, nil, "params error")
		return
	}
	user := GetUser(c)
	var client models.Client
	if err := common.FindById(&client, forward.ClientId); err != nil {
		response.Error(c, http.StatusBadRequest, err, "")
		return
	}

	if !user.IsSuperUser && user.ID != client.CreatedBy {
		response.Error(c, http.StatusBadRequest, nil, "")
		return
	}

	for _, v := range client.Forwards {
		if v.ClientPort == forward.ClientPort || v.ServerPort == forward.ServerPort {
			response.Error(c, http.StatusBadRequest, nil, "服务端/客户端监听端口重复!")
			return
		}
	}
	if err := common.Create(forward); err != nil {
		response.Error(c, http.StatusInternalServerError, nil, "")
		return
	}
	response.OK(c, nil, "ok")
}

func UpdateForward(c *gin.Context) {
	var forward models.Forward
	if err := c.Bind(&forward); err != nil {
		response.Error(c, http.StatusBadRequest, nil, "params error")
		return
	}
	user := GetUser(c)
	var existForward models.Forward
	if err := common.FindById(&existForward, forward.ID); err != nil {
		response.Error(c, http.StatusBadRequest, err, "")
		return
	}

	var client models.Client
	if err := common.FindById(&client, existForward.ClientId); err != nil {
		response.Error(c, http.StatusBadRequest, err, "")
		return
	}

	if !user.IsSuperUser && user.ID != client.CreatedBy {
		response.Error(c, http.StatusBadRequest, nil, "")
		return
	}

	forward.ClientId = existForward.ClientId
	if err := common.Updates(&models.Forward{Model: gorm.Model{ID: forward.ID}}, forward); err != nil {
		response.Error(c, http.StatusInternalServerError, nil, "")
		return
	}
	response.OK(c, nil, "ok")
}

func DeleteForward(c *gin.Context) {
	user := GetUser(c)

	id := c.GetInt("id")
	if id <= 0 {
		response.OK(c, nil, "ok")
		return
	}
	var existForward models.Forward
	if err := common.FindById(&existForward, uint64(id)); err != nil {
		response.OK(c, nil, "ok")
		return
	}
	var client models.Client
	if err := common.FindById(&client, existForward.ClientId); err != nil {
		response.Error(c, http.StatusBadRequest, err, "")
		return
	}

	if !user.IsSuperUser && user.ID != client.CreatedBy {
		response.Error(c, http.StatusBadRequest, nil, "")
		return
	}
	response.OK(c, nil, "ok")
	return
}
