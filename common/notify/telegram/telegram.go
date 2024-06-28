package telegram

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labulaka521/crocodile/common/log"
	"github.com/labulaka521/crocodile/common/notify"
	"go.uber.org/zap"
)

var (
	once sync.Once
)

// Telegram conf
type Telegram struct {
	token  string
	client *http.Client
}

// NewTelegram init telegram
func NewTelegram(token string) (notify.Sender, error) {
	telegram := &Telegram{
		token: token,
		client: &http.Client{
			Timeout: time.Second * 10,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		},
	}
	return telegram, nil
}

// Send will send notify to channel
func (t *Telegram) Send(tos []string, title string, content string) error {
	for _, id := range tos {
		uid, err := strconv.Atoi(id)
		if err != nil {
			return err
		}
		go func(uid int) {
			resp, err := t.client.Post("https://api.telegram.org/bot"+t.token+"/sendMessage",
				"application/json", strings.NewReader(fmt.Sprintf(`{
			"chat_id":%d,
			"text":"%v"
		}
		`, uid, title+"\n"+content)))
			if err != nil {
				log.Error("push tg message", zap.Error(err))
				return
			}
			resp.Body.Close()
		}(uid)
	}
	return nil
}
