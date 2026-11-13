package remotesync

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"go-unit-mangement/internal/auth"
	"go-unit-mangement/internal/models"
	"go-unit-mangement/internal/units"
)

const (
	// httpTimeout bounds one request to a remote.
	httpTimeout = 30 * time.Second
	// tokenTTL is how long each JWT the client signs is valid. A fresh one is
	// signed for every request, so it can be short. The remote re-checks a
	// WebSocket's token every minute, so the event stream reconnects about
	// this often.
	tokenTTL = 5 * time.Minute
	// maxResponseSize caps a remote's unit list.
	maxResponseSize = 32 << 20
)

// Client talks to another instance's API, authenticating with this server's
// key pair: every request carries a fresh RS256 JWT whose "kid" is keyID,
// under which the remote has the public key registered as an API key.
type Client struct {
	baseURL string
	keyID   uuid.UUID
	key     *rsa.PrivateKey
	http    *http.Client
}

// NewClient returns a client for the instance at baseURL (without /api).
func NewClient(baseURL string, keyID uuid.UUID, key *rsa.PrivateKey) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		keyID:   keyID,
		key:     key,
		http:    &http.Client{Timeout: httpTimeout},
	}
}

func (c *Client) authHeader() (string, error) {
	token, err := auth.SignAPIKeyToken(c.key, c.keyID, tokenTTL)
	if err != nil {
		return "", err
	}
	return "Bearer " + token, nil
}

// StatusError is returned when the remote answers with a status other than
// 200 OK.
type StatusError struct {
	// Method is empty for GET.
	Method  string
	Path    string
	Status  int
	Message string
}

func (e *StatusError) Error() string {
	method := e.Method
	if method == "" {
		method = http.MethodGet
	}
	if e.Message != "" {
		return fmt.Sprintf("%s %s: status %d: %s", method, e.Path, e.Status, e.Message)
	}
	return fmt.Sprintf("%s %s: status %d", method, e.Path, e.Status)
}

// ListUnits returns every unit on the remote.
func (c *Client) ListUnits(ctx context.Context) ([]RemoteUnit, error) {
	const path = "/api/units"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	header, err := c.authHeader()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", header)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := io.LimitReader(resp.Body, maxResponseSize)
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(body).Decode(&e)
		return nil, &StatusError{Path: path, Status: resp.StatusCode, Message: e.Error}
	}
	var list []RemoteUnit
	if err := json.NewDecoder(body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode GET %s: %w", path, err)
	}
	return list, nil
}

// AcknowledgeRequest acknowledges the unit's status request of code on the
// remote. 202 Accepted counts too: the remote forwarded it to the instance it
// mirrors the unit from.
func (c *Client) AcknowledgeRequest(ctx context.Context, unitID uuid.UUID, code int) error {
	path := fmt.Sprintf("/api/units/%s/status-requests/%d/acknowledge", unitID, code)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	header, err := c.authHeader()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", header)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := io.LimitReader(resp.Body, maxResponseSize)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(body).Decode(&e)
		return &StatusError{Method: http.MethodPost, Path: path, Status: resp.StatusCode, Message: e.Error}
	}
	return nil
}

// DialEvents opens the remote's unit event stream. The caller closes the
// connection.
func (c *Client) DialEvents(ctx context.Context) (*websocket.Conn, error) {
	header, err := c.authHeader()
	if err != nil {
		return nil, err
	}
	// Dial maps http(s):// to ws(s):// itself.
	conn, resp, err := websocket.Dial(ctx, c.baseURL+"/api/units/events", &websocket.DialOptions{
		HTTPClient: &http.Client{Timeout: httpTimeout},
		HTTPHeader: http.Header{"Authorization": []string{header}},
	})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("dial %s/api/units/events: %w", c.baseURL, err)
	}
	// A unit list message can be larger than the default 32 KiB limit.
	conn.SetReadLimit(1 << 20)
	return conn, nil
}

// RemoteUnit is a unit as the remote's API serves it (see the unit JSON in
// internal/server). Who created and changed it on the remote isn't mirrored:
// those users don't exist here.
type RemoteUnit struct {
	ID           uuid.UUID            `json:"id"`
	Name         string               `json:"name"`
	Position     *RemotePosition      `json:"position"`
	Status       *RemoteStatus        `json:"status"`
	Symbol       *models.UnitSymbol   `json:"symbol"`
	TacticalName *models.TacticalName `json:"tacticalName"`
	// Requests are the unit's FMS 0, 5 and 9 reports. Remotes from before
	// requests existed send none.
	Requests []RemoteStatusRequest `json:"requests"`
}

// RemoteStatusRequest is a unit's latest report of a request code.
type RemoteStatusRequest struct {
	Code           int        `json:"code"`
	Timestamp      time.Time  `json:"timestamp"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt"`
	AcknowledgedBy *string    `json:"acknowledgedBy"`
}

type RemotePosition struct {
	Lat       float64    `json:"lat"`
	Lon       float64    `json:"lon"`
	Height    *float64   `json:"height"`
	Accuracy  *float64   `json:"accuracy"`
	Speed     *float64   `json:"speed"`
	Course    *float64   `json:"course"`
	Timestamp *time.Time `json:"timestamp"`
}

// RemoteStatus is a unit's FMS status. Remotes from before statuses existed
// send none.
type RemoteStatus struct {
	Code      *int       `json:"code"`
	Timestamp *time.Time `json:"timestamp"`
}

// Input returns the unit's editable fields.
func (u *RemoteUnit) Input() units.Input {
	in := units.Input{Name: u.Name, Symbol: u.Symbol, TacticalName: u.TacticalName}
	if p := u.Position; p != nil && p.Timestamp != nil {
		in.Position = &units.Position{
			Latitude: p.Lat, Longitude: p.Lon, Height: p.Height, Accuracy: p.Accuracy,
			Speed: p.Speed, Course: p.Course, Timestamp: *p.Timestamp,
		}
	}
	if st := u.Status; st != nil && st.Code != nil && st.Timestamp != nil {
		in.Status = &units.Status{Code: *st.Code, Timestamp: *st.Timestamp}
	}
	for _, r := range u.Requests {
		in.StatusRequests = append(in.StatusRequests, units.StatusRequest{
			Code: r.Code, Timestamp: r.Timestamp, AcknowledgedAt: r.AcknowledgedAt, AcknowledgedBy: r.AcknowledgedBy,
		})
	}
	return in
}

// remoteEvent is a message on the remote's unit event stream.
type remoteEvent struct {
	Type units.EventType `json:"type"`
	ID   uuid.UUID       `json:"id"`
	Unit *RemoteUnit     `json:"unit"`
}
