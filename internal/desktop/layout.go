package desktop

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/Tarhche/cli/internal/access"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/resource"
)

// sites are where a context's links lead, by site: its website and its API,
// with no slash at their end; a site the context names none of is none.
type sites map[resource.Site]string

// sitesOf are the sites of c.
func sitesOf(c config.Context) sites {
	return sites{
		resource.SiteWeb: strings.TrimRight(c.Web, "/"),
		resource.SiteAPI: strings.TrimRight(c.API, "/"),
	}
}

// reaches reports whether a link to site can be followed: one to a page of
// the app always, one to a site the context names.
func (s sites) reaches(site resource.Site) bool {
	return site == resource.SiteApp || len(s[site]) > 0
}

// address is where path on site is: path itself on the app.
func (s sites) address(site resource.Site, path string) string {
	if site == resource.SiteApp {
		return path
	}

	return s[site] + path
}

// audienceOf is whose things a listing of everybody's, or of one's own, is
// of.
func audienceOf(everyone bool) resource.Audience {
	if everyone {
		return resource.AudienceEveryone
	}

	return resource.AudienceMine
}

// allows reports whether the routes allow every one of ops.
func (r Routes) allows(ops []operation) bool {
	for _, op := range ops {
		allowed := map[operation]bool{operationList: r.List, operationGet: r.Get, operationCreate: r.Create, operationUpdate: r.Update, operationDelete: r.Delete}[op]
		if !allowed {
			return false
		}
	}

	return true
}

// newOf is where one of d is made, on the web dashboard, for a session that
// may make one or not: nowhere when it may not, or sites have no website.
func newOf(d resource.Definition, creates bool, sites sites) ResourceNew {
	if len(d.App.New.Name) == 0 || !creates || !sites.reaches(resource.SiteWeb) {
		return ResourceNew{}
	}

	return ResourceNew{Name: d.App.New.Name, URL: sites.address(resource.SiteWeb, d.App.New.Path)}
}

// linksOf are the links of d, each offered in those of the audiences the
// session may list that it is offered to, where the session is allowed what
// it requires, and only where sites reach it.
func linksOf(d resource.Definition, audiences []string, granted access.Granted, sites sites) []ResourceLink {
	links := make([]ResourceLink, len(d.App.Links))

	for i, link := range d.App.Links {
		offered := []string{}

		for _, audience := range audiences {
			everyone := audience == string(resource.AudienceEveryone)

			if sites.reaches(link.Site) && link.OfferedTo(resource.Audience(audience)) && routesOf(d, everyone, granted).allows(link.Requires) {
				offered = append(offered, audience)
			}
		}

		links[i] = ResourceLink{Name: link.Name, Kind: string(link.Kind), Resource: link.Resource, Filter: link.Filter, Audiences: offered}
	}

	return links
}

// withPagesListed are described with no link to a page of the app the
// session may not see: everybody's listing of a resource it may not list.
func withPagesListed(described []ResourceDefinition) []ResourceDefinition {
	listsEverybodys := func(name string) bool {
		return slices.ContainsFunc(described, func(d ResourceDefinition) bool {
			return d.Name == name && slices.Contains(d.Audiences, string(resource.AudienceEveryone))
		})
	}

	for _, d := range described {
		for i, link := range d.Links {
			if len(link.Resource) > 0 && !listsEverybodys(link.Resource) {
				d.Links[i].Audiences = []string{}
			}
		}
	}

	return described
}

// layout is how the rows of a listing of one audience of a resource are laid
// out: its cells, where its links lead, which of its actions are offered,
// and which rows are not deleted.
type layout struct {
	cells     []cell
	links     []rowLink
	actions   []rowAction
	protected *cell
}

// rowLink is where a link leads for a row.
type rowLink struct {
	name string
	site resource.Site
	path cell
}

// rowAction is an action of one thing, offered for the rows when holds.
type rowAction struct {
	name string
	when *cell
}

