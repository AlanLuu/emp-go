package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

const (
	TERM_WIDTH  = 100
	TERM_HEIGHT = 14
	TIME_FORMAT = "01/02/06 " + time.Kitchen

	STATUS_IDLE        = "idle"
	STATUS_CLOCKED_IN  = "clocked in"
	STATUS_CLOCKED_OUT = "clocked out"

	STARTING_EMPLOYEE_ID = 1
)

var _ list.DefaultItem = (*Employee)(nil)

// Single clock-in/clock-out session
type Session struct {
	ClockInAt  time.Time
	ClockOutAt time.Time
	Wage       float64
}

type Employee struct {
	ID             int
	FirstName      string
	MiddleName     string // Optional
	LastName       string
	HourlyRate     float64
	CurrentClockIn *time.Time // Current active clock-in (nil if clocked out)
	Sessions       []Session  // History of completed sessions
}

func (e *Employee) Title() string {
	// status := STATUS_IDLE
	// if e.CurrentClockIn != nil {
	// 	status = STATUS_CLOCKED_IN
	// } else if len(e.Sessions) > 0 {
	// 	status = STATUS_CLOCKED_OUT
	// }
	var status string
	if e.CurrentClockIn != nil {
		status = STATUS_CLOCKED_IN
	} else {
		status = STATUS_CLOCKED_OUT
	}
	name := e.FirstName
	if e.MiddleName != "" {
		name += " " + e.MiddleName
	}
	name += " " + e.LastName
	return fmt.Sprintf("%s (%s)", name, status)
}

func (e *Employee) Description() string {
	desc := fmt.Sprintf(
		"ID: %d | Rate: $%.2f/hr",
		e.ID,
		e.HourlyRate,
	)
	// if e.CurrentClockIn != nil {
	// 	desc += fmt.Sprintf(" | In: %s", e.CurrentClockIn.Format(TIME_FORMAT))
	// }
	// if len(e.Sessions) > 0 {
	// 	lastSession := e.Sessions[len(e.Sessions)-1]
	// 	desc += fmt.Sprintf(" | Last session: %s - %s ($%.2f)",
	// 		lastSession.ClockInAt.Format(TIME_FORMAT),
	// 		lastSession.ClockOutAt.Format(TIME_FORMAT),
	// 		lastSession.Wage)
	// 	desc += fmt.Sprintf(" | Total sessions: %d", len(e.Sessions))
	// }
	return desc
}

func (e *Employee) TotalWage() float64 {
	total := 0.0
	for _, session := range e.Sessions {
		total += session.Wage
	}
	return total
}

func (e *Employee) FilterValue() string {
	return e.Title()
}

type mode int

const (
	modeList mode = iota
	modeAdd
	modeViewSessions
	modeConfirmDelete
)

type addField int

const (
	fieldFirstName addField = iota
	fieldMiddleName
	fieldLastName
	fieldRate
)

var _ tea.Model = (*model)(nil)

// Session history list item
type SessionItem struct {
	Session    Session
	SessionNum int
}

var _ list.Item = (*SessionItem)(nil)

func (s *SessionItem) Title() string {
	return fmt.Sprintf("Session #%d: %s - %s",
		s.SessionNum,
		s.Session.ClockInAt.Format(TIME_FORMAT),
		s.Session.ClockOutAt.Format(TIME_FORMAT))
}

func (s *SessionItem) Description() string {
	hours := s.Session.ClockOutAt.Sub(s.Session.ClockInAt).Hours()
	return fmt.Sprintf(
		"Duration: %.2f hours | Wage: $%.2f",
		hours,
		s.Session.Wage,
	)
}

func (s *SessionItem) FilterValue() string {
	return s.Title()
}

