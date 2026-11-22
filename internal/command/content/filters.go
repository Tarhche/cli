package content

import (
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/resource"
)

// filterFlags are a flag for each filter a resource's listings take, and
// the query their values ask.
type filterFlags struct {
	resource resource.Definition

	// audiences are whose things may be listed with them.
	audiences []resource.Audience

	now    func() time.Time
	values map[string]*string
}

func newFilterFlags(d resource.Definition, audiences []resource.Audience) *filterFlags {
	return &filterFlags{resource: d, audiences: audiences, now: time.Now}
}

// offered are the filters the listing of any of the audiences takes.
func (f *filterFlags) offered() []resource.Filter {
	var names []string

	for _, audience := range f.audiences {
		for _, filter := range f.resource.FiltersOf(audience) {
			names = append(names, filter.Name)
		}
	}

	return slices.DeleteFunc(slices.Clone(f.resource.Filters), func(filter resource.Filter) bool {
		return !slices.Contains(names, filter.Name)
	})
}

// define defines the flag of every filter offered on flags.
func (f *filterFlags) define(flags *console.FlagSet) {
	f.values = make(map[string]*string)

	for _, filter := range f.offered() {
		value := new(string)
		console.Var(flags, value, console.Long(flagName(filter.Name)), f.explain(filter), console.Default(""))
		f.values[filter.Name] = value
	}
}

// explain is the usage of the flag of a filter.
func (f *filterFlags) explain(filter resource.Filter) string {
	usage := filter.Description

	if len(filter.Values) > 0 {
		usage += ": " + strings.Join(filter.Values, ", ")
	}

	if filter.Moment {
		usage += ", a moment in RFC 3339 or a duration before now (1h)"
	}

	if len(f.audiences) > 1 && !takes(f.resource.FiltersOf(resource.AudienceMine), filter) {
		usage += ", among everybody's (--all-users)"
	}

	return usage + "."
}

// query is what the filters given ask of the listing of audience. A filter
// the listing does not take is a usage error.
func (f *filterFlags) query(flags *console.FlagSet, audience resource.Audience) (url.Values, error) {
	query := url.Values{}
	taken := f.resource.FiltersOf(audience)

	for _, filter := range f.offered() {
		value := *f.values[filter.Name]
		if len(value) == 0 {
			continue
		}

		if !takes(taken, filter) {
			return nil, usageError(flags, "--%s narrows everybody's %s: add --all-users.", flagName(filter.Name), plural(f.resource))
		}

		if filter.Moment {
			value = f.moment(value)
		}

		query.Set(filter.Name, value)
	}

	return query, nil
}

// moment is a moment as the API takes it: one given as a duration is that
// long before now; anything else is given as it is, for the API to judge.
func (f *filterFlags) moment(given string) string {
	before, err := time.ParseDuration(given)
	if err != nil {
		return given
	}

	return f.now().Add(-before).UTC().Format(time.RFC3339)
}

// takes reports whether filters hold filter.
func takes(filters []resource.Filter, filter resource.Filter) bool {
	return slices.ContainsFunc(filters, func(f resource.Filter) bool { return f.Name == filter.Name })
}
