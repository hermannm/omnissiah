package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"hermannm.dev/wrap"

	"hermannm.dev/omnissiah/config"

	"hermannm.dev/omnissiah/internal/sliceutil"
	"hermannm.dev/omnissiah/sbx"
)

type PickerAction struct {
	Name         string
	Key          string
	Asynchronous bool
	// Set this to show a Yes/No confirmation before performing the Action.
	// Returns a list of strings for multiple lines.
	//exhaustruct:optional
	Confirmation func(item PickerItem) []string
	Action       func(ctx context.Context, spirit sbx.Sandbox) (*sbx.Sandbox, error)
}

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

	model := initPickerModel(ctx, items, itemDelegate)

	_, err = tea.NewProgram(model, tea.WithContext(ctx)).Run()
	if err != nil {
		return err
	}

	return nil
}

type pickerModel struct {
	ctx  context.Context
	list list.Model
}

func initPickerModel(
	ctx context.Context,
	items []list.Item,
	itemDelegate list.ItemDelegate,
) pickerModel {
	model := pickerModel{
		ctx:  ctx,
		list: list.New(items, itemDelegate, 0, 0),
	}

	model.list.Title = "Machine spirits"
	model.list.SetStatusBarItemName("spirit", "spirits")

	model.list.Styles.Title = model.list.Styles.Title.
		Foreground(ansiColorBlack).
		Background(ansiColorBlue)

	model.list.KeyMap.CursorUp.SetKeys("up")
	model.list.KeyMap.CursorUp.SetHelp("↑", "up")
	model.list.KeyMap.CursorDown.SetKeys("down")
	model.list.KeyMap.CursorDown.SetHelp("↓", "down")
	model.list.DisableQuitKeybindings()

	return model
}

func (m pickerModel) Init() tea.Cmd {
	return nil
}

var docStyle = lipgloss.NewStyle().Margin(1, 2)

// https://en.wikipedia.org/wiki/ANSI_escape_code#Colors
var (
	ansiColorBlack = lipgloss.ANSIColor(0)
	ansiColorRed   = lipgloss.ANSIColor(1)
	ansiColorBlue  = lipgloss.ANSIColor(4)
)

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m pickerModel) View() tea.View {
	v := tea.NewView(docStyle.Render(m.list.View()))
	v.AltScreen = true
	return v
}

type PickerItem struct {
	Spirit           sbx.Sandbox
	Age              string
	PrimaryWorkspace string // Optional.
	Info             string
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

			return PickerItem{
				Spirit:           spirit,
				Age:              age,
				PrimaryWorkspace: primaryWorkspace,
				Info:             makePickerItemInfo(spirit, primaryWorkspace, age),
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
		builder.WriteString("awake")
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

func (i PickerItem) Title() string {
	return i.Spirit.Name
}

func (i PickerItem) Description() string {
	return i.Info
}

func (i PickerItem) FilterValue() string {
	return i.Spirit.Name
}

func initPickerItemDelegate(actions []PickerAction, defaultAction string) list.ItemDelegate {
	delegate := list.NewDefaultDelegate()

	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(ansiColorRed).
		BorderLeftForeground(ansiColorRed)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle

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