type model struct {
	employees []list.Item
	nextID    int

	list             list.Model
	sessionList      list.Model // For viewing session history
	mode             mode
	field            addField
	firstName        textinput.Model
	middleName       textinput.Model
	lastName         textinput.Model
	rate             textinput.Model
	selectedEmpIdx   int // Index of employee whose sessions we're viewing
	pendingDeleteIdx int // Index of employee to delete when in modeConfirmDelete (-1 otherwise)

	lastWageMessage string

	err error
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	helpStyle  = lipgloss.NewStyle().Foreground(
		lipgloss.Color("241"),
	)
	errStyle = lipgloss.NewStyle().Foreground(
		lipgloss.Color("196"),
	)
	confirmStyle = lipgloss.NewStyle().Bold(true).Background(
		lipgloss.Color("196"),
	)
)

func newModel() *model {
	items := []list.Item{}
	l := list.New(
		items,
		list.NewDefaultDelegate(),
		TERM_WIDTH,
		TERM_HEIGHT,
	)
	l.Title = "Employees"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	fn := textinput.New()
	fn.Placeholder = "First name"
	fn.Prompt = "> "
	fn.Focus()

	mn := textinput.New()
	mn.Placeholder = "Middle name"
	mn.Prompt = "> "

	ln := textinput.New()
	ln.Placeholder = "Last name"
	ln.Prompt = "> "

	rate := textinput.New()
	rate.Placeholder = "Hourly rate (e.g. 25.50)"
	rate.Prompt = "> "

	sessionItems := []list.Item{}
	sessionList := list.New(
		sessionItems,
		list.NewDefaultDelegate(),
		TERM_WIDTH,
		TERM_HEIGHT,
	)
	sessionList.Title = "Session History"
	sessionList.SetShowStatusBar(false)
	sessionList.SetFilteringEnabled(false)
	sessionList.SetShowHelp(false)

	return &model{
		employees:        []list.Item{},
		nextID:           STARTING_EMPLOYEE_ID,
		list:             l,
		sessionList:      sessionList,
		mode:             modeList,
		field:            fieldFirstName,
		firstName:        fn,
		middleName:       mn,
		lastName:         ln,
		rate:             rate,
		selectedEmpIdx:   -1,
		pendingDeleteIdx: -1,
	}
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "a":
				m.lastWageMessage = ""
				m.mode = modeAdd
				m.field = fieldFirstName
				m.firstName.SetValue("")
				m.middleName.SetValue("")
				m.lastName.SetValue("")
				m.rate.SetValue("")
				m.firstName.Focus()
				m.middleName.Blur()
				m.lastName.Blur()
				m.rate.Blur()
				return m, nil
			case "d":
				idx := m.getSelectedIndex()
				if idx >= 0 {
					m.pendingDeleteIdx = idx
					m.mode = modeConfirmDelete
				}
				return m, nil
			case "i":
				m.clockInSelected()
				m.lastWageMessage = ""
				return m, nil
			case "o":
				m.clockOutSelected()
				return m, nil
			case "v":
				m.viewSessions()
				return m, nil
			default:
				var cmd tea.Cmd
				m.list, cmd = m.list.Update(msg)
				return m, cmd
			}

		case modeViewSessions:
			switch msg.String() {
			case "esc", "q":
				m.mode = modeList
				return m, nil
			default:
				var cmd tea.Cmd
				m.sessionList, cmd = m.sessionList.Update(msg)
				return m, cmd
			}

		case modeConfirmDelete:
			switch msg.String() {
			case "y", "enter":
				m.deleteByIndex(m.pendingDeleteIdx)
				m.pendingDeleteIdx = -1
				m.lastWageMessage = ""
				m.mode = modeList
				return m, nil
			case "n", "esc":
				m.pendingDeleteIdx = -1
				m.mode = modeList
				return m, nil
			}

		case modeAdd:
			switch msg.String() {
			case "esc":
				m.mode = modeList
				return m, nil
			case "enter", "tab":
				switch m.field {
				case fieldFirstName:
					m.field = fieldMiddleName
					m.firstName.Blur()
					m.middleName.Focus()
				case fieldMiddleName:
					m.field = fieldLastName
					m.middleName.Blur()
					m.lastName.Focus()
				case fieldLastName:
					m.field = fieldRate
					m.lastName.Blur()
					m.rate.Focus()
				case fieldRate:
					if err := m.addEmployeeFromInputs(); err != nil {
						m.err = err
					} else {
						m.mode = modeList
						m.err = nil
					}
				}
				return m, nil
			case "shift+tab":
				switch m.field {
				case fieldRate:
					m.field = fieldLastName
					m.rate.Blur()
					m.lastName.Focus()
				case fieldLastName:
					m.field = fieldMiddleName
					m.lastName.Blur()
					m.middleName.Focus()
				case fieldMiddleName:
					m.field = fieldFirstName
					m.middleName.Blur()
					m.firstName.Focus()
				}
				return m, nil
			default:
				switch m.field {
				case fieldFirstName:
					var cmd tea.Cmd
					m.firstName, cmd = m.firstName.Update(msg)
					cmds = append(cmds, cmd)
				case fieldMiddleName:
					var cmd tea.Cmd
					m.middleName, cmd = m.middleName.Update(msg)
					cmds = append(cmds, cmd)
				case fieldLastName:
					var cmd tea.Cmd
					m.lastName, cmd = m.lastName.Update(msg)
					cmds = append(cmds, cmd)
				case fieldRate:
					var cmd tea.Cmd
					m.rate, cmd = m.rate.Update(msg)
					cmds = append(cmds, cmd)
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *model) syncList() {
	m.list.SetItems(m.employees)
}

func (m *model) addEmployeeFromInputs() error {
	firstName := strings.TrimSpace(m.firstName.Value())
	middleName := strings.TrimSpace(m.middleName.Value())
	lastName := strings.TrimSpace(m.lastName.Value())
	value := strings.TrimSpace(m.rate.Value())
	if firstName == "" || lastName == "" || value == "" {
		errStr := ""
		if firstName == "" {
			errStr += "\nMissing first name"
		}
		if lastName == "" {
			errStr += "\nMissing last name"
		}
		if value == "" {
			errStr += "\nMissing hourly rate"
		}
		return errors.New(errStr[1:])
	}
	rate, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return errors.New("Hourly rate must be a numeric value.")
	}

	e := &Employee{
		ID:         m.nextID,
		FirstName:  firstName,
		MiddleName: middleName,
		LastName:   lastName,
		HourlyRate: rate,
	}
	m.nextID++
	m.employees = append(m.employees, e)
	m.syncList()
	return nil
}

