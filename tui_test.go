package main

import (
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

var testEnvs = []environment{{Name: "staging", Color: "#B9770E"}, {Name: "prod", Color: "#C0392B"}}

func TestPerPageCapsBothLists(t *testing.T) {
	m := newModel(options{envs: testEnvs, perPage: 5})
	orgs, users := make([]list.Item, 50), make([]list.Item, 50)
	for i := range 50 {
		orgs[i], users[i] = orgItem{}, userItem{}
	}
	m.orgs.SetItems(orgs)
	m.users.SetItems(users)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 60})
	m = updated.(model)

	if got := m.orgs.Paginator.PerPage; got != 5 {
		t.Errorf("%d organizations per page, want 5", got)
	}
	if got := m.users.Paginator.PerPage; got != 5 {
		t.Errorf("%d users per page, want 5", got)
	}
}

func TestPickingAnEnvironment(t *testing.T) {
	m := newModel(options{envs: testEnvs})
	for _, code := range []rune{tea.KeyDown, tea.KeyEnter} {
		updated, _ := m.Update(tea.KeyPressMsg{Code: code})
		m = updated.(model)
	}

	if m.env.Name != "prod" || m.stage != loadingOrgs {
		t.Errorf("picked %q at stage %d, want prod loading its organizations", m.env.Name, m.stage)
	}
	if m.orgs.Title != "prod · organizations" {
		t.Errorf("organizations titled %q", m.orgs.Title)
	}
}

func TestEscapeGoesBackToTheEnvironment(t *testing.T) {
	m := newModel(options{envs: testEnvs, env: "prod"})
	steps := []tea.Msg{orgsLoadedMsg{orgs: []org{{ID: "1", Name: "Acme"}}}, tea.KeyPressMsg{Code: tea.KeyEscape}}
	for _, msg := range steps {
		updated, _ := m.Update(msg)
		m = updated.(model)
	}

	if m.stage != pickingEnv || m.envCursor != 1 {
		t.Errorf("stage %d with the cursor on %d, want the picker on prod", m.stage, m.envCursor)
	}
}

func TestDisabledUsersAreHiddenUntilToggled(t *testing.T) {
	m := newModel(options{envs: testEnvs, env: "staging"})
	m.org = org{ID: "1", Name: "Acme"}
	users := usersLoadedMsg{
		{ID: "a", Email: "active@acme.test", Enabled: true},
		{ID: "d", Email: "disabled@acme.test"},
		{ID: "l", Email: "locked@acme.test", Enabled: true, Locked: true},
	}
	updated, _ := m.Update(users)
	m = updated.(model)
	if got := len(m.users.Items()); got != 1 || m.users.Title != "staging · Acme · 2 hidden" {
		t.Errorf("%d users titled %q, want only the active one and 2 hidden", got, m.users.Title)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = updated.(model)
	if got := len(m.users.Items()); got != 3 || m.users.Title != "staging · Acme" {
		t.Errorf("%d users titled %q once toggled, want all 3", got, m.users.Title)
	}
}
