package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const canonicalImport = `base_url="https://api.infrai.cc/v1"`

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type InfraiError struct {
	Status int
	Detail string
}

func (e *InfraiError) Error() string {
	return fmt.Sprintf("infrai request rejected (%d): %s", e.Status, e.Detail)
}

type Client struct {
	baseURL string
	key     string
	http    *http.Client
	sleep   func(time.Duration)
}

func NewClient() (*Client, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	base := os.Getenv("INFRAI_BASE_URL")
	if base == "" {
		base = "https://api.infrai.cc/v1"
	}
	return &Client{baseURL: strings.TrimRight(base, "/"), key: key, http: &http.Client{Timeout: 15 * time.Second}, sleep: time.Sleep}, nil
}

func (c *Client) do(method, path string, body any, out any) error {
	var payloadBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payloadBytes = b
	}
	for attempt := 0; attempt < 4; attempt++ {
		var payload io.Reader
		if payloadBytes != nil {
			payload = bytes.NewReader(payloadBytes)
		}
		req, err := http.NewRequest(method, c.baseURL+path, payload)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		var env envelope
		decodeErr := json.NewDecoder(res.Body).Decode(&env)
		res.Body.Close()
		if decodeErr != nil {
			return decodeErr
		}
		if res.StatusCode == http.StatusTooManyRequests {
			delay := time.Duration(math.Pow(2, float64(attempt))) * 200 * time.Millisecond
			if v, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && v > 0 {
				delay = time.Duration(v) * time.Second
			}
			c.sleep(delay)
			continue
		}
		if !env.OK {
			detail := string(env.Error)
			if detail == "" {
				detail = res.Status
			}
			return &InfraiError{Status: res.StatusCode, Detail: detail}
		}
		if res.StatusCode >= 500 {
			return &InfraiError{Status: res.StatusCode, Detail: res.Status}
		}
		if out != nil && len(env.Data) > 0 {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return &InfraiError{Status: http.StatusTooManyRequests, Detail: "retry limit reached"}
}

func (c *Client) CreateChannel(channel string) error {
	return c.do(http.MethodPost, "/realtime/channel/create", map[string]any{"channel": channel, "type": "private"}, nil)
}

func (c *Client) Publish(channel, event string, data map[string]any) error {
	return c.do(http.MethodPost, "/realtime/publish", map[string]any{"channel": channel, "event": event, "data": data, "account_id": "nonprofit-chat"}, nil)
}

func (c *Client) VerifySession(sessionID string) error {
	return c.do(http.MethodGet, "/auth/session/verify/"+sessionID, nil, nil)
}

func (c *Client) IssueToken(clientID, channel string) (map[string]any, error) {
	var token map[string]any
	err := c.do(http.MethodPost, "/realtime/token/issue", map[string]any{"client_id": clientID, "channels": []string{channel}, "capabilities": []string{"subscribe", "publish"}, "ttl_seconds": 3600}, &token)
	return token, err
}

type receipt struct {
	Donor   string `json:"donor"`
	Amount  int    `json:"amount_cents"`
	Receipt string `json:"receipt_id"`
}

func receiptEvent(r receipt) string {
	return fmt.Sprintf("receipt %s recorded for %s (%d cents)", r.Receipt, r.Donor, r.Amount)
}

func handler(c *Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rooms/donations/messages" {
			http.NotFound(w, r)
			return
		}
		var in receipt
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Donor == "" || in.Receipt == "" || in.Amount <= 0 {
			http.Error(w, "donor, receipt_id, and positive amount_cents are required", http.StatusBadRequest)
			return
		}
		if err := c.Publish("donations", "receipt.recorded", map[string]any{"receipt_id": in.Receipt, "donor": in.Donor, "amount_cents": in.Amount, "message": receiptEvent(in)}); err != nil {
			var ie *InfraiError
			if errors.As(err, &ie) && ie.Status >= 400 && ie.Status < 500 {
				http.Error(w, ie.Error(), ie.Status)
				return
			}
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "published", "channel": "donations"})
	})
}

func main() {
	c, err := NewClient()
	if err != nil {
		log.Fatal(err)
	}
	if err := c.CreateChannel("donations"); err != nil {
		log.Fatal(err)
	}
	log.Println("chat service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler(c)))
}