func (m *model) getSelectedIndex() int {
	if len(m.employees) == 0 {
		return -1
	}
	idx := m.list.Index()
	if idx < 0 || idx >= len(m.employees) {
		return -1
	}
	return idx
}

// func (m *model) deleteSelected() {
// 	m.deleteByIndex(m.getSelectedIndex())
// }

func (m *model) deleteByIndex(idx int) {
	if idx < 0 || idx >= len(m.employees) {
		return
	}
	m.employees = append(m.employees[:idx], m.employees[idx+1:]...)
	m.syncList()
}

func (m *model) clockInSelected() {
	idx := m.getSelectedIndex()
	if idx < 0 {
		return
	}
	now := time.Now()
	switch e := m.employees[idx].(type) {
	case *Employee:
		// If already clocked in, show error
		if e.CurrentClockIn != nil {
			m.err = errors.New("Employee is already clocked in.")
			return
		}
		e.CurrentClockIn = &now
	}
	m.syncList()
}

func (m *model) clockOutSelected() {
	idx := m.getSelectedIndex()
	if idx < 0 {
		return
	}
	switch e := m.employees[idx].(type) {
	case *Employee:
		if e.CurrentClockIn == nil {
			m.err = errors.New("Employee is not clocked in yet.")
			return
		}
		now := time.Now()
		hours := now.Sub(*e.CurrentClockIn).Hours()
		wage := hours * e.HourlyRate

		// Create and save the session
		session := Session{
			ClockInAt:  *e.CurrentClockIn,
			ClockOutAt: now,
			Wage:       wage,
		}
		e.Sessions = append(e.Sessions, session)

		// Clear current clock-in
		e.CurrentClockIn = nil

		name := e.FirstName
		if e.MiddleName != "" {
			name += " " + e.MiddleName
		}
		name += " " + e.LastName
		m.lastWageMessage = fmt.Sprintf(
			"%s clocked out — Session wage: $%.2f",
			name,
			wage,
		)
	}
	m.syncList()
}

