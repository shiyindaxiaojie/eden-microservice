package notify

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
)

// Message represents the notification content
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Engine handles the dispatching of notifications to different channels
type Engine struct {
	httpClient *http.Client
}

func NewEngine() *Engine {
	return &Engine{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send dispatches a message through the specified channel
func (e *Engine) Send(channel Channel, msg Message) error {
	if !channel.Enabled {
		return nil
	}

	logger.Info("[Notify] Sending notification through channel: %s (%s/%s)", channel.Name, channel.Type, channel.Provider)

	switch channel.Type {
	case "webhook":
		return e.sendWebhook(channel, msg)
	case "email":
		return e.sendEmail(channel, msg)
	default:
		return fmt.Errorf("unsupported channel type: %s", channel.Type)
	}
}

func (e *Engine) sendWebhook(channel Channel, msg Message) error {
	urlStr, _ := channel.Config["url"].(string)
	if urlStr == "" {
		return fmt.Errorf("webhook URL is empty for channel %s", channel.Name)
	}

	provider := strings.ToLower(channel.Provider)
	var payload []byte
	var err error

	switch provider {
	case "dingtalk":
		payload, err = e.buildDingTalkPayload(channel, msg)
		if secret, ok := channel.Config["secret"].(string); ok && secret != "" {
			timestamp := time.Now().UnixNano() / 1e6
			sign := e.signDingTalk(timestamp, secret)
			if strings.Contains(urlStr, "?") {
				urlStr += fmt.Sprintf("&timestamp=%d&sign=%s", timestamp, url.QueryEscape(sign))
			} else {
				urlStr += fmt.Sprintf("?timestamp=%d&sign=%s", timestamp, url.QueryEscape(sign))
			}
		}
	case "feishu":
		payload, err = e.buildFeishuPayload(channel, msg)
	case "wecom":
		payload, err = e.buildWeComPayload(channel, msg)
	default:
		// Generic webhook
		payload, _ = json.Marshal(msg)
	}

	if err != nil {
		return fmt.Errorf("failed to build webhook payload: %w", err)
	}

	logger.Info("[Notify] Sending webhook to %s, url: %s, payload: %s", provider, urlStr, string(payload))

	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Check for application-level errors (DingTalk/Feishu/WeCom use errcode)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Code    int    `json:"code"`    // Some use code
		Msg     string `json:"msg"`     // Some use msg
	}
	if err := json.Unmarshal(body, &result); err == nil {
		if result.ErrCode != 0 {
			return fmt.Errorf("webhook provider error: %d - %s", result.ErrCode, result.ErrMsg)
		}
		if result.Code != 0 && provider != "generic" {
			return fmt.Errorf("webhook provider error: %d - %s", result.Code, result.Msg)
		}
	}

	logger.Info("[Notify] Webhook sent successfully to %s, response: %s", provider, string(body))
	return nil
}

func (e *Engine) buildDingTalkPayload(channel Channel, msg Message) ([]byte, error) {
	// DingTalk markdown message
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": msg.Title,
			"text":  fmt.Sprintf("### %s\n\n%s", msg.Title, strings.ReplaceAll(msg.Body, "\n", "\n\n")),
		},
	}
	return json.Marshal(payload)
}

func (e *Engine) signDingTalk(timestamp int64, secret string) string {
	strToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(strToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (e *Engine) buildFeishuPayload(channel Channel, msg Message) ([]byte, error) {
	payload := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": fmt.Sprintf("%s\n\n%s", msg.Title, msg.Body),
		},
	}
	return json.Marshal(payload)
}

func (e *Engine) buildWeComPayload(channel Channel, msg Message) ([]byte, error) {
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": fmt.Sprintf("# %s\n%s", msg.Title, msg.Body),
		},
	}
	return json.Marshal(payload)
}

func (e *Engine) sendEmail(channel Channel, msg Message) error {
	host, _ := channel.Config["host"].(string)
	port, _ := channel.Config["port"].(float64) // JSON numbers are float64
	user, _ := channel.Config["username"].(string)
	password, _ := channel.Config["password"].(string)
	from, _ := channel.Config["from"].(string)
	recipientsRaw, _ := channel.Config["recipients"].([]interface{})

	if host == "" || port == 0 || from == "" || len(recipientsRaw) == 0 {
		return fmt.Errorf("incomplete email configuration for channel %s", channel.Name)
	}

	recipients := make([]string, len(recipientsRaw))
	for i, r := range recipientsRaw {
		recipients[i], _ = r.(string)
	}

	auth := smtp.PlainAuth("", user, password, host)
	addr := fmt.Sprintf("%s:%d", host, int(port))

	header := make(map[string]string)
	header["From"] = from
	header["To"] = strings.Join(recipients, ",")
	header["Subject"] = msg.Title
	header["Content-Type"] = "text/plain; charset=UTF-8"

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + msg.Body

	err := smtp.SendMail(addr, auth, from, recipients, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
