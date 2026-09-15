package line

import "sync"

// NameCache resolves and caches display names for groups and users for the
// lifetime of the process, so we don't hit the LINE API on every message.
type NameCache struct {
	client *Client

	mu     sync.Mutex
	groups map[string]string
	users  map[string]string
}

func NewNameCache(client *Client) *NameCache {
	return &NameCache{
		client: client,
		groups: make(map[string]string),
		users:  make(map[string]string),
	}
}

// GroupName returns a group's display name, falling back to a stable
// placeholder if it can't be resolved (e.g. no access token configured).
func (c *NameCache) GroupName(groupID string) string {
	c.mu.Lock()
	if name, ok := c.groups[groupID]; ok {
		c.mu.Unlock()
		return name
	}
	c.mu.Unlock()

	name, err := c.client.GroupName(groupID)
	if err != nil || name == "" {
		name = "group_" + groupID
	}

	c.mu.Lock()
	c.groups[groupID] = name
	c.mu.Unlock()
	return name
}

// UserName returns a user's display name, falling back to a stable
// placeholder if it can't be resolved.
func (c *NameCache) UserName(userID, groupID, roomID string) string {
	if userID == "" {
		return "unknown"
	}

	c.mu.Lock()
	if name, ok := c.users[userID]; ok {
		c.mu.Unlock()
		return name
	}
	c.mu.Unlock()

	name, err := c.client.UserName(userID, groupID, roomID)
	if err != nil || name == "" {
		name = "user_" + userID
	}

	c.mu.Lock()
	c.users[userID] = name
	c.mu.Unlock()
	return name
}
