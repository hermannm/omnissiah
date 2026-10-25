package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"hermannm.dev/wrap"

	"hermannm.dev/omnissiah/internal/color"

	"hermannm.dev/omnissiah/config"

	"hermannm.dev/omnissiah/internal/sliceutil"
	"hermannm.dev/omnissiah/sbx"
)

func RenderMachineSpiritPicker(
	ctx context.Context,
	machineSpirits []sbx.Sandbox,
	actions []PickerAction,
	defaultAction string,
) error {
	items, err := initPickerItems(machineSpirits)
	if err != nil {
		return wrap.Error(err, "failed to initialize picker items")
	}

	itemDelegate := initPickerItemDelegate(actions, defaultAction)

	model := initPickerModel(ctx, items, itemDelegate, actions, defaultAction)

	_, err = tea.NewProgram(model, tea.WithContext(ctx)).Run()
	if err != nil {
		return err
	}

	if model.selectedSpirit != nil && model.selectedAction != nil {
		if err := model.selectedAction.Action(ctx, model.selectedSpirit); err != nil {
			return wrap.Errorf(err, "%s failed", model.selectedSpirit.Name)
		}
	}

	return nil
}

type PickerAction struct {
	Name                string
	Key                 string
	Background          bool
	RequireConfirmation bool //exhaustruct:optional
	// Return [ErrRemovePickerItem] to remove the item from the picker after the action.
	Action func(ctx context.Context, spirit *sbx.Sandbox) error
}

var ErrRemovePickerItem = errors.New("remove picker item")

type pickerModel struct {
	//nolint:containedctx // Only way to propagate context through bubbletea
	ctx            context.Context
	list           list.Model
	actions        []PickerAction
	defaultAction  string
	selectedSpirit *sbx.Sandbox
	selectedAction *PickerAction
}

func initPickerModel(
	ctx context.Context,
	items []list.Item,
	itemDelegate list.ItemDelegate,
	actions []PickerAction,
	defaultAction string,
) *pickerModel {
	m := pickerModel{
		ctx:            ctx,
		list:           list.New(items, itemDelegate, 0, 0),
		actions:        actions,
		defaultAction:  defaultAction,
		selectedSpirit: nil,
		selectedAction: nil,
	}

	m.list.Title = "Machine spirits"
	m.list.SetStatusBarItemName("spirit", "spirits")

	m.list.Styles.Title = m.list.Styles.Title.
		Foreground(ansiColorBlack).
		Background(ansiColorBlue)
	m.list.Styles.StatusBar = m.list.Styles.StatusBar.Foreground(ansiColorGray)
	m.list.Help.Styles.ShortKey = m.list.Help.Styles.ShortKey.Foreground(ansiColorWhite)
	m.list.Help.Styles.FullKey = m.list.Help.Styles.ShortKey
	m.list.Help.Styles.ShortDesc = m.list.Help.Styles.ShortDesc.Foreground(ansiColorGray)
	m.list.Help.Styles.FullDesc = m.list.Help.Styles.ShortDesc
	m.list.Help.Styles.ShortSeparator = m.list.Help.Styles.ShortDesc
	m.list.Help.Styles.FullSeparator = m.list.Help.Styles.ShortSeparator

	m.list.KeyMap.CursorUp.SetKeys("up")
	m.list.KeyMap.CursorUp.SetHelp("↑", "up")
	m.list.KeyMap.CursorDown.SetKeys("down")
	m.list.KeyMap.CursorDown.SetHelp("↓", "down")
	m.list.DisableQuitKeybindings()

	return &m
}

func (m *pickerModel) Init() tea.Cmd {
	return nil
}

var docStyle = lipgloss.NewStyle().Margin(1, 2)

// https://en.wikipedia.org/wiki/ANSI_escape_code#Colors
//
//nolint:unused // For future use
//goland:noinspection GoUnusedGlobalVariable
var (
	ansiColorBlack    = lipgloss.ANSIColor(0)
	ansiColorRed      = lipgloss.ANSIColor(1)
	ansiColorGreen    = lipgloss.ANSIColor(2)
	ansiColorYellow   = lipgloss.ANSIColor(3)
	ansiColorBlue     = lipgloss.ANSIColor(4)
	ansiColorMagenta  = lipgloss.ANSIColor(5)
	ansiColorCyan     = lipgloss.ANSIColor(6)
	ansiColorGray     = lipgloss.ANSIColor(7)
	ansiColorDarkGray = lipgloss.ANSIColor(8)
	ansiColorWhite    = lipgloss.ANSIColor(15)
)

