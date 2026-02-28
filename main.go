package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- APPLICATION STATES ---
type sessionState int

const (
	stateMenu       sessionState = iota
	stateFood                    // Interactive food list
	stateFoodRecipe              // Generated recipe screen
	stateFoodBuy                 // Checkout screen (Delivery/Pick Up)
	stateSubs
	stateStudy
)

// --- DATA STRUCTURES ---
type FoodItem struct {
	Name     string
	Price    float64
	Selected bool
}

// --- MAIN MODEL ---
type model struct {
	state  sessionState
	cursor int // Dynamic cursor for the current screen

	menuChoices []string

	// Food Data
	foodItems  []FoodItem
	buyChoices []string

	// Mocks for other categories
	mockSubs  []string
	mockStudy []string
}

func initialModel() model {
	return model{
		state:  stateMenu,
		cursor: 0,
		menuChoices: []string{
			"🛒 Food (Tracking, Recipes & Shopping)",
			"💳 Subscriptions (Payments & Dates)",
			"📚 Academics (Tasks & Deadlines)",
		},
		foodItems: []FoodItem{
			{Name: "Onions (1kg)", Price: 1.50, Selected: false},
			{Name: "Tomatoes (1kg)", Price: 2.00, Selected: false},
			{Name: "Chicken Breast", Price: 5.50, Selected: false},
			{Name: "Rice (1kg)", Price: 1.20, Selected: false},
			{Name: "Bell Peppers", Price: 0.90, Selected: false},
		},
		buyChoices: []string{
			"🚚 Delivery (+$3.00)",
			"🏪 Pick Up (Free)",
		},
		mockSubs: []string{
			"Netflix   - $15.99 (Due: Mar 05, 2026)",
			"Spotify   - $10.99 (Due: Mar 12, 2026)",
		},
		mockStudy: []string{
			"🔴 Math: Final Exam (Due in 2 days)",
			"🟡 History: Essay (Due in 5 days)",
		},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

// --- UPDATE (Interaction Logic) ---
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "esc", "backspace": // Go back
			if m.state == stateFoodRecipe || m.state == stateFoodBuy {
				m.state = stateFood // Go back to food list
				m.cursor = 0
			} else if m.state != stateMenu {
				m.state = stateMenu // Go back to main menu
				m.cursor = 0
			}

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
				limit = len(m.mockSubs) - 1
			}
			if m.state == stateStudy {
				limit = len(m.mockStudy) - 1
			}

			if m.cursor < limit {
				m.cursor++
			}

		case " ": // SPACE to select/deselect food items
			if m.state == stateFood {
				m.foodItems[m.cursor].Selected = !m.foodItems[m.cursor].Selected
			}

		case "r": // Generate Recipe
			if m.state == stateFood {
				m.state = stateFoodRecipe
			}

		case "c": // Checkout / Buy
			if m.state == stateFood {
				m.state = stateFoodBuy
				m.cursor = 0 // Reset cursor to choose delivery method
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
				// Here you would trigger the backend call to save the order
				return m, tea.Quit // For now, we quit simulating success
			}
		}
	}
	return m, nil
}

// --- STYLES ---
var (
	titleStyle = lipgloss.NewStyle().MarginBottom(1).Padding(0, 1).Foreground(lipgloss.Color("#FFF")).Background(lipgloss.Color("#7D56F4")).Bold(true)
	itemStyle  = lipgloss.NewStyle().PaddingLeft(2)
	selStyle   = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#04B575")).Bold(true)
	checkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#EE6FF8")).Bold(true)
	hintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#767676")).MarginTop(2)
	boxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).BorderForeground(lipgloss.Color("#7D56F4"))
)

// --- VIEW (Visual Rendering) ---
func (m model) View() string {
	var s string

	switch m.state {
	case stateMenu:
		s += titleStyle.Render("⚡ PERSONAL DASHBOARD") + "\n"
		s += renderList(m.menuChoices, m.cursor)
		s += hintStyle.Render("[↑/↓: Navigate • Enter: Select • q: Quit]")

	case stateFood:
		s += titleStyle.Render("🛒 FOOD - Inventory") + "\n"
		for i, item := range m.foodItems {
			cursor := "  "
			if m.cursor == i {
				cursor = "▶ "
			}

			check := "[ ]"
			if item.Selected {
				check = checkStyle.Render("[x]")
			}

			line := fmt.Sprintf("%s %s %-20s ($%.2f)", cursor, check, item.Name, item.Price)
			if m.cursor == i {
				s += selStyle.Render(line) + "\n"
			} else {
				s += itemStyle.Render(line) + "\n"
			}
		}
		s += hintStyle.Render("\n[Space: Select • r: Generate Recipe • c: Checkout • Esc: Back]")

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
			content := fmt.Sprintf("Selected ingredients:\n• %s\n\n💡 Suggestion:\nMix everything in a pan with olive oil,\nsalt, and pepper. A quick and nutritious stir-fry!", strings.Join(ingredients, "\n• "))
			s += boxStyle.Render(content)
		}
		s += hintStyle.Render("\n[Esc: Back to Food]")

	case stateFoodBuy:
		s += titleStyle.Render("🚚 CHECKOUT") + "\n"
		var total float64
		var count int
		for _, item := range m.foodItems {
			if item.Selected {
				total += item.Price
				count++
			}
		}

		if count == 0 {
			s += "No items in the cart.\n"
		} else {
			s += fmt.Sprintf("Selected items: %d\nSubtotal: $%.2f\n\nChoose delivery method:\n\n", count, total)
			for i, choice := range m.buyChoices {
				cursor := "  "
				if m.cursor == i {
					cursor = "▶ "
				}

				line := fmt.Sprintf("%s %s", cursor, choice)
				if m.cursor == i {
					s += selStyle.Render(line) + "\n"
				} else {
					s += itemStyle.Render(line) + "\n"
				}
			}

			// Calculate dynamic final total
			shipping := 0.0
			if m.cursor == 0 {
				shipping = 3.00
			} // Delivery
			s += fmt.Sprintf("\n💰 TOTAL TO PAY: $%.2f\n", total+shipping)
		}
		s += hintStyle.Render("\n[↑/↓: Choose • Enter: Confirm Purchase • Esc: Cancel]")

	case stateSubs:
		s += titleStyle.Render("💳 SUBSCRIPTIONS") + "\n"
		s += renderList(m.mockSubs, m.cursor)
		s += hintStyle.Render("[↑/↓: Navigate • Esc: Back to Menu]")

	case stateStudy:
		s += titleStyle.Render("📚 ACADEMICS") + "\n"
		s += renderList(m.mockStudy, m.cursor)
		s += hintStyle.Render("[↑/↓: Navigate • Esc: Back to Menu]")
	}

	return lipgloss.NewStyle().Margin(1, 2).Render(s)
}

func renderList(items []string, cursor int) string {
	var s string
	for i, item := range items {
		if cursor == i {
			s += selStyle.Render("▶ "+item) + "\n"
		} else {
			s += itemStyle.Render("  "+item) + "\n"
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
