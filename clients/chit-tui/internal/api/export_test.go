package api

import (
	"fmt"
	"time"
)

// SetTimeout shortens a client's request timeout so a test need not wait
// out the production one. It accepts only a client from this package.
func SetTimeout(c ChitClient, d time.Duration) {
	hc, ok := c.(*httpClient)
	if !ok {
		panic(fmt.Sprintf("api.SetTimeout: %T is not a client from this package", c))
	}
	hc.http.Timeout = d
}
