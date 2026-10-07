package api

import "time"

// SetTimeout shortens a client's request timeout so a test need not wait
// out the production one.
func SetTimeout(c ChitClient, d time.Duration) { c.(*httpClient).http.Timeout = d }
