package main

import (
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
	Name     string
	Price    float64
	Amount   int
	Selected bool
}

type SubItem struct {
	Name    string
	Price   float64
	DueDate string
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

	menuChoices []string
	foodItems   []FoodItem
	buyChoices  []string
	subItems    []SubItem
	studyItems  []StudyItem
}

func initialModel() model {
	return model{
		state:  stateMenu,
		cursor: 0,
		menuChoices: []string{
			"Food (Tracking, Recipes & Shopping)",
			"Subscriptions (Payments & Dates)",
			"Academics (Scraped Assignments)",
		},
		foodItems: []FoodItem{
			{Name: "Onions", Price: 1.50, Amount: 2, Selected: false},
			{Name: "Tomatoes", Price: 2.00, Amount: 3, Selected: false},
			{Name: "Chicken Breast", Price: 5.50, Amount: 1, Selected: false},
		},
		buyChoices: []string{
			"Delivery (+$3.00)",
			"Pick Up (Free)",
		},
		subItems: []SubItem{
			{Name: "Netflix", Price: 15.99, DueDate: "Mar 05, 2026"},
			{Name: "Spotify", Price: 10.99, DueDate: "Mar 12, 2026"},
		},
		studyItems: []StudyItem{
			{Name: "[High] Math: Final Exam", DueDate: "Due in 2 days"},
			{Name: "[Med] History: Essay", DueDate: "Due in 5 days"},
		},
	}
}