func (m *pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		pressedKey := msg.String()
		if pressedKey == "ctrl+c" {
			return m, tea.Quit
		}

		if m.list.FilterState() == list.Filtering {
			break
		}

		if pressedKey == "esc" {
			if _, isConfirming := m.getConfirmingAction(); isConfirming {
				m.selectedItem().clearConfirmation()
				return m, nil
			}

			return m, tea.Quit
		}

		if pressedKey == "enter" {
			if confirmingAction, ok := m.getConfirmingAction(); ok {
				m.selectedItem().clearConfirmation()
				cmd := m.executeAction(confirmingAction, true)
				return m, cmd
			}

			defaultAction, ok := sliceutil.Find(
				m.actions,
				func(action PickerAction) bool {
					return action.Name == m.defaultAction
				},
			)
			if ok {
				cmd := m.executeAction(defaultAction, false)
				return m, cmd
			}
		}

		for _, action := range m.actions {
			if pressedKey == action.Key {
				cmd := m.executeAction(action, false)
				return m, cmd
			}
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *pickerModel) executeAction(action PickerAction, confirmed bool) tea.Cmd {
	selectedItem := m.selectedItem()

	if action.RequireConfirmation && !confirmed {
		selectedItem.setConfirmingAction(action)
		return nil
	}

	if action.Background {
		// TODO: Execute background work
		return nil
	} else {
		m.selectedAction = &action
		m.selectedSpirit = selectedItem.Spirit
		return tea.Quit
	}
}

func (m *pickerModel) getConfirmingAction() (action PickerAction, ok bool) {
	selectedItem := m.selectedItem()
	if selectedItem.ConfirmingAction != nil {
		return selectedItem.ConfirmingAction.PickerAction, true
	} else {
		return PickerAction{}, false
	}
}

func (m *pickerModel) selectedItem() *PickerItem {
	//nolint:errcheck // All model items are PickerItem
	return m.list.SelectedItem().(*PickerItem)
}

func (m *pickerModel) View() tea.View {
	v := tea.NewView(docStyle.Render(m.list.View()))
	v.AltScreen = true
	return v
}

type PickerItem struct {
	Spirit           *sbx.Sandbox
	Age              string
	PrimaryWorkspace string // Optional.
	Info             string
	ConfirmingAction *ConfirmingAction
}

type ConfirmingAction struct {
	PickerAction
	Prompt string
}

func initPickerItems(spirits []sbx.Sandbox) ([]list.Item, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil, wrap.Error(err, "failed to get user home dir")
	}

	now := time.Now()

	items := sliceutil.Map(
		spirits,
		func(spirit sbx.Sandbox) list.Item {
			age := formatAge(spirit, now)

			var primaryWorkspace string
			if len(spirit.Workspaces) != 0 {
				primaryWorkspace = config.CollapseHomedir(spirit.Workspaces[0], homedir)
			}

			return &PickerItem{
				Spirit:           &spirit,
				Age:              age,
				PrimaryWorkspace: primaryWorkspace,
				Info:             makePickerItemInfo(spirit, primaryWorkspace, age),
				ConfirmingAction: nil,
			}
		},
	)
	return items, nil
}

func makePickerItemInfo(spirit sbx.Sandbox, primaryWorkspace string, age string) string {
	var builder strings.Builder

	builder.WriteString(age)

	if primaryWorkspace != "" {
		builder.WriteString(" • ")
		builder.WriteString(primaryWorkspace)
	}

	builder.WriteString(" • ")

	switch spirit.Status {
	case sbx.StatusRunning:
		builder.WriteString(color.Green.On("awake"))
	case sbx.StatusStopped:
		builder.WriteString("sleeping")
	}

	return builder.String()
}

func formatAge(spirit sbx.Sandbox, now time.Time) string {
	age := now.Sub(spirit.CreatedAt)

	hours := int64(age.Hours())
	if hours >= 24 {
		days := hours / 24
		return fmt.Sprintf("%dd", days)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}

	minutes := int64(age.Minutes())
	return fmt.Sprintf("%dm", minutes)
}

func (i *PickerItem) setConfirmingAction(action PickerAction) {
	var prompt strings.Builder

	color.Yellow.Set(&prompt)

	if len(action.Name) > 0 {
		prompt.WriteString(strings.ToTitle(action.Name[0:1]))
		if len(action.Name) > 1 {
			prompt.WriteString(action.Name[1:])
		}
	}

	prompt.WriteByte(' ')
	prompt.WriteString(i.Spirit.Name)
	prompt.WriteByte('?')
	prompt.WriteByte(' ')

	color.Gray.Set(&prompt)
	prompt.WriteString("[enter/esc]")
	color.Reset(&prompt)

	i.ConfirmingAction = &ConfirmingAction{
		PickerAction: action,
		Prompt:       prompt.String(),
	}
}

func (i *PickerItem) clearConfirmation() {
	i.ConfirmingAction = nil
}

func (i *PickerItem) Title() string {
	if i.ConfirmingAction != nil {
		return i.ConfirmingAction.Prompt
	} else {
		return i.Spirit.Name
	}
}

func (i *PickerItem) Description() string {
	return i.Info
}

func (i *PickerItem) FilterValue() string {
	return i.Spirit.Name
}

func initPickerItemDelegate(actions []PickerAction, defaultAction string) list.ItemDelegate {
	delegate := list.NewDefaultDelegate()

	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(ansiColorRed).
		BorderLeftForeground(ansiColorRed)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Foreground(ansiColorGray)
	delegate.Styles.NormalDesc = delegate.Styles.NormalTitle

	keyBindings := sliceutil.Map(
		actions,
		func(action PickerAction) key.Binding {
			if action.Name == defaultAction {
				return key.NewBinding(
					key.WithKeys(action.Key, "enter"),
					key.WithHelp(action.Key+"/enter", action.Name),
				)
			} else {
				return key.NewBinding(
					key.WithKeys(action.Key),
					key.WithHelp(action.Key, action.Name),
				)
			}
		},
	)
	delegate.ShortHelpFunc = func() []key.Binding {
		return keyBindings
	}
	delegate.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{keyBindings}
	}

	return delegate
}