func (m *model) viewSessions() {
	idx := m.getSelectedIndex()
	if idx < 0 {
		m.err = errors.New("No employee selected.")
		return
	}

	switch e := m.employees[idx].(type) {
	case *Employee:
		m.selectedEmpIdx = idx
		items := make([]list.Item, len(e.Sessions))
		for i, session := range e.Sessions {
			items[len(e.Sessions)-1-i] = &SessionItem{
				Session:    session,
				SessionNum: len(e.Sessions) - i, // Show most recent first
			}
		}
		m.sessionList.SetItems(items)
		m.mode = modeViewSessions
	}
}

func (m *model) View() string {
	defer func() {
		m.err = nil
	}()
	switch m.mode {
	case modeList:
		header := titleStyle.Render("Employee Management System") + "\n\n"
		body := m.list.View()
		help := helpStyle.Render(
			"\n[a] add  [d] delete  [i] clock in  [o] clock out  [v] view sessions  [q] quit",
		)
		if m.err != nil {
			help += "\n" + errStyle.Render(m.err.Error())
		}
		if m.lastWageMessage != "" {
			help += "\n" + titleStyle.Render(m.lastWageMessage)
		}
		return header + body + help + "\n"

	case modeAdd:
		var builder strings.Builder
		builder.WriteString(titleStyle.Render("Add Employee") + "\n\n")
		builder.WriteString("First name*:\n" + m.firstName.View() + "\n\n")
		builder.WriteString("Middle name:\n" + m.middleName.View() + "\n\n")
		builder.WriteString("Last name*:\n" + m.lastName.View() + "\n\n")
		builder.WriteString("Hourly rate*:\n" + m.rate.View() + "\n\n")
		builder.WriteString("* = required field" + "\n\n")
		builder.WriteString(
			helpStyle.Render(
				"[enter/tab] next [shift+tab] previous [esc] cancel",
			),
		)
		if m.err != nil {
			builder.WriteString("\n" + errStyle.Render(m.err.Error()))
		}
		return builder.String() + "\n"

	case modeViewSessions:
		var builder strings.Builder
		if m.selectedEmpIdx >= 0 && m.selectedEmpIdx < len(m.employees) {
			switch e := m.employees[m.selectedEmpIdx].(type) {
			case *Employee:
				name := e.FirstName
				if e.MiddleName != "" {
					name += " " + e.MiddleName
				}
				name += " " + e.LastName
				builder.WriteString(
					titleStyle.Render(
						fmt.Sprintf("Session History: %s", name),
					) + "\n",
				)
				fmt.Fprintf(
					&builder,
					"ID: %d | Rate: $%.2f/hr | Total Sessions: %d | Total Wage: $%.2f\n\n",
					e.ID,
					e.HourlyRate,
					len(e.Sessions),
					e.TotalWage(),
				)
			}
		}
		builder.WriteString(m.sessionList.View())
		builder.WriteString("\n" + helpStyle.Render("[esc/q] back"))
		return builder.String() + "\n"

	case modeConfirmDelete:
		var name string
		if m.pendingDeleteIdx >= 0 && m.pendingDeleteIdx < len(m.employees) {
			switch e := m.employees[m.pendingDeleteIdx].(type) {
			case *Employee:
				name = e.FirstName
				if e.MiddleName != "" {
					name += " " + e.MiddleName
				}
				name += " " + e.LastName
			}
		}
		if name == "" {
			name = "selected employee"
		}
		prompt := fmt.Sprintf("Delete %s? (y/n)", name)
		return confirmStyle.Render("Confirm Delete") + "\n\n" +
			prompt + "\n\n" +
			helpStyle.Render("[y/enter] yes  [n/esc] no") + "\n"
	}

	return ""
}

func main() {
	if !term.IsTerminal(os.Stdin.Fd()) {
		fmt.Fprintln(os.Stderr, "not a terminal")
		os.Exit(1)
	}
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error running program: %v\n", err)
		os.Exit(1)
	}
}
