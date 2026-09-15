package line

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const apiBase = "https://api.line.me"

// Client is a minimal LINE Messaging API client for the read-only calls
// this program needs (resolving group and user display names).
type Client struct {
	accessToken string
	http        *http.Client
}

func NewClient(accessToken string) *Client {
	return &Client{
		accessToken: accessToken,
		http:        &http.Client{Timeout: 10 * time.Second},
	}
}

type groupSummary struct {
	GroupID   string `json:"groupId"`
	GroupName string `json:"groupName"`
}

type profile struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
}

func (c *Client) get(path string, out any) error {
	if c.accessToken == "" {
		return fmt.Errorf("no channel access token configured")
	}

	req, err := http.NewRequest(http.MethodGet, apiBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("line api %s returned status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// GroupName fetches the display name of a group chat.
func (c *Client) GroupName(groupID string) (string, error) {
	var g groupSummary
	if err := c.get("/v2/bot/group/"+groupID+"/summary", &g); err != nil {
		return "", err
	}
	return g.GroupName, nil
}

// UserName fetches a user's display name. If groupID or roomID is set, the
// user is looked up as a member of that group/room (works even if they
// haven't added the bot as a friend); otherwise it looks up their profile
// directly, which requires them to be a friend of the bot.
func (c *Client) UserName(userID, groupID, roomID string) (string, error) {
	var p profile
	var err error
	switch {
	case groupID != "":
		err = c.get("/v2/bot/group/"+groupID+"/member/"+userID, &p)
	case roomID != "":
		err = c.get("/v2/bot/room/"+roomID+"/member/"+userID, &p)
	default:
		err = c.get("/v2/bot/profile/"+userID, &p)
	}
	if err != nil {
		return "", err
	}
	return p.DisplayName, nil
}
