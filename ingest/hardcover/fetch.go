package hardcover

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// endpoint is Hardcover's public GraphQL endpoint.
const endpoint = "https://api.hardcover.app/v1/graphql"

// readStatusID is the Hardcover status_id for the "Read" shelf (1 want, 2 reading,
// 3 read). It is a literal in the shelf query, not a bound variable.
const readStatusID = 3

// meQuery resolves the authenticated user's id from the bearer token.
const meQuery = `query { me { id username } }`

// shelfQuery pages a user's Read shelf. status_id is fixed to readStatusID; userId,
// limit, and offset are bound variables. The selection is the bibliographic subset the
// crosswalk needs plus the reading-log extras (rating, read dates).
const shelfQuery = `query ReadShelf($userId: Int!, $limit: Int!, $offset: Int!) {
  user_books(
    where: { user_id: { _eq: $userId }, status_id: { _eq: 3 } }
    order_by: { id: asc }
    limit: $limit
    offset: $offset
  ) {
    id
    rating
    last_read_date
    first_read_date
    book {
      id
      slug
      title
      subtitle
      description
      image { url }
      contributions { contribution author { name } }
      cached_tags
      editions {
        id
        isbn_13
        isbn_10
        reading_format_id
        edition_format
        physical_format
        image { url }
      }
    }
  }
}`

// introspectQuery dumps a type's fields; Hardcover's schema drifts, so the subcommand
// keeps this affordance for diagnosing shape changes.
const introspectQuery = `query I($n: String!) { __type(name: $n) { name kind fields { name type { name kind ofType { name kind } } } } }`

// normalizeToken trims whitespace and strips a leading, case-insensitive "Bearer "
// prefix, so a raw token or a pre-prefixed one both work. It never logs the value.
func normalizeToken(t string) string {
	t = strings.TrimSpace(t)
	if len(t) >= 7 && strings.EqualFold(t[:7], "bearer ") {
		t = strings.TrimSpace(t[7:])
	}
	return t
}

// gql posts a GraphQL query and unmarshals data into out. A non-2xx status or a
// GraphQL errors array is returned as an error; the bearer token is sent in the
// Authorization header and never included in an error.
func (p Provider) gql(ctx context.Context, query string, variables map[string]any, out any) error {
	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("hardcover: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("hardcover: decode response: %w", err)
	}
	if len(env.Errors) > 0 && string(env.Errors) != "null" {
		return fmt.Errorf("hardcover: GraphQL errors: %s", env.Errors)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

// userID resolves the authenticated user's numeric id. Hardcover may return `me` as a
// single object or a single-element array; both are handled.
func (p Provider) userID(ctx context.Context) (int, error) {
	var data struct {
		Me json.RawMessage `json:"me"`
	}
	if err := p.gql(ctx, meQuery, nil, &data); err != nil {
		return 0, err
	}
	type me struct {
		ID int `json:"id"`
	}
	if len(data.Me) > 0 && data.Me[0] == '[' {
		var arr []me
		if err := json.Unmarshal(data.Me, &arr); err != nil {
			return 0, err
		}
		if len(arr) == 0 {
			return 0, fmt.Errorf("hardcover: me query returned no user")
		}
		return arr[0].ID, nil
	}
	var m me
	if err := json.Unmarshal(data.Me, &m); err != nil {
		return 0, err
	}
	if m.ID == 0 {
		return 0, fmt.Errorf("hardcover: me query returned no user")
	}
	return m.ID, nil
}

// fetchShelf pages the authenticated user's Read shelf into a flat userBook slice,
// de-duping by book id so multi-edition reads collapse to one book. ctx cancels
// in-flight requests. It stops at the first short page.
func (p Provider) fetchShelf(ctx context.Context) ([]userBook, error) {
	uid, err := p.userID(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[int]bool{}
	var out []userBook
	for offset := 0; ; offset += p.limit {
		var data struct {
			UserBooks []userBook `json:"user_books"`
		}
		vars := map[string]any{"userId": uid, "limit": p.limit, "offset": offset}
		if err := p.gql(ctx, shelfQuery, vars, &data); err != nil {
			return nil, err
		}
		for _, ub := range data.UserBooks {
			if ub.Book.ID != 0 && seen[ub.Book.ID] {
				continue
			}
			seen[ub.Book.ID] = true
			out = append(out, ub)
		}
		if len(data.UserBooks) < p.limit {
			break
		}
	}
	return out, nil
}

// Introspect dumps the fields of a GraphQL type (default "query_root") as pretty JSON,
// a diagnostic for Hardcover's drifting schema. It is exposed for the CLI subcommand.
func (p Provider) Introspect(ctx context.Context, typeName string) ([]byte, error) {
	if typeName == "" {
		typeName = "query_root"
	}
	var data json.RawMessage
	if err := p.gql(ctx, introspectQuery, map[string]any{"n": typeName}, &data); err != nil {
		return nil, err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		return data, nil
	}
	return pretty.Bytes(), nil
}
