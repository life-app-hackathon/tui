package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- APPLICATION STATES ---
type sessionState int

const (
	stateMenu sessionState = iota
	stateFood
	stateFoodRecipe
	stateFoodBuy
	stateSubs
	stateStudy
	stateAddFood
	stateAddSub
)

// --- DATA STRUCTURES ---
type FoodItem struct {
	Name           string
	Price          float64
	Amount         int
	RenewThreshold int
	Selected       bool
}

type SubItem struct {
	Name    string
	Price   float64
	DueDate string
	Cycle   string
}

type StudyItem struct {
	Name    string
	DueDate string
}

// --- MAIN MODEL ---
type model struct {
	state      sessionState
	cursor     int
	inputs     []textinput.Model
	focusIndex int
	editIndex  int
	token      string

	// Radio button states for Subscriptions
	subCycleChoices []string
	subCycleChoice  int

	menuChoices []string
	foodItems   []FoodItem
	buyChoices  []string
	subItems    []SubItem
	studyItems  []StudyItem
}

func initialModel(token string) model {
	m := model{
		state:     stateMenu,
		cursor:    0,
		editIndex: -1,
		token:     token,

		// Initialize the radio button options
		subCycleChoices: []string{"Monthly", "3 Months", "Yearly"},
		subCycleChoice:  0,

		menuChoices: []string{
			"🛒 Food (Tracking, Recipes & Shopping)",
			"💳 Subscriptions (Payments & Dates)",
			"📚 Academics (Scraped Assignments)",
		},
		buyChoices: []string{
			"🚚 Delivery (+$3.00)",
			"🏪 Pick Up (Free)",
		},
	}

	// Mock Backend
	switch token {
	case "user1":
		m.foodItems = []FoodItem{
			{Name: "Apples", Price: 2.50, Amount: 5, RenewThreshold: 2, Selected: false},
			{Name: "Oatmeal", Price: 3.00, Amount: 1, RenewThreshold: 1, Selected: false},
			{Name: "Almond Milk", Price: 4.50, Amount: 2, RenewThreshold: 0, Selected: false},
		}
		m.subItems = []SubItem{
			{Name: "Gym", Price: 30.00, DueDate: "Mar 01, 2026", Cycle: "Monthly"},
			{Name: "Spotify", Price: 10.99, DueDate: "Mar 15, 2026", Cycle: "Monthly"},
			{Name: "Domain Name", Price: 12.00, DueDate: "Jan 10, 2027", Cycle: "Yearly"},
		}
		m.studyItems = []StudyItem{
			{Name: "🟢 Biology: Lab Report", DueDate: "Due in 1 day"},
			{Name: "🟡 English: Reading", DueDate: "Due in 3 days"},
		}

	case "user2":
		m.foodItems = []FoodItem{
			{Name: "Steak", Price: 12.00, Amount: 2, RenewThreshold: 0, Selected: false},
			{Name: "Eggs (Dozen)", Price: 4.00, Amount: 1, RenewThreshold: 1, Selected: false},
		}
		m.subItems = []SubItem{
			{Name: "Netflix", Price: 15.99, DueDate: "Apr 05, 2026", Cycle: "Monthly"},
			{Name: "VPN", Price: 45.00, DueDate: "Aug 12, 2026", Cycle: "Yearly"},
		}
		m.studyItems = []StudyItem{
			{Name: "🔴 Math: Calculus Exam", DueDate: "Due in 10 days"},
		}

	default:
		m.foodItems = []FoodItem{}
		m.subItems = []SubItem{}
		m.studyItems = []StudyItem{}
	}

	return m
}

