package desktop

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"

	"github.com/Tarhche/cli/internal/access"
	"github.com/Tarhche/cli/internal/resource"
)

// operation is one of the five things a resource may be asked.
type operation = resource.Operation

const (
	operationList   = resource.OperationList
	operationGet    = resource.OperationGet
	operationCreate = resource.OperationCreate
	operationUpdate = resource.OperationUpdate
	operationDelete = resource.OperationDelete
)

// methods are what each operation is asked with.
var methods = map[operation]string{
	operationList:   http.MethodGet,
	operationGet:    http.MethodGet,
	operationCreate: http.MethodPost,
	operationUpdate: http.MethodPut,
	operationDelete: http.MethodDelete,
}

// methodOf is what an action is asked with: its method, POST when it names
// none.
func methodOf(a resource.Action) string {
	if len(a.Method) == 0 {
		return http.MethodPost
	}

	return a.Method
}

// routeOf is the path op is served at: for one's own things unless
// everybody's are asked for or one's own are not served, for everybody's
// otherwise; empty when op is not served that way.
func routeOf(d resource.Definition, op operation, everyone bool) string {
	path, _, _ := d.Route(op, everyone)

	return path
}

// audiencesOf are the audiences of a resource the app lists that a session
// granted what granted says may list, one's own first.
func audiencesOf(d resource.Definition, granted access.Granted) []string {
	audiences := []string{}

	for _, audience := range d.AppAudiences() {
		if list := d.PathsOf(audience).List; len(list) > 0 && granted.Allows(methods[operationList], list) {
			audiences = append(audiences, string(audience))
		}
	}

	return audiences
}

// keyed reports whether an action is asked of one thing of its resource,
// which its route names the key of, rather than of the resource.
func keyed(d resource.Definition, a resource.Action) bool {
	return len(d.BodyKey(a.Path, d.Key)) < len(d.Key)
}

// filtersOf are the filters the listings of a resource take, as the
// frontend offers them.
func filtersOf(d resource.Definition) []ResourceFilter {
	filters := make([]ResourceFilter, len(d.Filters))

	for i, filter := range d.Filters {
		audiences := make([]string, len(filter.Audiences))
		for j, audience := range filter.Audiences {
			audiences[j] = string(audience)
		}

		filters[i] = ResourceFilter{
			Name:        filter.Name,
			Description: filter.Description,
			Values:      append([]string{}, filter.Values...),
			Moment:      filter.Moment,
			Audiences:   audiences,
		}
	}

	return filters
}

// chosenAction is the action of that name asked of the audience asked, or
// of the other when only it is served.
func chosenAction(d resource.Definition, name string, everyone bool) (resource.Action, bool) {
	var other *resource.Action

	for i, a := range d.Actions {
		if a.Name != name {
			continue
		}

		if a.Mine != everyone {
			return a, true
		}

		if other == nil {
			other = &d.Actions[i]
		}
	}

	if other == nil {
		return resource.Action{}, false
	}

	return *other, true
}

// withDefaults is body with the defaults it does not say itself.
func withDefaults(body, defaults map[string]any) map[string]any {
	for field, value := range defaults {
		if _, said := body[field]; !said {
			body[field] = value
		}
	}

	return body
}

// requestBody is what a request carries of fields: nothing for none.
func requestBody(fields map[string]any) any {
	if len(fields) == 0 {
		return nil
	}

	return fields
}

// decodeJSON is JSON content as a value, its numbers as they were written,
// so that a template writes 31536000 rather than 3.1536e+07; nothing at
// all is null.
func decodeJSON(content []byte) (any, error) {
	if len(bytes.TrimSpace(content)) == 0 {
		return nil, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	return value, nil
}

// decodeItems are the items of a page as decodeJSON reads each: they are
// JSON, since the page they are on was read whole.
func decodeItems(raws []json.RawMessage) []any {
	items := make([]any, len(raws))
	for i, raw := range raws {
		items[i], _ = decodeJSON(raw)
	}

	return items
}

// cloned is fields, which may be nil, as a map of its own.
func cloned(fields map[string]any) map[string]any {
	if fields == nil {
		return make(map[string]any)
	}

	return maps.Clone(fields)
}
