package tui

import (
	"context"
	"errors"
	"fmt"
	"iter"
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
	homedir, err := os.UserHomeDir()
	if err != nil {
		return wrap.Error(err, "failed to get user home dir")
	}

	items := initPickerItems(machineSpirits, homedir)

	itemDelegate := initPickerItemDelegate(actions, defaultAction)

	model := initPickerModel(ctx, items, itemDelegate, actions, defaultAction, homedir)

	_, err = tea.NewProgram(model, tea.WithContext(ctx)).Run()
	if err != nil {
		return err
	}
	if model.err != nil {
		return model.err
	}

	if model.selectedSpirit != nil && model.selectedAction != nil {
		if _, err := model.selectedAction.Action(ctx, *model.selectedSpirit); err != nil {
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
	// Return [ErrRemovePickerItem] (and the spirit) to remove the item from the picker after the
	// action.
	Action func(ctx context.Context, spirit sbx.Sandbox) (sbx.Sandbox, error)
}

var ErrRemovePickerItem = errors.New("remove picker item")

type pickerModel struct {
	//nolint:containedctx // Only way to propagate context through bubbletea
	ctx                  context.Context
	list                 list.Model
	actions              []PickerAction
	defaultAction        string
	homedir              string
	backgroundActionChan chan backgroundActionResult
	selectedSpirit       *sbx.Sandbox  // Optional.
	selectedAction       *PickerAction // Optional.
	// Optional. Used to return an error after quitting the Bubbletea program, as suggested here:
	// https://github.com/charmbracelet/bubbletea/discussions/914#discussioncomment-8569719
	err error
}

type backgroundActionResult struct {
	spirit sbx.Sandbox
	err    error
}

func initPickerModel(
	ctx context.Context,
	items []list.Item,
	itemDelegate list.ItemDelegate,
	actions []PickerAction,
	defaultAction string,
	homedir string,
) *pickerModel {
	m := pickerModel{
		ctx:                  ctx,
		list:                 list.New(items, itemDelegate, 0, 0),
		actions:              actions,
		defaultAction:        defaultAction,
		homedir:              homedir,
		backgroundActionChan: make(chan backgroundActionResult),
		selectedSpirit:       nil,
		selectedAction:       nil,
		err:                  nil,
	}

	m.list.Title = "The Omnissiah"
	m.list.SetStatusBarItemName("machine spirit", "machine spirits")

	m.list.Styles.Title = m.list.Styles.Title.Foreground(colorBlack).Background(colorBlue)
	m.list.Styles.StatusBar = m.list.Styles.StatusBar.Foreground(colorGray)
	m.list.Styles.StatusEmpty = m.list.Styles.StatusEmpty.Foreground(colorGray)
	// Hide "No items" display, since it duplicates status bar
	m.list.Styles.NoItems = m.list.Styles.NoItems.Foreground(colorBlack)
	m.list.Styles.Spinner = m.list.Styles.Spinner.Foreground(colorRed)
	m.list.Styles.StatusBarFilterCount = m.list.Styles.StatusBarFilterCount.Foreground(colorGray)

	m.list.Help.Styles.ShortKey = m.list.Help.Styles.ShortKey.Foreground(colorWhite)
	m.list.Help.Styles.FullKey = m.list.Help.Styles.ShortKey
	m.list.Help.Styles.ShortDesc = m.list.Help.Styles.ShortDesc.Foreground(colorGray)
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
	colorBlack   = lipgloss.ANSIColor(0)
	colorRed     = lipgloss.ANSIColor(1)
	colorGreen   = lipgloss.ANSIColor(2)
	colorYellow  = lipgloss.ANSIColor(3)
	colorBlue    = lipgloss.ANSIColor(4)
	colorMagenta = lipgloss.ANSIColor(5)
	colorCyan    = lipgloss.ANSIColor(6)
	colorGray    = lipgloss.ANSIColor(7)
	colorWhite   = lipgloss.ANSIColor(15)
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
			if item, ok := m.getItemWithConfirmingAction(); ok {
				item.clearConfirmation()
				return m, nil
			}

			return m, tea.Quit
		}

		if pressedKey == "enter" {
			if item, ok := m.getItemWithConfirmingAction(); ok {
				action := item.ConfirmingAction.PickerAction
				item.clearConfirmation()
				cmd := m.executeAction(action, true)
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
	case backgroundActionResult:
		m.list.StopSpinner()

		if msg.err != nil {
			if errors.Is(msg.err, ErrRemovePickerItem) {
				for i, item := range m.items() {
					if item.Spirit.Name == msg.spirit.Name {
						m.list.RemoveItem(i)
						break
					}
				}
				return m, nil
			}

			m.err = msg.err
			return m, tea.Quit
		}

		updatedSpirit := msg.spirit
		for i, item := range m.items() {
			if item.Spirit.Name == updatedSpirit.Name {
				m.list.SetItem(i, makePickerItem(updatedSpirit, time.Now(), m.homedir))
			}
		}
		return m, nil
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *pickerModel) executeAction(action PickerAction, confirmed bool) tea.Cmd {
	selectedItem, ok := m.selectedItem()
	if !ok {
		return nil
	}

	if action.RequireConfirmation && !confirmed {
		selectedItem.setConfirmingAction(action)
		return nil
	}

	if action.Background {
		return tea.Batch(
			m.list.StartSpinner(),
			// Pattern for asynchronous messages in Bubbletea:
			// https://github.com/charmbracelet/bubbletea/discussions/305#discussioncomment-2650714
			executeBackgroundAction(m.ctx, action, selectedItem.Spirit, m.backgroundActionChan),
			listenForBackgroundActionResult(m.ctx, m.backgroundActionChan),
		)
	} else {
		m.selectedAction = &action
		m.selectedSpirit = &selectedItem.Spirit
		return tea.Quit
	}
}

func executeBackgroundAction(
	ctx context.Context,
	action PickerAction,
	spirit sbx.Sandbox,
	resultChan chan<- backgroundActionResult,
) tea.Cmd {
	return func() tea.Msg {
		updatedSpirit, err := action.Action(ctx, spirit)
		select {
		case resultChan <- backgroundActionResult{spirit: updatedSpirit, err: err}:
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}

func listenForBackgroundActionResult(
	ctx context.Context,
	resultChan <-chan backgroundActionResult,
) tea.Cmd {
	return func() tea.Msg {
		select {
		case result := <-resultChan:
			return result
		case <-ctx.Done():
			return nil
		}
	}
}

func (m *pickerModel) getItemWithConfirmingAction() (item *PickerItem, ok bool) {
	selectedItem, ok := m.selectedItem()
	if ok && selectedItem.ConfirmingAction != nil {
		return selectedItem, true
	} else {
		return nil, false
	}
}

func (m *pickerModel) selectedItem() (item *PickerItem, ok bool) {
	item, ok = m.list.SelectedItem().(*PickerItem)
	return item, ok
}

func (m *pickerModel) items() iter.Seq2[int, *PickerItem] {
	return func(yield func(int, *PickerItem) bool) {
		for i, item := range m.list.Items() {
			pickerItem, ok := item.(*PickerItem)
			if !ok {
				continue
			}
			if !yield(i, pickerItem) {
				break
			}
		}
	}
}

func (m *pickerModel) View() tea.View {
	v := tea.NewView(docStyle.Render(m.list.View()))
	v.AltScreen = true
	return v
}

type PickerItem struct {
	Spirit           sbx.Sandbox
	Age              string
	PrimaryWorkspace string // Optional.
	Info             string
	ConfirmingAction *ConfirmingAction
}

type ConfirmingAction struct {
	PickerAction
	Prompt string
}

func initPickerItems(spirits []sbx.Sandbox, homedir string) []list.Item {
	now := time.Now()

	return sliceutil.Map(
		spirits,
		func(spirit sbx.Sandbox) list.Item {
			return makePickerItem(spirit, now, homedir)
		},
	)
}

func makePickerItem(spirit sbx.Sandbox, now time.Time, homedir string) *PickerItem {
	age := formatAge(spirit, now)

	var primaryWorkspace string
	if len(spirit.Workspaces) != 0 {
		primaryWorkspace = config.CollapseHomedir(spirit.Workspaces[0], homedir)
	}

	return &PickerItem{
		Spirit:           spirit,
		Age:              age,
		PrimaryWorkspace: primaryWorkspace,
		Info:             makePickerItemInfo(spirit, primaryWorkspace, age),
		ConfirmingAction: nil,
	}
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
		Foreground(colorRed).
		BorderLeftForeground(colorRed)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Foreground(colorGray)
	delegate.Styles.NormalDesc = delegate.Styles.NormalTitle
	delegate.Styles.DimmedTitle = delegate.Styles.NormalTitle
	delegate.Styles.DimmedDesc = delegate.Styles.NormalDesc

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