func (m *model) initForm(state sessionState, isEdit bool) {
	m.focusIndex = 0

	if state == stateAddFood {
		// Food uses 4 text inputs
		m.inputs = make([]textinput.Model, 4)
		for i := range m.inputs {
			t := textinput.New()
			t.CharLimit = 32
			if i == 0 {
				t.Focus()
				t.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
				t.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
			}
			m.inputs[i] = t
		}

		m.inputs[0].Placeholder = "Food Name (e.g., Apple)"
		m.inputs[1].Placeholder = "Price (e.g., 2.50)"
		m.inputs[2].Placeholder = "Current Amount (e.g., 5)"
		m.inputs[3].Placeholder = "Auto-Renew Threshold (0 = disabled)"

		if isEdit && m.editIndex >= 0 {
			item := m.foodItems[m.editIndex]
			m.inputs[0].SetValue(item.Name)
			m.inputs[1].SetValue(fmt.Sprintf("%.2f", item.Price))
			m.inputs[2].SetValue(strconv.Itoa(item.Amount))
			m.inputs[3].SetValue(strconv.Itoa(item.RenewThreshold))
		}

	} else if state == stateAddSub {
		// Subs uses 3 text inputs + 1 custom radio button row
		m.inputs = make([]textinput.Model, 3)
		for i := range m.inputs {
			t := textinput.New()
			t.CharLimit = 32
			if i == 0 {
				t.Focus()
				t.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
				t.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
			}
			m.inputs[i] = t
		}

		m.inputs[0].Placeholder = "Service Name (e.g., Netflix)"
		m.inputs[1].Placeholder = "Price (e.g., 15.99)"
		m.inputs[2].Placeholder = "Payment Date (e.g., Apr 01, 2026)"

		m.subCycleChoice = 0 // Default to Monthly

		if isEdit && m.editIndex >= 0 {
			item := m.subItems[m.editIndex]
			m.inputs[0].SetValue(item.Name)
			m.inputs[1].SetValue(fmt.Sprintf("%.2f", item.Price))
			m.inputs[2].SetValue(item.DueDate)

			// Match the cycle to our radio button choices
			for i, c := range m.subCycleChoices {
				if c == item.Cycle {
					m.subCycleChoice = i
					break
				}
			}
		}
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// --- UPDATE ---
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// --- FORM HANDLING ---
		if m.state == stateAddFood || m.state == stateAddSub {
			switch msg.String() {
			case "esc":
				m.goBack()
				return m, nil

			// Catch left/right specifically for the radio buttons
			case "left", "right":
				if m.state == stateAddSub && m.focusIndex == 3 {
					if msg.String() == "left" && m.subCycleChoice > 0 {
						m.subCycleChoice--
					} else if msg.String() == "right" && m.subCycleChoice < len(m.subCycleChoices)-1 {
						m.subCycleChoice++
					}
					return m, nil // Don't pass arrow keys to text input
				}

			case "tab", "shift+tab", "enter", "up", "down":
				s := msg.String()

				totalFields := 4 // Both forms have 4 logical fields (Inputs + Radios)

				// If we hit enter on the LAST field, save the form!
				if s == "enter" && m.focusIndex == totalFields-1 {
					m.saveForm()
					m.goBack()
					return m, nil
				}

				if s == "up" || s == "shift+tab" {
					m.focusIndex--
				} else if s == "down" || s == "tab" || s == "enter" {
					m.focusIndex++
				}

				// Wrap around
				if m.focusIndex > totalFields-1 {
					m.focusIndex = 0
				} else if m.focusIndex < 0 {
					m.focusIndex = totalFields - 1
				}

				// Apply focus styles visually
				cmds := make([]tea.Cmd, len(m.inputs))
				for i := 0; i < len(m.inputs); i++ {
					if i == m.focusIndex {
						cmds[i] = m.inputs[i].Focus()
						m.inputs[i].PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
						m.inputs[i].TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
					} else {
						m.inputs[i].Blur()
						m.inputs[i].PromptStyle = lipgloss.NewStyle()
						m.inputs[i].TextStyle = lipgloss.NewStyle()
					}
				}
				return m, tea.Batch(cmds...)
			}

			cmd := m.updateInputs(msg)
			return m, cmd
		}

		// --- NORMAL NAVIGATION ---
		switch msg.String() {
		case "q":
			return m, tea.Quit

		case "esc", "backspace":
			m.goBack()

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			limit := 0
			if m.state == stateMenu {
				limit = len(m.menuChoices) - 1
			}
			if m.state == stateFood {
				limit = len(m.foodItems) - 1
			}
			if m.state == stateFoodBuy {
				limit = len(m.buyChoices) - 1
			}
			if m.state == stateSubs {
				limit = len(m.subItems) - 1
			}
			if m.state == stateStudy {
				limit = len(m.studyItems) - 1
			}

			if limit < 0 {
				limit = 0
			}
			if m.cursor < limit {
				m.cursor++
			}

		case "a": // Add
			if m.state == stateFood {
				m.state = stateAddFood
				m.editIndex = -1
				m.initForm(stateAddFood, false)
			} else if m.state == stateSubs {
				m.state = stateAddSub
				m.editIndex = -1
				m.initForm(stateAddSub, false)
			}

		case "e": // Edit
			if m.state == stateFood && len(m.foodItems) > 0 {
				m.state = stateAddFood
				m.editIndex = m.cursor
				m.initForm(stateAddFood, true)
			} else if m.state == stateSubs && len(m.subItems) > 0 {
				m.state = stateAddSub
				m.editIndex = m.cursor
				m.initForm(stateAddSub, true)
			}

		case " ":
			if m.state == stateFood && len(m.foodItems) > 0 {
				m.foodItems[m.cursor].Selected = !m.foodItems[m.cursor].Selected
			}

		case "r":
			if m.state == stateFood {
				m.state = stateFoodRecipe
			}

		case "c":
			if m.state == stateFood {
				m.state = stateFoodBuy
				m.cursor = 0
			}

		case "enter":
			if m.state == stateMenu {
				switch m.cursor {
				case 0:
					m.state = stateFood
				case 1:
					m.state = stateSubs
				case 2:
					m.state = stateStudy
				}
				m.cursor = 0
			} else if m.state == stateFoodBuy {
				for i := range m.foodItems {
					m.foodItems[i].Selected = false
				}
				m.state = stateFood
				m.cursor = 0
			}
		}
	}
	return m, nil
}