func (m *model) initForm(state sessionState) {
	m.focusIndex = 0
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

	if state == stateAddFood {
		m.inputs[0].Placeholder = "Food Name (e.g., Apple)"
		m.inputs[1].Placeholder = "Price (e.g., 2.50)"
		m.inputs[2].Placeholder = "Amount (e.g., 5)"
	} else if state == stateAddSub {
		m.inputs[0].Placeholder = "Service Name (e.g., Netflix)"
		m.inputs[1].Placeholder = "Price (e.g., 15.99)"
		m.inputs[2].Placeholder = "Payment Date (e.g., Apr 01, 2026)"
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

		if m.state == stateAddFood || m.state == stateAddSub {
			switch msg.String() {
			case "esc":
				m.goBack()
				return m, nil

			case "tab", "shift+tab", "enter", "up", "down":
				s := msg.String()

				if s == "enter" && m.focusIndex == len(m.inputs)-1 {
					m.saveForm()
					m.goBack()
					return m, nil
				}

				if s == "up" || s == "shift+tab" {
					m.focusIndex--
				} else {
					m.focusIndex++
				}

				if m.focusIndex > len(m.inputs)-1 {
					m.focusIndex = 0
				} else if m.focusIndex < 0 {
					m.focusIndex = len(m.inputs) - 1
				}

				cmds := make([]tea.Cmd, len(m.inputs))
				for i := 0; i <= len(m.inputs)-1; i++ {
					if i == m.focusIndex {
						cmds[i] = m.inputs[i].Focus()
						m.inputs[i].PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
						m.inputs[i].TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
						continue
					}
					m.inputs[i].Blur()
					m.inputs[i].PromptStyle = lipgloss.NewStyle()
					m.inputs[i].TextStyle = lipgloss.NewStyle()
				}
				return m, tea.Batch(cmds...)
			}

			cmd := m.updateInputs(msg)
			return m, cmd
		}

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

		case "a":
			if m.state == stateFood {
				m.state = stateAddFood
				m.initForm(stateAddFood)
			} else if m.state == stateSubs {
				m.state = stateAddSub
				m.initForm(stateAddSub)
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

		m.foodItems = append(m.foodItems, FoodItem{Name: name, Price: price, Amount: amount, Selected: false})
	} else if m.state == stateAddSub {
		price, _ := strconv.ParseFloat(m.inputs[1].Value(), 64)
		date := m.inputs[2].Value()
		if date == "" {
			date = "TBD"
		}

		m.subItems = append(m.subItems, SubItem{Name: name, Price: price, DueDate: date})
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
// Removed PaddingLeft from all styles so we can manually control spaces!
var (
	titleStyle = lipgloss.NewStyle().MarginBottom(1).Padding(0, 1).Foreground(lipgloss.Color("#FFF")).Background(lipgloss.Color("#7D56F4")).Bold(true)
	itemStyle  = lipgloss.NewStyle()
	selStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	checkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#EE6FF8")).Bold(true)
	hintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#767676")) // Margin Top removed, handled via string \n
	boxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).BorderForeground(lipgloss.Color("#7D56F4"))
)

// --- VIEW ---
func (m model) View() string {
	var s string

	if m.state == stateAddFood || m.state == stateAddSub {
		s += titleStyle.Render("ADD NEW ITEM") + "\n\n"
		for i := range m.inputs {
			s += m.inputs[i].View()
			if i < len(m.inputs)-1 {
				s += "\n"
			}
		}
		// Newlines pulled OUTSIDE the render function
		s += "\n\n" + hintStyle.Render("[Tab/Up/Down: Next Field • Enter: Save • Esc: Cancel]")
		return lipgloss.NewStyle().Margin(1, 2).Render(s)
	}

	switch m.state {
	case stateMenu:
		s += titleStyle.Render("PERSONAL DASHBOARD") + "\n"
		s += renderList(m.menuChoices, m.cursor)
		s += "\n" + hintStyle.Render("[up/down: Navigate • Enter: Select • q: Quit]")

	case stateFood:
		s += titleStyle.Render("FOOD - Inventory") + "\n"
		if len(m.foodItems) == 0 {
			s += "    No items. Press 'a' to add one.\n"
		} else {
			for i, item := range m.foodItems {
				cursor := "  "
				if m.cursor == i {
					cursor = "> "
				}

				check := "[ ]"
				if item.Selected {
					check = checkStyle.Render("[x]")
				}

				// Added manual spaces "  %s" at the beginning for perfect padding
				line := fmt.Sprintf("  %s %s %-18s (x%d)  -  $%.2f", cursor, check, item.Name, item.Amount, item.Price)

				if m.cursor == i {
					s += selStyle.Render(line) + "\n"
				} else {
					s += itemStyle.Render(line) + "\n"
				}
			}
		}
		s += "\n" + hintStyle.Render("[a: Add • Space: Select • r: Recipe • c: Checkout • Esc: Back]")

	case stateFoodRecipe:
		s += titleStyle.Render("GENERATED RECIPE") + "\n"
		var ingredients []string
		for _, item := range m.foodItems {
			if item.Selected {
				ingredients = append(ingredients, item.Name)
			}
		}

		if len(ingredients) == 0 {
			s += boxStyle.Render("[!] You haven't selected any food items.\nGo back and select ingredients with [Space].")
		} else {
			content := fmt.Sprintf("Selected ingredients:\n- %s\n\nTip:\nMix everything in a pan with olive oil,\nsalt, and pepper. A quick and nutritious stir-fry!", strings.Join(ingredients, "\n- "))
			s += boxStyle.Render(content)
		}
		s += "\n" + hintStyle.Render("[Esc: Back to Food]")

	case stateFoodBuy:
		s += titleStyle.Render("CHECKOUT") + "\n"
		var total float64
		var count int
		for _, item := range m.foodItems {
			if item.Selected {
				total += item.Price * float64(item.Amount)
				count++
			}
		}

		if count == 0 {
			s += boxStyle.Render("No items in the cart.\nGo back and select items with [Space].")
		} else {
			s += fmt.Sprintf("Selected unique items: %d\nSubtotal: $%.2f\n\nChoose delivery method:\n\n", count, total)
			for i, choice := range m.buyChoices {
				cursor := "  "
				if m.cursor == i {
					cursor = "> "
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
			s += fmt.Sprintf("\nTOTAL TO PAY: $%.2f\n", total+shipping)
		}

		if count > 0 {
			s += "\n" + hintStyle.Render("[up/down: Choose • Enter: Confirm Purchase • Esc: Cancel]")
		} else {
			s += "\n" + hintStyle.Render("[Esc: Back to Food]")
		}

	case stateSubs:
		s += titleStyle.Render("SUBSCRIPTIONS") + "\n"
		if len(m.subItems) == 0 {
			s += "    No items.\n"
		} else {
			for i, item := range m.subItems {
				cursor := "  "
				if m.cursor == i {
					cursor = "> "
				}

				line := fmt.Sprintf("  %s %-15s | $%.2f | Due: %s", cursor, item.Name, item.Price, item.DueDate)

				if m.cursor == i {
					s += selStyle.Render(line) + "\n"
				} else {
					s += itemStyle.Render(line) + "\n"
				}
			}
		}
		s += "\n" + hintStyle.Render("[a: Add • up/down: Navigate • Esc: Back]")

	case stateStudy:
		s += titleStyle.Render("ACADEMICS (Automated Scraper)") + "\n"
		// The bug was here! Pulled \n\n outside of the Render call.
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Render("Status: Synced with university portal") + "\n\n"

		if len(m.studyItems) == 0 {
			s += "    No pending assignments.\n"
		} else {
			for i, item := range m.studyItems {
				cursor := "  "
				if m.cursor == i {
					cursor = "> "
				}

				line := fmt.Sprintf("  %s %-25s | %s", cursor, item.Name, item.DueDate)

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
			s += selStyle.Render("  > "+item) + "\n"
		} else {
			s += itemStyle.Render("    "+item) + "\n"
		}
	}
	return s
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error starting TUI: %v\n", err)
		os.Exit(1)
	}
}
