package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"hermannm.dev/wrap"

	"hermannm.dev/omnissiah/config"
	"hermannm.dev/omnissiah/internal/sliceutil"
	"hermannm.dev/omnissiah/sbx"
)

func RenderMachineSpiritPicker(machineSpirits []sbx.Sandbox) error {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return wrap.Error(err, "failed to get user home dir")
	}

	now := time.Now()
	items := sliceutil.Map(
		machineSpirits,
		func(spirit sbx.Sandbox) list.Item {
			return pickerItem{
				spirit: spirit,
				info:   makePickerItemInfo(spirit, homedir, now),
			}
		},
	)

	itemDelegate := list.NewDefaultDelegate()
	itemDelegate.Styles.SelectedTitle = itemDelegate.Styles.SelectedTitle.
		Foreground(ansiColorRed).
		BorderLeftForeground(ansiColorRed)
	itemDelegate.Styles.SelectedDesc = itemDelegate.Styles.SelectedDesc.
		Foreground(ansiColorRed).
		BorderLeftForeground(ansiColorRed)

	model := pickerModel{list: list.New(items, itemDelegate, 0, 0)}
	model.list.Title = "Machine spirits"
	model.list.SetStatusBarItemName("spirit", "spirits")
	model.list.Styles.Title = model.list.Styles.Title.
		Foreground(ansiColorBlack).
		Background(ansiColorBlue)

	_, err = tea.NewProgram(model).Run()
	if err != nil {
		return err
	}

	return nil
}

type pickerModel struct {
	list list.Model
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
		key := msg.String()
		if key == "ctrl+c" ||
			// Esc is used to quit filter menu, so we don't want to override that
			(key == "esc" && m.list.FilterState() != list.Filtering) {
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

type pickerItem struct {
	spirit sbx.Sandbox
	info   string
}

func (i pickerItem) Title() string {
	return i.spirit.Name
}

func (i pickerItem) Description() string {
	return i.info
}

func (i pickerItem) FilterValue() string {
	return i.spirit.Name
}

func makePickerItemInfo(spirit sbx.Sandbox, homedir string, now time.Time) string {
	var builder strings.Builder

	builder.WriteString(formatAge(spirit, now))

	if len(spirit.Workspaces) != 0 {
		builder.WriteString(" • ")
		primaryWorkspace := spirit.Workspaces[0]
		builder.WriteString(config.CollapseHomedir(primaryWorkspace, homedir))
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
