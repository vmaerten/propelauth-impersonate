package main

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type stage int

const (
	pickingEnv stage = iota
	loadingOrgs
	pickingOrg
	loadingUsers
	pickingUser
	impersonating
)

type orgsLoadedMsg struct {
	client *propelAuth
	orgs   []org
}

type usersLoadedMsg []user

type impersonatedMsg struct{}

type errMsg struct{ err error }

type orgItem org

func (o orgItem) Title() string       { return o.Name }
func (o orgItem) Description() string { return "" }
func (o orgItem) FilterValue() string { return o.Name }

type userItem struct {
	user
	role string
}

func (u userItem) Title() string { return u.Email }

func (u userItem) Description() string {
	description := strings.TrimSpace(u.FirstName+" "+u.LastName) + " · " + u.role
	if !u.Enabled {
		description += " · disabled"
	}
	if u.Locked {
		description += " · locked"
	}
	return description
}

func (u userItem) FilterValue() string { return u.Email + " " + u.FirstName + " " + u.LastName }

type model struct {
	envs         []environment
	env          environment
	awsProfile   string
	query        string
	perPage      int
	showDisabled bool
	chrome       chromeProfile
	client       *propelAuth
	stage        stage
	spinner      spinner.Model
	envCursor    int
	orgs         list.Model
	users        list.Model
	org          org
	orgUsers     []user
	user         user
	done         string
	err          error
}

var toggleDisabled = key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "show/hide disabled"))

type options struct {
	envs []environment
	// env names the environment to start on, the picker opening when it is empty.
	env          string
	awsProfile   string
	query        string
	perPage      int
	showDisabled bool
	chrome       chromeProfile
}

func newModel(o options) model {
	orgDelegate := list.NewDefaultDelegate()
	orgDelegate.ShowDescription = false
	orgDelegate.SetSpacing(0)
	m := model{
		envs:         o.envs,
		awsProfile:   o.awsProfile,
		query:        o.query,
		perPage:      o.perPage,
		showDisabled: o.showDisabled,
		chrome:       o.chrome,
		spinner:      spinner.New(spinner.WithSpinner(spinner.Dot)),
		orgs:         newList("organization", orgDelegate),
		users:        newList("user", list.NewDefaultDelegate(), toggleDisabled),
	}
	if i := findEnv(o.envs, o.env); i >= 0 {
		m = m.withEnv(o.envs[i])
	}
	return m
}

func newList(itemName string, delegate list.ItemDelegate, help ...key.Binding) list.Model {
	l := list.New(nil, delegate, 0, 0)
	l.SetStatusBarItemName(itemName, itemName+"s")
	l.KeyMap.Quit = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))
	back := key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))
	l.AdditionalShortHelpKeys = func() []key.Binding { return append([]key.Binding{back}, help...) }
	return l
}

func (m model) withEnv(env environment) model {
	m.env = env
	m.envCursor = findEnv(m.envs, env.Name)
	m.stage = loadingOrgs
	m.orgs.Title = env.Name + " · organizations"
	m.orgs.Styles.Title = m.orgs.Styles.Title.Background(lipgloss.Color(env.Color))
	m.users.Styles.Title = m.users.Styles.Title.Background(lipgloss.Color(env.Color))
	return m
}

// fitList sizes l to the window, shortened until a page holds at most perPage items (0 fills it).
func fitList(l *list.Model, width, height, perPage int) {
	l.SetSize(width, height)
	for perPage > 0 && l.Paginator.PerPage > perPage {
		height--
		l.SetSize(width, height)
	}
}

func (m model) Init() tea.Cmd {
	if m.stage == pickingEnv {
		return m.spinner.Tick
	}
	return tea.Batch(m.spinner.Tick, loadOrgs(m.env, m.awsProfile))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		fitList(&m.orgs, msg.Width, msg.Height, m.perPage)
		fitList(&m.users, msg.Width, msg.Height, m.perPage)
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case errMsg:
		m.err = msg.err
		return m, tea.Quit
	case orgsLoadedMsg:
		m.client = msg.client
		items := make([]list.Item, len(msg.orgs))
		for i, o := range msg.orgs {
			items[i] = orgItem(o)
		}
		cmd := m.orgs.SetItems(items)
		m.stage = pickingOrg
		if m.query != "" {
			m.orgs.SetFilterText(m.query)
			if len(m.orgs.VisibleItems()) == 1 {
				return m.pickOrg()
			}
		}
		return m, cmd
	case usersLoadedMsg:
		m.orgUsers = msg
		m.users.ResetFilter()
		m.stage = pickingUser
		return m.listUsers()
	case impersonatedMsg:
		who := m.user.Email + " (" + m.org.Name + ") on " + m.env.Name
		m.done = "Impersonating " + who
		if m.chrome.dir != "" {
			m.done = "Opened the impersonation of " + who + " in the Chrome profile " + m.chrome.name + ": click Impersonate there"
		}
		return m, tea.Quit
	}

	switch m.stage {
	case pickingEnv:
		return m.updateEnvs(msg)
	case pickingOrg:
		return m.updateOrgs(msg)
	case pickingUser:
		return m.updateUsers(msg)
	}
	return m, nil
}

