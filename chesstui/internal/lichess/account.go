package lichess

import "context"

func (c *Client) Account(ctx context.Context) (Account, error) {
	var account Account
	err := c.doJSON(ctx, "GET", "/api/account", nil, &account)
	return account, err
}
