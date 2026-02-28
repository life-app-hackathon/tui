package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const baseURL = "https://backend1.study-with-me.org" // Change to your actual backend URL if needed

// --- APPLICATION STATES ---
type sessionState int

const (
	stateMenu sessionState = iota
	stateFood
	stateFoodRecipe
	stateFoodBuy
	stateProcessingBuy
	stateSubs
	stateStudy
	stateScrapingCanvas // NEW: Scraping loading state
	stateAddFood
	stateAddSub
)

// --- DATA STRUCTURES ---
type FoodItem struct {
	Name           string  `json:"name"`
	Price          float64 `json:"price"`
	Amount         int     `json:"amount"`
	RenewThreshold int     `json:"renewThreshold"`
	CartQty        int     `json:"-"`
}

type SubItem struct {
	Name    string  `json:"name"`
	Price   float64 `json:"price"`
	DueDate string  `json:"dueDate"`
	Cycle   string  `json:"cycle"`
}

type StudyItem struct {
	Name    string `json:"name"`
	DueDate string `json:"dueDate"`
}

type CategoryResponse struct {
	Id      string          `json:"id"`
	UserId  string          `json:"user_id"`
	Name    string          `json:"name"`
	Content json.RawMessage `json:"content"`
}

// --- MESSAGES ---
type dataFetchedMsg []CategoryResponse
type syncSuccessMsg struct{}
type recipeGeneratedMsg string
type buyCompleteMsg struct{}
type canvasScrapedMsg []StudyItem // NEW: Message to handle scraped data
type errMsg struct{ err error }

// --- MAIN MODEL ---
type model struct {
	state      sessionState
	cursor     int
	inputs     []textinput.Model
	focusIndex int
	editIndex  int
	token      string
	statusMsg  string
	catIDs     map[string]string

	subCycleChoices []string
	subCycleChoice  int

	menuChoices []string
	foodItems   []FoodItem
	buyChoices  []string
	subItems    []SubItem
	studyItems  []StudyItem

	generatedRecipe string
	isGenerating    bool
}

func initialModel(token string) model {
	return model{
		state:     stateMenu,
		cursor:    0,
		editIndex: -1,
		token:     token,
		statusMsg: "Fetching data...",
		catIDs:    make(map[string]string),

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
		foodItems:  []FoodItem{},
		subItems:   []SubItem{},
		studyItems: []StudyItem{},
	}
}

// --- HTTP COMMANDS ---
func fetchCategoriesCmd(token string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(baseURL + "/categories/" + token)
		if err != nil {
			return errMsg{err}
		}
		defer resp.Body.Close()

		var cats []CategoryResponse
		if err := json.NewDecoder(resp.Body).Decode(&cats); err != nil {
			return errMsg{err}
		}
		return dataFetchedMsg(cats)
	}
}

func syncCategoryCmd(token, name, catId string, items interface{}) tea.Cmd {
	return func() tea.Msg {
		contentBytes, _ := json.Marshal(map[string]interface{}{"items": items})
		payload := CategoryResponse{Id: catId, UserId: token, Name: name, Content: contentBytes}
		body, _ := json.Marshal(payload)

		var req *http.Request
		var err error

		if catId == "" {
			req, err = http.NewRequest("POST", baseURL+"/categories", bytes.NewBuffer(body))
		} else {
			req, err = http.NewRequest("PUT", baseURL+"/categories/"+catId, bytes.NewBuffer(body))
		}

		if err != nil {
			return errMsg{err}
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)

		if err != nil || resp.StatusCode >= 400 {
			msg := "Sync failed"
			if err != nil {
				msg = err.Error()
			}
			return errMsg{fmt.Errorf(msg)}
		}

		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return syncSuccessMsg{}
	}
}

func generateRecipeCmd(ingredients []string) tea.Cmd {
	return func() tea.Msg {
		payload := map[string][]string{"ingredients": ingredients}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(baseURL+"/recipes/generate", "application/json", bytes.NewBuffer(body))
		if err != nil {
			return errMsg{err}
		}
		defer resp.Body.Close()

		var result map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return errMsg{err}
		}

		return recipeGeneratedMsg(result["recipe"])
	}
}

func processBuyCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(1500 * time.Millisecond)
		return buyCompleteMsg{}
	}
}

// NEW: Command to scrape canvas
func scrapeCanvasCmd(token string) tea.Cmd {
	return func() tea.Msg {
		// Notice we added ?user_id= to the URL
		resp, err := http.Post(baseURL+"/scrapers/canvas?user_id="+token, "application/json", nil)
		if err != nil {
			return errMsg{err}
		}
		defer resp.Body.Close()

		var result map[string][]StudyItem
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return errMsg{err}
		}

		return canvasScrapedMsg(result["items"])
	}
}

// --- FORM INIT ---
func (m *model) initForm(state sessionState, isEdit bool) {
	m.focusIndex = 0

	if state == stateAddFood {
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
		m.inputs[0].Placeholder = "Food Name"
		m.inputs[1].Placeholder = "Price per unit"
		m.inputs[2].Placeholder = "Current Stock Amount"
		m.inputs[3].Placeholder = "Auto-Renew Threshold (0 = disabled)"

		if isEdit && m.editIndex >= 0 {
			item := m.foodItems[m.editIndex]
			m.inputs[0].SetValue(item.Name)
			m.inputs[1].SetValue(fmt.Sprintf("%.2f", item.Price))
			m.inputs[2].SetValue(strconv.Itoa(item.Amount))
			m.inputs[3].SetValue(strconv.Itoa(item.RenewThreshold))
		}
	} else if state == stateAddSub {
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
		m.inputs[0].Placeholder = "Service Name"
		m.inputs[1].Placeholder = "Price"
		m.inputs[2].Placeholder = "Payment Date"
		m.subCycleChoice = 0

		if isEdit && m.editIndex >= 0 {
			item := m.subItems[m.editIndex]
			m.inputs[0].SetValue(item.Name)
			m.inputs[1].SetValue(fmt.Sprintf("%.2f", item.Price))
			m.inputs[2].SetValue(item.DueDate)
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
	return tea.Batch(textinput.Blink, fetchCategoriesCmd(m.token))
}

// --- UPDATE ---
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case dataFetchedMsg:
		m.statusMsg = "Data loaded successfully."
		for _, cat := range msg {
			m.catIDs[cat.Name] = cat.Id
			var wrapper map[string]json.RawMessage
			json.Unmarshal(cat.Content, &wrapper)

			switch cat.Name {
			case "Food":
				json.Unmarshal(wrapper["items"], &m.foodItems)
			case "Subscriptions":
				json.Unmarshal(wrapper["items"], &m.subItems)
			case "Academics":
				json.Unmarshal(wrapper["items"], &m.studyItems)
			}
		}
		return m, nil

	case syncSuccessMsg:
		if m.statusMsg == "Syncing..." || m.statusMsg == "Syncing deletion..." || m.statusMsg == "Syncing Canvas data..." {
			m.statusMsg = "Saved securely to database ✓"
		}
		return m, fetchCategoriesCmd(m.token)

	case recipeGeneratedMsg:
		m.isGenerating = false
		m.generatedRecipe = string(msg)
		return m, nil

	case buyCompleteMsg:
		for i := range m.foodItems {
			if m.foodItems[i].CartQty > 0 {
				m.foodItems[i].Amount += m.foodItems[i].CartQty
				m.foodItems[i].CartQty = 0
			}
		}
		m.state = stateFood
		m.cursor = 0
		m.statusMsg = "Order placed! Stock updated in database 🚚"
		return m, syncCategoryCmd(m.token, "Food", m.catIDs["Food"], m.foodItems)

	// NEW: Handle Canvas scraping completion
	// Replace your old case canvasScrapedMsg with this:
	case canvasScrapedMsg:
		m.studyItems = msg
		m.state = stateStudy
		m.cursor = 0
		m.statusMsg = "Canvas sync complete! ✅"

		// Instead of syncing to the database (the backend did that for us),
		// we just fetch categories to grab the new Database ID!
		return m, fetchCategoriesCmd(m.token)

	case errMsg:
		m.isGenerating = false
		m.statusMsg = "Error: " + msg.err.Error()
		if m.state == stateFoodRecipe {
			m.generatedRecipe = "Server Error: " + msg.err.Error()
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Block normal inputs if we are in a loading state
		if m.state == stateProcessingBuy || m.state == stateScrapingCanvas {
			return m, nil
		}

		if m.state == stateAddFood || m.state == stateAddSub {
			switch msg.String() {
			case "esc":
				m.goBack()
				return m, nil
			case "left", "right":
				if m.state == stateAddSub && m.focusIndex == 3 {
					if msg.String() == "left" && m.subCycleChoice > 0 {
						m.subCycleChoice--
					} else if msg.String() == "right" && m.subCycleChoice < len(m.subCycleChoices)-1 {
						m.subCycleChoice++
					}
					return m, nil
				}
			case "tab", "shift+tab", "enter", "up", "down":
				s := msg.String()
				totalFields := 4

				if s == "enter" && m.focusIndex == totalFields-1 {
					cmd := m.saveForm()
					m.goBack()
					return m, cmd
				}
				if s == "up" || s == "shift+tab" {
					m.focusIndex--
				} else if s == "down" || s == "tab" || s == "enter" {
					m.focusIndex++
				}
				if m.focusIndex > totalFields-1 {
					m.focusIndex = 0
				} else if m.focusIndex < 0 {
					m.focusIndex = totalFields - 1
				}

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
			return m, m.updateInputs(msg)
		}

		// --- MAIN APP NAVIGATION ---
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
			if m.state == stateSubs {
				limit = len(m.subItems) - 1
			}
			if m.state == stateStudy {
				limit = len(m.studyItems) - 1
			}
			if m.state == stateFoodBuy {
				limit = len(m.buyChoices) - 1
			}
			if m.cursor < limit {
				m.cursor++
			}

		case "a":
			if m.state == stateFood {
				m.state = stateAddFood
				m.editIndex = -1
				m.initForm(stateAddFood, false)
			} else if m.state == stateSubs {
				m.state = stateAddSub
				m.editIndex = -1
				m.initForm(stateAddSub, false)
			}

		case "e":
			if m.state == stateFood && len(m.foodItems) > 0 {
				m.state = stateAddFood
				m.editIndex = m.cursor
				m.initForm(stateAddFood, true)
			} else if m.state == stateSubs && len(m.subItems) > 0 {
				m.state = stateAddSub
				m.editIndex = m.cursor
				m.initForm(stateAddSub, true)
			}

		case "d":
			m.statusMsg = "Syncing deletion..."
			if m.state == stateFood && len(m.foodItems) > 0 {
				m.foodItems = append(m.foodItems[:m.cursor], m.foodItems[m.cursor+1:]...)
				if m.cursor >= len(m.foodItems) && len(m.foodItems) > 0 {
					m.cursor = len(m.foodItems) - 1
				} else if len(m.foodItems) == 0 {
					m.cursor = 0
				}
				return m, syncCategoryCmd(m.token, "Food", m.catIDs["Food"], m.foodItems)
			} else if m.state == stateSubs && len(m.subItems) > 0 {
				m.subItems = append(m.subItems[:m.cursor], m.subItems[m.cursor+1:]...)
				if m.cursor >= len(m.subItems) && len(m.subItems) > 0 {
					m.cursor = len(m.subItems) - 1
				} else if len(m.subItems) == 0 {
					m.cursor = 0
				}
				return m, syncCategoryCmd(m.token, "Subscriptions", m.catIDs["Subscriptions"], m.subItems)
			}

		// Replace your old case "s" with this:
		case "s":
			if m.state == stateStudy {
				m.state = stateScrapingCanvas
				return m, scrapeCanvasCmd(m.token) // Pass the token here
			}

		// ADD TO CART / REDUCE FROM CART
		case "right", "+":
			if m.state == stateFood && len(m.foodItems) > 0 {
				m.foodItems[m.cursor].CartQty++
			}
		case "left", "-":
			if m.state == stateFood && len(m.foodItems) > 0 {
				if m.foodItems[m.cursor].CartQty > 0 {
					m.foodItems[m.cursor].CartQty--
				}
			}
		case " ":
			if m.state == stateFood && len(m.foodItems) > 0 {
				if m.foodItems[m.cursor].CartQty == 0 {
					m.foodItems[m.cursor].CartQty = 1
				} else {
					m.foodItems[m.cursor].CartQty = 0
				}
			}

		case "r":
			if m.state == stateFood {
				m.state = stateFoodRecipe
				m.isGenerating = true
				m.generatedRecipe = "⏳ Connecting to API and generating recipe..."

				var ingredients []string
				for _, item := range m.foodItems {
					if item.CartQty > 0 {
						ingredients = append(ingredients, item.Name)
					}
				}
				return m, generateRecipeCmd(ingredients)
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
				m.state = stateProcessingBuy
				return m, processBuyCmd()
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

func (m *model) saveForm() tea.Cmd {
	name := m.inputs[0].Value()
	if name == "" {
		return nil
	}
	m.statusMsg = "Syncing..."

	if m.state == stateAddFood {
		price, _ := strconv.ParseFloat(m.inputs[1].Value(), 64)
		amount, _ := strconv.Atoi(m.inputs[2].Value())
		if amount < 0 {
			amount = 0
		}
		thresh, _ := strconv.Atoi(m.inputs[3].Value())

		// --- NEW: AUTO-RENEW LOGIC ---
		// If threshold is enabled (> 0) and the stock drops to or below the threshold
		if thresh > 0 && amount <= thresh {
			amount += 3 // Automatically buy 3 more
			// Set a custom status message to inform the user!
			m.statusMsg = fmt.Sprintf("Auto-renew triggered! +3 %s bought 🚚", name)
		}

		newItem := FoodItem{Name: name, Price: price, Amount: amount, RenewThreshold: thresh, CartQty: 0}

		if m.editIndex >= 0 {
			newItem.CartQty = m.foodItems[m.editIndex].CartQty
			m.foodItems[m.editIndex] = newItem
		} else {
			m.foodItems = append(m.foodItems, newItem)
		}

		return syncCategoryCmd(m.token, "Food", m.catIDs["Food"], m.foodItems)

	} else if m.state == stateAddSub {
		price, _ := strconv.ParseFloat(m.inputs[1].Value(), 64)
		date := m.inputs[2].Value()
		if date == "" {
			date = "TBD"
		}
		cycle := m.subCycleChoices[m.subCycleChoice]

		newItem := SubItem{Name: name, Price: price, DueDate: date, Cycle: cycle}
		if m.editIndex >= 0 {
			m.subItems[m.editIndex] = newItem
		} else {
			m.subItems = append(m.subItems, newItem)
		}
		return syncCategoryCmd(m.token, "Subscriptions", m.catIDs["Subscriptions"], m.subItems)
	}
	return nil
}

func (m *model) goBack() {
	if m.state == stateFoodRecipe || m.state == stateFoodBuy || m.state == stateAddFood || m.state == stateProcessingBuy {
		m.state = stateFood
	} else if m.state == stateAddSub {
		m.state = stateSubs
	} else if m.state == stateScrapingCanvas {
		m.state = stateStudy
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
		for i := range m.inputs {
			s += m.inputs[i].View() + "\n"
		}

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
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Render(fmt.Sprintf("🔑 Auth: %s | %s", m.token, m.statusMsg)) + "\n\n"
		s += renderList(m.menuChoices, m.cursor)
		s += "\n" + hintStyle.Render("[up/down: Navigate • Enter: Select • q: Quit]")

	case stateFood:
		s += titleStyle.Render("🛒 FOOD - Inventory & Cart") + "\n"
		if len(m.foodItems) == 0 {
			s += "    No items. Press 'a' to add one.\n"
		} else {
			for i, item := range m.foodItems {
				cursor := "  "
				if m.cursor == i {
					cursor = "▶ "
				}

				cartIndicator := "[ ]"
				if item.CartQty > 0 {
					cartIndicator = checkStyle.Render(fmt.Sprintf("[%2d]", item.CartQty))
				} else {
					cartIndicator = "[  ]"
				}

				nameCol := lipgloss.NewStyle().Width(18).Render(item.Name)
				renewTag := "       "
				if item.RenewThreshold > 0 {
					renewTag = lipgloss.NewStyle().Width(7).Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#E1B12C")).Render(fmt.Sprintf("[R≤%d]", item.RenewThreshold)))
				}

				line := fmt.Sprintf("  %s %s %s (Stock: %2d) %s -  $%.2f", cursor, cartIndicator, nameCol, item.Amount, renewTag, item.Price)
				if m.cursor == i {
					s += selStyle.Render(line) + "\n"
				} else {
					s += itemStyle.Render(line) + "\n"
				}
			}
		}
		s += "\n" + hintStyle.Render("[Left/Right: Add Qty • a: Add • e: Edit • d: Del • r: Recipe • c: Checkout]")
		s += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Render(m.statusMsg)

	case stateFoodRecipe:
		s += titleStyle.Render("🍳 GENERATED RECIPE (API)") + "\n\n"
		if m.isGenerating {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("#E1B12C")).Render(m.generatedRecipe)
		} else {
			s += boxStyle.Render(m.generatedRecipe)
		}
		if !m.isGenerating {
			s += "\n\n" + hintStyle.Render("[Esc: Back]")
		}

	case stateFoodBuy:
		s += titleStyle.Render("🚚 CHECKOUT") + "\n"
		var total float64
		var count int
		var cartSummary string

		for _, item := range m.foodItems {
			if item.CartQty > 0 {
				cost := item.Price * float64(item.CartQty)
				total += cost
				count++
				cartSummary += fmt.Sprintf("  %dx %-15s - $%.2f\n", item.CartQty, item.Name, cost)
			}
		}

		if count == 0 {
			s += boxStyle.Render("🛒 Cart empty.\nGo back and press Right Arrow to add items to cart.")
		} else {
			s += fmt.Sprintf("Items in Cart:\n%s\nSubtotal: $%.2f\n\nChoose delivery:\n\n", cartSummary, total)
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
			ship := 0.0
			if m.cursor == 0 {
				ship = 3.00
			}
			s += fmt.Sprintf("\n💰 TOTAL TO PAY: $%.2f\n", total+ship)
		}
		s += "\n" + hintStyle.Render("[Enter: Buy • Esc: Cancel]")

	case stateProcessingBuy:
		s += titleStyle.Render("🚚 PROCESSING ORDER") + "\n\n"
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("#E1B12C")).Render("⏳ Please wait, securely placing your order and processing payment...")
		s += "\n\n" + hintStyle.Render("[Processing... please do not close]")

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
		s += "\n" + hintStyle.Render("[a: Add • e: Edit • d: Delete • up/down: Navigate • Esc: Back]")
		s += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Render(m.statusMsg)

	// NEW: The loading screen that shows while fetching Canvas assignments
	case stateScrapingCanvas:
		s += titleStyle.Render("📚 ACADEMICS (Automated Scraper)") + "\n\n"
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("#E1B12C")).Render("⏳ Connecting to Canvas LMS... bypassing CAPTCHA... extracting assignments...")
		s += "\n\n" + hintStyle.Render("[Scraping... please wait]")

	case stateStudy:
		s += titleStyle.Render("📚 ACADEMICS") + "\n\n"
		if len(m.studyItems) == 0 {
			s += "    No pending assignments.\n"
		} else {
			for i, item := range m.studyItems {
				cursor := "  "
				if m.cursor == i {
					cursor = "▶ "
				}
				nameCol := lipgloss.NewStyle().Width(35).Render(item.Name)
				line := fmt.Sprintf("  %s %s | %s", cursor, nameCol, item.DueDate)
				if m.cursor == i {
					s += selStyle.Render(line) + "\n"
				} else {
					s += itemStyle.Render(line) + "\n"
				}
			}
		}
		s += "\n" + hintStyle.Render("[s: Sync Canvas • up/down: Navigate • Esc: Back]")
		s += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Render(m.statusMsg)
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
