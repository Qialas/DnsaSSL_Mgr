package whois

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	whoisclient "github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
)

// LookupExpiresAt returns the domain expiration time parsed from WHOIS data.
func LookupExpiresAt(ctx context.Context, domain string) (*time.Time, error) {
	type result struct {
		expiresAt *time.Time
		err       error
	}

	ch := make(chan result, 1)
	go func() {
		client := whoisclient.NewClient()
		client.SetTimeout(8 * time.Second)

		raw, err := client.Whois(domain)
		if err != nil {
			expiresAt, rdapErr := lookupRDAPExpiresAt(ctx, domain)
			if rdapErr != nil {
				ch <- result{err: fmt.Errorf("%w；RDAP查询失败：%w", err, rdapErr)}
				return
			}
			ch <- result{expiresAt: expiresAt}
			return
		}

		info, err := whoisparser.Parse(raw)
		if err != nil {
			expiresAt, rdapErr := lookupRDAPExpiresAt(ctx, domain)
			if rdapErr != nil {
				ch <- result{err: fmt.Errorf("%w；RDAP查询失败：%w", err, rdapErr)}
				return
			}
			ch <- result{expiresAt: expiresAt}
			return
		}

		if info.Domain.ExpirationDateInTime == nil {
			expiresAt, err := lookupRDAPExpiresAt(ctx, domain)
			ch <- result{expiresAt: expiresAt, err: err}
			return
		}
		expiresAt := info.Domain.ExpirationDateInTime.UTC()
		ch <- result{expiresAt: &expiresAt}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		return res.expiresAt, res.err
	}
}

func lookupRDAPExpiresAt(ctx context.Context, domain string) (*time.Time, error) {
	type rdapEvent struct {
		EventAction string `json:"eventAction"`
		EventDate   string `json:"eventDate"`
	}
	type rdapResponse struct {
		Events []rdapEvent `json:"events"`
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://rdap.org/domain/"+url.PathEscape(domain), nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP状态码：%d", resp.StatusCode)
	}

	var body rdapResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	for _, event := range body.Events {
		if event.EventAction != "expiration" && event.EventAction != "registration expiration" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, event.EventDate)
		if err != nil {
			return nil, err
		}
		expiresAt := parsed.UTC()
		return &expiresAt, nil
	}
	return nil, nil
}
