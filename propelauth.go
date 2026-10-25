package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const propelAuthAPI = "https://propelauth-api.com/api/backend/v1"

type propelAuth struct {
	hostname string
	apiKey   string
}

type org struct {
	ID   string `json:"org_id"`
	Name string `json:"name"`
}

type user struct {
	ID        string `json:"user_id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Enabled   bool   `json:"enabled"`
	Locked    bool   `json:"locked"`
	Orgs      map[string]struct {
		Role string `json:"user_role"`
	} `json:"org_id_to_org_info"`
}

// newPropelAuth returns a client of env, whose API key it reads from AWS with awsProfile.
func newPropelAuth(ctx context.Context, env environment, awsProfile string) (*propelAuth, error) {
	apiKey, err := fetchAPIKey(ctx, env.Secret, awsProfile)
	if err != nil {
		return nil, err
	}
	return &propelAuth{hostname: env.AuthHostname, apiKey: apiKey}, nil
}

// orgs lists the organizations by name.
func (p *propelAuth) orgs(ctx context.Context) ([]org, error) {
	orgs, err := fetchAll[org](ctx, p, "/org/query", "orgs", url.Values{})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(orgs, func(a, b org) int {
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return orgs, nil
}

// usersInOrg lists the users of an organization by email.
func (p *propelAuth) usersInOrg(ctx context.Context, orgID string) ([]user, error) {
	users, err := fetchAll[user](ctx, p, "/user/org/"+orgID, "users", url.Values{"include_orgs": {"true"}})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(users, func(a, b user) int { return cmp.Compare(a.Email, b.Email) })
	return users, nil
}

// fetchAll walks every page of a paginated endpoint, whose items sit under field.
func fetchAll[T any](ctx context.Context, p *propelAuth, path, field string, query url.Values) ([]T, error) {
	var all []T
	query.Set("page_size", "100")
	for page := 0; ; page++ {
		query.Set("page_number", strconv.Itoa(page))
		var body map[string]json.RawMessage
		if err := p.get(ctx, path+"?"+query.Encode(), &body); err != nil {
			return nil, err
		}
		var items []T
		if err := json.Unmarshal(body[field], &items); err != nil {
			return nil, fmt.Errorf("decoding %s: %w", field, err)
		}
		all = append(all, items...)
		var hasMore bool
		if err := json.Unmarshal(body["has_more_results"], &hasMore); err != nil || !hasMore {
			return all, nil
		}
	}
}

func (p *propelAuth) get(ctx context.Context, pathAndQuery string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, propelAuthAPI+pathAndQuery, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("X-Propelauth-url", p.hostname)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PropelAuth GET %s: %s %s", req.URL.Path, resp.Status, body)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