func (m model) updateEnvs(msg tea.Msg) (tea.Model, tea.Cmd) {
	press, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch press.String() {
	case "up", "k":
		m.envCursor = max(m.envCursor-1, 0)
	case "down", "j":
		m.envCursor = min(m.envCursor+1, len(m.envs)-1)
	case "enter":
		m = m.withEnv(m.envs[m.envCursor])
		return m, loadOrgs(m.env, m.awsProfile)
	case "q", "esc":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateOrgs(msg tea.Msg) (tea.Model, tea.Cmd) {
	if press, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case press.String() == "enter" && m.orgs.SelectedItem() != nil:
			return m.pickOrg()
		case press.String() == "esc" && m.orgs.FilterState() == list.Unfiltered:
			m.stage = pickingEnv
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.orgs, cmd = m.orgs.Update(msg)
	return m, cmd
}

func (m model) pickOrg() (tea.Model, tea.Cmd) {
	m.org = org(m.orgs.SelectedItem().(orgItem))
	m.stage = loadingUsers
	return m, loadUsers(m.client, m.org.ID)
}

func (m model) updateUsers(msg tea.Msg) (tea.Model, tea.Cmd) {
	if press, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case press.String() == "enter" && m.users.SelectedItem() != nil:
			m.user = m.users.SelectedItem().(userItem).user
			m.stage = impersonating
			return m, impersonateUser(m.env, m.user, m.chrome)
		case press.String() == "esc" && m.users.FilterState() == list.Unfiltered:
			m.stage = pickingOrg
			return m, nil
		case key.Matches(press, toggleDisabled) && m.users.FilterState() != list.Filtering:
			m.showDisabled = !m.showDisabled
			return m.listUsers()
		}
	}
	var cmd tea.Cmd
	m.users, cmd = m.users.Update(msg)
	return m, cmd
}

// listUsers fills the user list with the users of the org, the disabled and locked ones only
// when shown.
func (m model) listUsers() (tea.Model, tea.Cmd) {
	var items []list.Item
	for _, u := range m.orgUsers {
		if m.showDisabled || (u.Enabled && !u.Locked) {
			items = append(items, userItem{user: u, role: u.Orgs[m.org.ID].Role})
		}
	}
	m.users.Title = m.env.Name + " · " + m.org.Name
	if hidden := len(m.orgUsers) - len(items); hidden > 0 {
		m.users.Title += fmt.Sprintf(" · %d hidden", hidden)
	}
	return m, m.users.SetItems(items)
}

func (m model) View() tea.View {
	var content string
	switch m.stage {
	case pickingEnv:
		content = m.envsView()
	case loadingOrgs:
		content = m.status("Loading the " + m.env.Name + " organizations")
	case pickingOrg:
		content = m.orgs.View()
	case loadingUsers:
		content = m.status("Loading the users of " + m.org.Name)
	case pickingUser:
		content = m.users.View()
	case impersonating:
		text := "Impersonating " + m.user.Email + ", sign in to the dashboard in Chrome if it asks"
		if m.chrome.dir != "" {
			text = "Opening the impersonation of " + m.user.Email + " in the Chrome profile " + m.chrome.name
		}
		content = m.status(text)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

var (
	envStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Padding(0, 1)
	helpStyle = lipgloss.NewStyle().Faint(true)
)

func (m model) envsView() string {
	var b strings.Builder
	b.WriteString("\n  Environment\n\n")
	for i, env := range m.envs {
		cursor := "  "
		if i == m.envCursor {
			cursor = "> "
		}
		b.WriteString("  " + cursor + envStyle.Background(lipgloss.Color(env.Color)).Render(env.Name) + "\n")
	}
	b.WriteString("\n  " + helpStyle.Render("↑/↓ choose • enter confirm • q quit") + "\n")
	return b.String()
}

func (m model) status(text string) string {
	return "\n  " + m.spinner.View() + " " + text + "\n"
}

func loadOrgs(env environment, awsProfile string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := newPropelAuth(ctx, env, awsProfile)
		if err != nil {
			return errMsg{err}
		}
		orgs, err := client.orgs(ctx)
		if err != nil {
			return errMsg{err}
		}
		return orgsLoadedMsg{client: client, orgs: orgs}
	}
}

func loadUsers(client *propelAuth, orgID string) tea.Cmd {
	return func() tea.Msg {
		users, err := client.usersInOrg(context.Background(), orgID)
		if err != nil {
			return errMsg{err}
		}
		return usersLoadedMsg(users)
	}
}

func impersonateUser(env environment, u user, chrome chromeProfile) tea.Cmd {
	return func() tea.Msg {
		if err := openImpersonation(context.Background(), env, u.ID, chrome); err != nil {
			return errMsg{err}
		}
		return impersonatedMsg{}
	}
}