func (m *model) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m *model) saveForm() {
	name := m.inputs[0].Value()
	if name == "" {
		return
	}

	if m.state == stateAddFood {
		price, _ := strconv.ParseFloat(m.inputs[1].Value(), 64)
		amount, _ := strconv.Atoi(m.inputs[2].Value())
		if amount == 0 {
			amount = 1
		}
		thresh, _ := strconv.Atoi(m.inputs[3].Value())

		newItem := FoodItem{Name: name, Price: price, Amount: amount, RenewThreshold: thresh, Selected: false}

		if m.editIndex >= 0 {
			newItem.Selected = m.foodItems[m.editIndex].Selected
			m.foodItems[m.editIndex] = newItem
		} else {
			m.foodItems = append(m.foodItems, newItem)
		}

	} else if m.state == stateAddSub {
		price, _ := strconv.ParseFloat(m.inputs[1].Value(), 64)
		date := m.inputs[2].Value()
		if date == "" {
			date = "TBD"
		}

		// Pull the value from our radio button state instead of a text input!
		cycle := m.subCycleChoices[m.subCycleChoice]

		newItem := SubItem{Name: name, Price: price, DueDate: date, Cycle: cycle}

		if m.editIndex >= 0 {
			m.subItems[m.editIndex] = newItem
		} else {
			m.subItems = append(m.subItems, newItem)
		}
	}
}

func (m *model) goBack() {
	if m.state == stateFoodRecipe || m.state == stateFoodBuy || m.state == stateAddFood {
		m.state = stateFood
	} else if m.state == stateAddSub {
		m.state = stateSubs
	} else if m.state != stateMenu {
		m.state = stateMenu
	}
	m.cursor = 0
}

