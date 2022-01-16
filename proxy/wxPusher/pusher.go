package wxPusher

import (
	"github.com/wxpusher/wxpusher-sdk-go"
	"github.com/wxpusher/wxpusher-sdk-go/model"
)

type WxPusher struct {
	ApiKey string `json:"api_key"`
}

func NewPusher(apiKey string) *WxPusher {
	return &WxPusher{ApiKey: apiKey}
}

func (w *WxPusher) SendMessage(text string, uid ...string) error {
	m := model.NewMessage(w.ApiKey)
	m.UIds = uid
	_, err := wxpusher.SendMessage(m.SetContent(text))
	return err
}

func (w *WxPusher) ShowQrCode() (string, error) {
	resp, err := wxpusher.CreateQrcode(&model.Qrcode{
		AppToken: w.ApiKey,
		Extra:    "miner-proxy",
	})
	return resp.Url, err
}

func (w *WxPusher) GetAllUser() ([]model.WxUser, error) {
	var (
		page  = 1
		limit = 20
	)
	var result []model.WxUser
	for {
		resp, err := wxpusher.QueryWxUser(w.ApiKey, page, limit)
		if err != nil {
			return nil, err
		}
		if len(resp.Records) == 0 {
			break
		}
		result = append(result, resp.Records...)
		page++
	}
	return result, nil
}

func (w *WxPusher) GetToken() string {
	return w.ApiKey
}
