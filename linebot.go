package line

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const apiBase = "https://api.line.me"

// apiDataBase is the host serving binary message content (images, video,
// audio, files), as opposed to apiBase's JSON endpoints.
const apiDataBase = "https://api-data.line.me"

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

// maxContentBytes limits the size of message content MessageContent
// downloads, since the whole content is held in memory.
const maxContentBytes = 50 << 20 // 50 MiB

// MessageContent downloads the binary content of a message (e.g. an image,
// video, audio, or file message), identified by its message ID. It returns
// the content along with its Content-Type, or an error if the content is
// larger than maxContentBytes.
func (c *Client) MessageContent(messageID string) ([]byte, string, error) {
	if c.accessToken == "" {
		return nil, "", fmt.Errorf("no channel access token configured")
	}

	req, err := http.NewRequest(http.MethodGet, apiDataBase+"/v2/bot/message/"+messageID+"/content", nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("line api message content returned status %d", resp.StatusCode)
	}
	b, err := readLimited(resp.Body, maxContentBytes)
	if err != nil {
		return nil, "", err
	}
	return b, resp.Header.Get("Content-Type"), nil
}

// readLimited reads all of r, failing if it's longer than n bytes.
func readLimited(r io.Reader, n int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, n+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) == n+1 {
		return nil, fmt.Errorf("contents too long")
	}
	return b, nil
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