// --- STYLES ---
var (
	titleStyle = lipgloss.NewStyle().MarginBottom(1).Padding(0, 1).Foreground(lipgloss.Color("#FFF")).Background(lipgloss.Color("#7D56F4")).Bold(true)
	itemStyle  = lipgloss.NewStyle()
	selStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	checkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#EE6FF8")).Bold(true)
	hintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#767676"))
	boxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).BorderForeground(lipgloss.Color("#7D56F4"))
)

// --- VIEW ---
func (m model) View() string {
	var s string

	if m.state == stateAddFood || m.state == stateAddSub {
		if m.editIndex >= 0 {
			s += titleStyle.Render("✏️ EDIT ITEM") + "\n\n"
		} else {
			s += titleStyle.Render("➕ ADD NEW ITEM") + "\n\n"
		}

		// Render normal text inputs
		for i := range m.inputs {
			s += m.inputs[i].View() + "\n"
		}

		// Render custom Radio Buttons if we are in Subscriptions
		if m.state == stateAddSub {
			radioPrompt := "  Cycle:"
			if m.focusIndex == 3 {
				radioPrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Render("> Cycle:")
			}

			s += radioPrompt + "\n  "
			for i, choice := range m.subCycleChoices {
				marker := "( )"
				if m.subCycleChoice == i {
					marker = checkStyle.Render("(x)")
				}
				s += fmt.Sprintf("%s %s   ", marker, choice)
			}
			s += "\n"
		}

		s += "\n\n" + hintStyle.Render("[Tab/Up/Down: Next • Left/Right: Select Cycle • Enter: Save]")
		return lipgloss.NewStyle().Margin(1, 2).Render(s)
	}

	switch m.state {
	case stateMenu:
		s += titleStyle.Render("⚡ PERSONAL DASHBOARD") + "\n"
		if m.token != "" {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Render(fmt.Sprintf("🔑 Authenticated as: %s", m.token)) + "\n\n"
		} else {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87")).Render("⚠️  No token provided. Running locally.") + "\n\n"
		}

		s += renderList(m.menuChoices, m.cursor)
		s += "\n" + hintStyle.Render("[up/down: Navigate • Enter: Select • q: Quit]")

	case stateFood:
		s += titleStyle.Render("🛒 FOOD - Inventory") + "\n"
		if len(m.foodItems) == 0 {
			s += "    No items. Press 'a' to add one.\n"
		} else {
			for i, item := range m.foodItems {
				cursor := "  "
				if m.cursor == i {
					cursor = "▶ "
				}

				check := "[ ]"
				if item.Selected {
					check = checkStyle.Render("[x]")
				}

				nameCol := lipgloss.NewStyle().Width(18).Render(item.Name)

				renewTag := "       "
				if item.RenewThreshold > 0 {
					renewTag = lipgloss.NewStyle().Foreground(lipgloss.Color("#E1B12C")).Render(fmt.Sprintf("[R≤%d]", item.RenewThreshold))
					renewTag = lipgloss.NewStyle().Width(7).Render(renewTag)
				}

				line := fmt.Sprintf("  %s %s %s (x%d) %s -  $%.2f", cursor, check, nameCol, item.Amount, renewTag, item.Price)

				if m.cursor == i {
					s += selStyle.Render(line) + "\n"
				} else {
					s += itemStyle.Render(line) + "\n"
				}
			}
		}
		s += "\n" + hintStyle.Render("[a: Add • e: Edit • Space: Select • r: Recipe • c: Checkout • Esc: Back]")

	case stateFoodRecipe:
		s += titleStyle.Render("🍳 GENERATED RECIPE") + "\n"
		var ingredients []string
		for _, item := range m.foodItems {
			if item.Selected {
				ingredients = append(ingredients, item.Name)
			}
		}

		if len(ingredients) == 0 {
			s += boxStyle.Render("❌ You haven't selected any food items.\nGo back and select ingredients with [Space].")
		} else {
			content := fmt.Sprintf("Selected ingredients:\n- %s\n\n💡 Tip:\nMix everything in a pan with olive oil,\nsalt, and pepper. A quick and nutritious stir-fry!", strings.Join(ingredients, "\n- "))
			s += boxStyle.Render(content)
		}
		s += "\n" + hintStyle.Render("[Esc: Back to Food]")

	case stateFoodBuy:
		s += titleStyle.Render("🚚 CHECKOUT") + "\n"
		var total float64
		var count int
		for _, item := range m.foodItems {
			if item.Selected {
				total += item.Price * float64(item.Amount)
				count++
			}
		}

		if count == 0 {
			s += boxStyle.Render("🛒 No items in the cart.\nGo back and select items with [Space].")
		} else {
			s += fmt.Sprintf("Selected unique items: %d\nSubtotal: $%.2f\n\nChoose delivery method:\n\n", count, total)
			for i, choice := range m.buyChoices {
				cursor := "  "
				if m.cursor == i {
					cursor = "▶ "
				}

				line := fmt.Sprintf("  %s %s", cursor, choice)
				if m.cursor == i {
					s += selStyle.Render(line) + "\n"
				} else {
					s += itemStyle.Render(line) + "\n"
				}
			}

			shipping := 0.0
			if m.cursor == 0 {
				shipping = 3.00
			}
			s += fmt.Sprintf("\n💰 TOTAL TO PAY: $%.2f\n", total+shipping)
		}

		if count > 0 {
			s += "\n" + hintStyle.Render("[up/down: Choose • Enter: Confirm Purchase • Esc: Cancel]")
		} else {
			s += "\n" + hintStyle.Render("[Esc: Back to Food]")
		}

	case stateSubs:
		s += titleStyle.Render("💳 SUBSCRIPTIONS") + "\n"
		if len(m.subItems) == 0 {
			s += "    No items.\n"
		} else {
			for i, item := range m.subItems {
				cursor := "  "
				if m.cursor == i {
					cursor = "▶ "
				}

				nameCol := lipgloss.NewStyle().Width(15).Render(item.Name)
				cycleCol := lipgloss.NewStyle().Width(10).Render(item.Cycle)

				line := fmt.Sprintf("  %s %s | %s | $%.2f | Due: %s", cursor, nameCol, cycleCol, item.Price, item.DueDate)

				if m.cursor == i {
					s += selStyle.Render(line) + "\n"
				} else {
					s += itemStyle.Render(line) + "\n"
				}
			}
		}
		s += "\n" + hintStyle.Render("[a: Add • e: Edit • up/down: Navigate • Esc: Back]")

	case stateStudy:
		s += titleStyle.Render("📚 ACADEMICS (Automated Scraper)") + "\n"
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Render("Status: Synced with university portal") + "\n\n"

		if len(m.studyItems) == 0 {
			s += "    No pending assignments.\n"
		} else {
			for i, item := range m.studyItems {
				cursor := "  "
				if m.cursor == i {
					cursor = "▶ "
				}

				nameCol := lipgloss.NewStyle().Width(25).Render(item.Name)
				line := fmt.Sprintf("  %s %s | %s", cursor, nameCol, item.DueDate)

				if m.cursor == i {
					s += selStyle.Render(line) + "\n"
				} else {
					s += itemStyle.Render(line) + "\n"
				}
			}
		}
		s += "\n" + hintStyle.Render("[up/down: Navigate • Esc: Back]")
	}

	return lipgloss.NewStyle().Margin(1, 2).Render(s)
}

func renderList(items []string, cursor int) string {
	var s string
	for i, item := range items {
		if cursor == i {
			s += selStyle.Render("  ▶ "+item) + "\n"
		} else {
			s += itemStyle.Render("    "+item) + "\n"
		}
	}
	return s
}

func main() {
	tokenPtr := flag.String("token", "", "User authentication token (Mandatory)")
	flag.Parse()

	if *tokenPtr == "" {
		fmt.Println("❌ Error: The --token flag is mandatory.")
		fmt.Println("Usage: go run main.go --token=user1")
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel(*tokenPtr), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error starting TUI: %v\n", err)
		os.Exit(1)
	}
}