// layoutOf is how the rows of a listing of d's audience are laid out.
func layoutOf(d resource.Definition, audience resource.Audience) (layout, error) {
	var l layout

	for format := range strings.SplitSeq(table(d), "\t") {
		parsed, err := parse(format)
		if err != nil {
			return layout{}, err
		}

		l.cells = append(l.cells, *parsed)
	}

	for _, link := range d.App.Links {
		if !link.OfferedTo(audience) {
			continue
		}

		path, err := parse(link.Path)
		if err != nil {
			return layout{}, err
		}

		l.links = append(l.links, rowLink{name: link.Name, site: link.Site, path: *path})
	}

	for _, a := range d.Actions {
		if len(a.Title) == 0 || !keyed(d, a) {
			continue
		}

		when, err := parseOptional(a.When)
		if err != nil {
			return layout{}, err
		}

		l.actions = append(l.actions, rowAction{name: a.Name, when: when})
	}

	protected, err := parseOptional(d.App.Protected)
	if err != nil {
		return layout{}, err
	}

	l.protected = protected

	return l, nil
}

// row is a row of a listing as it is laid out: its key, which a row that
// says none has none of, its cells, and itself, which, read from JSON, is
// always written as JSON again; where its links lead on sites, which of
// its actions are offered, and whether it is deleted.
func (l layout) row(d resource.Definition, sites sites, row any) (Row, error) {
	key, err := d.KeyOf(row)
	if err != nil {
		key = []string{}
	}

	item, _ := json.Marshal(row)
	laidOut := Row{Key: key, Cells: make([]string, len(l.cells)), Item: item, Links: map[string]string{}, Actions: []string{}}

	for i, c := range l.cells {
		if laidOut.Cells[i], err = c.write(row); err != nil {
			return Row{}, err
		}
	}

	for _, link := range l.links {
		path, err := link.path.write(row)
		if err != nil {
			return Row{}, err
		}

		if path = strings.TrimSpace(path); len(path) > 0 && sites.reaches(link.site) {
			laidOut.Links[link.name] = sites.address(link.site, path)
		}
	}

	for _, a := range l.actions {
		offered, err := a.when.holds(row)
		if err != nil {
			return Row{}, err
		}

		if offered && !slices.Contains(laidOut.Actions, a.name) {
			laidOut.Actions = append(laidOut.Actions, a.name)
		}
	}

	if l.protected != nil {
		if laidOut.Protected, err = l.protected.holds(row); err != nil {
			return Row{}, err
		}
	}

	return laidOut, nil
}

// parse is a cell of a template.
func parse(format string) (*cell, error) {
	parsed, err := formatter.Parse(format)
	if err != nil {
		return nil, err
	}

	return &cell{template: parsed}, nil
}

// parseOptional is a cell of a template, none for nothing.
func parseOptional(format string) (*cell, error) {
	if len(format) == 0 {
		return nil, nil
	}

	return parse(format)
}

// holds reports whether c writes something of row that is not false: every
// row holds where there is no c.
func (c *cell) holds(row any) (bool, error) {
	if c == nil {
		return true, nil
	}

	written, err := c.write(row)
	if err != nil {
		return false, err
	}

	written = strings.TrimSpace(written)

	return len(written) > 0 && written != "false", nil
}

// withoutEmptyColumns is listing without the columns none of its rows says
// anything in, which a page of rows that do not have them shows nothing of.
func withoutEmptyColumns(listing Listing) Listing {
	if len(listing.Rows) == 0 {
		return listing
	}

	var kept []int

	for i := range listing.Columns {
		if slices.ContainsFunc(listing.Rows, func(r Row) bool { return i < len(r.Cells) && len(strings.TrimSpace(r.Cells[i])) > 0 }) {
			kept = append(kept, i)
		}
	}

	columns := make([]string, len(kept))
	for j, i := range kept {
		columns[j] = listing.Columns[i]
	}

	listing.Columns = columns

	for r, row := range listing.Rows {
		cells := make([]string, len(kept))
		for j, i := range kept {
			cells[j] = row.Cells[i]
		}

		listing.Rows[r].Cells = cells
	}

	return listing
}
