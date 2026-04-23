package main

import (
	"fmt"
	"log"
	"strings"
)

// renderMenu builds and sends the gopher menu screen
func (s *Session) renderMenu() {
	scr := NewScreen(false)

	// ── Header bar (row 0) ──────────────────────────────────────────────
	scr.MoveTo(0, 0)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_TURQ, HIGHLIGHT_DEFAULT)
	title := TruncatePad(" Gopher 3270 Proxy  ", SCREEN_COLS-12)
	scr.WriteText(title)
	scr.StartFieldExtended(FA_PROTECTED_INTENSE, COLOR_YELLOW, HIGHLIGHT_DEFAULT)
	scr.WriteText(" PF1=Help")

	// ── URL bar (row 1) — editable input field ─────────────────────────
	// The field attribute occupies col 0; the editable text starts at col 1.
	scr.MoveTo(1, 0)
	scr.StartFieldExtended(FA_UNPROTECTED_INTENSE, COLOR_GREEN, HIGHLIGHT_DEFAULT)
	gopherSelector := TruncatePad(s.current.String(), SCREEN_COLS-1)
	scr.MoveTo(1, 1)
	scr.WriteText(gopherSelector)

	// ── Column headers (row 1) ──────────────────────────────────────────
	// Removed the column headers and move the next up 1 line
	//scr.MoveTo(1, 0)
	//scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_GREEN, HIGHLIGHT_DEFAULT)
	//scr.WriteText(TruncatePad("  Type Item", SCREEN_COLS))

	// ── Separator (row 2) ───────────────────────────────────────────────
	scr.MoveTo(2, 0)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_BLUE, HIGHLIGHT_DEFAULT)
	scr.WriteText(strings.Repeat("-", SCREEN_COLS-1))

	// ── Menu items (rows 3–21) ──────────────────────────────────────────
	// Page by total item index so TypeInfo (ascii art, blanks) are included
	// in pagination exactly as they appear on screen.
	startIdx := s.menuPage * menuContentRows
	endIdx := startIdx + menuContentRows
	row := 3

	for i, item := range s.menuItems {
		if i < startIdx {
			continue
		}
		if i >= endIdx || row >= 3+menuContentRows {
			break
		}
		s.renderMenuItem(scr, row, item)
		row++
	}

	// Fill unused rows
	for row < 3+menuContentRows {
		scr.MoveTo(row, 0)
		scr.StartField(FA_PROTECTED_NORMAL)
		scr.WriteText(strings.Repeat(" ", SCREEN_COLS))
		row++
	}

	// ── Separator (row 22) ──────────────────────────────────────────────
	scr.MoveTo(22, 0)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_BLUE, HIGHLIGHT_DEFAULT)
	scr.WriteText(strings.Repeat("-", SCREEN_COLS))

	// ── Status bar (row 23) ─────────────────────────────────────────────
	totalPages := s.menuTotalPages()
	status := fmt.Sprintf(" Gopherspace  Page %d/%d   Enter=Open/Go  PF3=Back  PF7=PgUp  PF8=PgDn  PF12=Home",
		s.menuPage+1, totalPages)
	scr.MoveTo(23, 0)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_YELLOW, HIGHLIGHT_DEFAULT)
	scr.WriteText(TruncatePad(status, SCREEN_COLS))

	// Place cursor on first navigable item row
	firstNavRow := s.firstNavRow()
	scr.MoveTo(firstNavRow, 6)
	scr.InsertCursor()

	if err := s.send3270(scr.Bytes()); err != nil {
		log.Printf("[%s] Send error: %v", s.conn.RemoteAddr(), err)
	}
}

func (s *Session) renderMenuItem(scr *Screen3270, row int, item GopherItem) {
	typeLabel := string(item.Type.TypeLabel())
	display := item.Display

	// Trim display to fit: 2 indent + 4 type+space + content
	maxDisplayWidth := SCREEN_COLS - 6
	if len(display) > maxDisplayWidth {
		display = display[:maxDisplayWidth-1] + ">"
	}

	scr.MoveTo(row, 0)

	// ALL navigable items use FA_PROTECTED — selection is by cursor position,
	// not by field modification. Using FA_UNPROTECTED causes 3270 emulators to
	// treat them as input fields, which right-justifies content and corrupts layout.
	switch item.Type {
	case TypeInfo:
		// Informational / ASCII art — render as-is with no type prefix.
		// Previously had 5 extra spaces that pushed content off-screen.
		scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_WHITE, HIGHLIGHT_DEFAULT)
		scr.WriteText(TruncatePad(display, SCREEN_COLS))

	case TypeError:
		scr.StartFieldExtended(FA_PROTECTED_INTENSE, COLOR_RED, HIGHLIGHT_DEFAULT)
		scr.WriteText(TruncatePad("  ERR "+display, SCREEN_COLS))

	case TypeMenu:
		// Was FA_UNPROTECTED_INTENSE — caused right-justification in emulators
		scr.StartFieldExtended(FA_PROTECTED_INTENSE, COLOR_TURQ, HIGHLIGHT_DEFAULT)
		scr.WriteText(TruncatePad("  (DIR) "+display, SCREEN_COLS))

	case TypeText:
		// Was FA_UNPROTECTED_NORMAL
		scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_GREEN, HIGHLIGHT_DEFAULT)
		scr.WriteText(TruncatePad("  (TXT) "+display, SCREEN_COLS))

	case TypeSearch:
		// Was FA_UNPROTECTED_INTENSE
		scr.StartFieldExtended(FA_PROTECTED_INTENSE, COLOR_YELLOW, HIGHLIGHT_DEFAULT)
		scr.WriteText(TruncatePad("  ("+typeLabel+") "+display, SCREEN_COLS))

	default:
		scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_WHITE, HIGHLIGHT_DEFAULT)
		scr.WriteText(TruncatePad("  ("+typeLabel+") "+display, SCREEN_COLS))
	}
}

// firstNavRow returns the screen row of the first navigable item on this page.
// Uses total-item-index paging to match renderMenu.
func (s *Session) firstNavRow() int {
	const headerRows = 3
	startIdx := s.menuPage * menuContentRows
	endIdx := startIdx + menuContentRows
	for i, item := range s.menuItems {
		if i < startIdx || i >= endIdx {
			continue
		}
		if item.Type.IsNavigable() {
			return headerRows + (i - startIdx)
		}
	}
	return headerRows
}

// renderText builds and sends the text viewer screen
func (s *Session) renderText() {
	scr := NewScreen(false)

	// ── Header ───────────────────────────────────────────────────────────
	scr.MoveTo(0, 0)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_TURQ, HIGHLIGHT_DEFAULT)
	title := TruncatePad(" Gopher3270  "+s.current.String(), SCREEN_COLS)
	scr.WriteText(title)

	// ── Separator ────────────────────────────────────────────────────────
	scr.MoveTo(1, 0)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_BLUE, HIGHLIGHT_DEFAULT)
	scr.WriteText(strings.Repeat("-", SCREEN_COLS))

	// ── Text content (rows 2–21) ─────────────────────────────────────────
	startLine := s.textPage * textContentRows
	row := 2
	for _, rawLine := range s.textLines {
		if startLine > 0 {
			startLine--
			continue
		}
		if row >= 2+textContentRows {
			break
		}

		// Expand tabs and wrap long lines
		expanded := strings.ReplaceAll(rawLine, "\t", "    ")
		wrappedLines := WrapText(expanded, SCREEN_COLS-1)
		for _, wl := range wrappedLines {
			if row >= 2+textContentRows {
				break
			}
			scr.MoveTo(row, 0)
			scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_GREEN, HIGHLIGHT_DEFAULT)
			scr.WriteText(TruncatePad(wl, SCREEN_COLS))
			row++
		}
	}

	// Fill remaining rows
	for row < 2+textContentRows {
		scr.MoveTo(row, 0)
		scr.StartField(FA_PROTECTED_NORMAL)
		scr.WriteText(strings.Repeat(" ", SCREEN_COLS))
		row++
	}

	// ── Separator ────────────────────────────────────────────────────────
	scr.MoveTo(22, 0)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_BLUE, HIGHLIGHT_DEFAULT)
	scr.WriteText(strings.Repeat("-", SCREEN_COLS))

	// ── Status ───────────────────────────────────────────────────────────
	totalPages := s.textTotalPages()
	status := fmt.Sprintf(" Text Document   Page %d/%d   PF3=Back  PF7=PgUp  PF8=PgDn  Enter=Next",
		s.textPage+1, totalPages)
	scr.MoveTo(23, 0)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_YELLOW, HIGHLIGHT_DEFAULT)
	scr.WriteText(TruncatePad(status, SCREEN_COLS))

	// Cursor at top-left of content
	scr.MoveTo(2, 0)
	scr.InsertCursor()

	if err := s.send3270(scr.Bytes()); err != nil {
		log.Printf("(%s) Send error: %v", s.conn.RemoteAddr(), err)
	}
}

// showSearchScreen displays a search input prompt
func (s *Session) showSearchScreen(item GopherItem) {
	scr := NewScreen(false)

	scr.MoveTo(0, 0)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_TURQ, HIGHLIGHT_DEFAULT)
	scr.WriteText(TruncatePad(" Gopher3270  Search", SCREEN_COLS))

	scr.MoveTo(5, 2)
	scr.StartFieldExtended(FA_PROTECTED_INTENSE, COLOR_WHITE, HIGHLIGHT_DEFAULT)
	scr.WriteText("Search Server:")

	scr.MoveTo(6, 4)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_GREEN, HIGHLIGHT_DEFAULT)
	scr.WriteText(TruncatePad(item.Display, SCREEN_COLS-4))

	scr.MoveTo(7, 4)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_TURQ, HIGHLIGHT_DEFAULT)
	scr.WriteText(fmt.Sprintf("%s:%d%s", item.Host, item.Port, item.Selector))

	scr.MoveTo(9, 2)
	scr.StartFieldExtended(FA_PROTECTED_INTENSE, COLOR_WHITE, HIGHLIGHT_DEFAULT)
	scr.WriteText("Enter search query:")

	// Input field at row 9, col 23
	scr.MoveTo(9, 22)
	scr.StartFieldExtended(FA_UNPROTECTED_INTENSE, COLOR_YELLOW, HIGHLIGHT_DEFAULT)
	scr.MoveTo(9, 23)
	scr.InsertCursor()

	scr.MoveTo(12, 2)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_WHITE, HIGHLIGHT_DEFAULT)
	scr.WriteText("Enter=Search   PF3=Cancel")

	scr.MoveTo(23, 0)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_YELLOW, HIGHLIGHT_DEFAULT)
	scr.WriteText(TruncatePad(" Enter query and press Enter, or PF3 to cancel", SCREEN_COLS))

	if err := s.send3270(scr.Bytes()); err != nil {
		log.Printf("(%s) Send error: %v", s.conn.RemoteAddr(), err)
	}
}

// showError displays an error message and returns to menu on next keypress
func (s *Session) showError(msg string) {
	scr := NewScreen(false)

	scr.MoveTo(0, 0)
	scr.StartFieldExtended(FA_PROTECTED_INTENSE, COLOR_RED, HIGHLIGHT_DEFAULT)
	scr.WriteText(TruncatePad(" ERROR", SCREEN_COLS))

	scr.MoveTo(5, 2)
	scr.StartFieldExtended(FA_PROTECTED_INTENSE, COLOR_WHITE, HIGHLIGHT_DEFAULT)
	scr.WriteText("An error occurred:")

	scr.MoveTo(7, 4)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_RED, HIGHLIGHT_DEFAULT)
	// Wrap error message
	lines := WrapText(msg, SCREEN_COLS-6)
	for i, l := range lines {
		if i > 5 {
			break
		}
		scr.WriteTextAt(7+i, 4, l)
	}

	scr.MoveTo(15, 2)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_WHITE, HIGHLIGHT_DEFAULT)
	scr.WriteText("Press PF3 to go back, or Enter to retry")

	scr.MoveTo(23, 0)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_YELLOW, HIGHLIGHT_DEFAULT)
	scr.WriteText(TruncatePad(" Error  PF3=Back  Enter=Retry", SCREEN_COLS))

	scr.MoveTo(15, 2)
	scr.InsertCursor()

	log.Printf("(%s) Error displayed: %s", s.conn.RemoteAddr(), msg)
	s.send3270(scr.Bytes())
}

// showMessage displays a brief status message overlay
func (s *Session) showMessage(msg string) {
	// For brevity, just re-render with message in status bar by doing a partial update
	// Full re-render is simpler and safer
	s.redisplay()
}

// showHelpScreen shows the help/key reference screen
func (s *Session) showHelpScreen() {
	scr := NewScreen(false)

	scr.MoveTo(0, 0)
	scr.StartFieldExtended(FA_PROTECTED_INTENSE, COLOR_TURQ, HIGHLIGHT_DEFAULT)
	scr.WriteText(TruncatePad(" Gopher3270 Proxy  --  Help", SCREEN_COLS))

	help := []struct{ key, desc string }{
		{"Enter", "Open selected menu item / scroll text"},
		{"PF1", "Show this help screen"},
		{"PF3", "Go back in history"},
		{"PF7", "Page up"},
		{"PF8", "Page down"},
		{"PF12", "Go to home server"},
		{"Clear", "Refresh current screen"},
		{"PA1/PA2", "Refresh current screen"},
	}

	scr.MoveTo(2, 2)
	scr.StartFieldExtended(FA_PROTECTED_INTENSE, COLOR_WHITE, HIGHLIGHT_DEFAULT)
	scr.WriteText("Key Bindings:")

	for i, h := range help {
		row := 4 + i
		scr.MoveTo(row, 4)
		scr.StartFieldExtended(FA_PROTECTED_INTENSE, COLOR_YELLOW, HIGHLIGHT_DEFAULT)
		scr.WriteText(TruncatePad(h.key, 10))
		scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_WHITE, HIGHLIGHT_DEFAULT)
		scr.WriteText(h.desc)
	}

	scr.MoveTo(14, 2)
	scr.StartFieldExtended(FA_PROTECTED_INTENSE, COLOR_WHITE, HIGHLIGHT_DEFAULT)
	scr.WriteText("About:")

	scr.MoveTo(15, 4)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_GREEN, HIGHLIGHT_DEFAULT)
	scr.WriteText("Gopher3270 Proxy - A TN3270 gateway to Gopherspace")

	scr.MoveTo(16, 4)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_GREEN, HIGHLIGHT_DEFAULT)
	scr.WriteText("Allows TN3270 terminals to browse the Gopher protocol (RFC 1436)")

	scr.MoveTo(17, 4)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_GREEN, HIGHLIGHT_DEFAULT)
	scr.WriteText("Default server: " + fmt.Sprintf("gopher://%s:%d", s.host, s.port))

	scr.MoveTo(19, 2)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_TURQ, HIGHLIGHT_DEFAULT)
	scr.WriteText("Item type indicators:")

	types := [][2]string{
		{"(DIR)", "Gopher menu / directory"},
		{"(TXT)", "Text document"},
		{"(SRC)", "Search server"},
		{"(BIN)", "Binary file (cannot display)"},
		{"(IMG)", "Image file (cannot display)"},
	}
	for i, t := range types {
		scr.MoveTo(20+i, 4)
		scr.StartFieldExtended(FA_PROTECTED_INTENSE, COLOR_YELLOW, HIGHLIGHT_DEFAULT)
		scr.WriteText(TruncatePad(t[0], 8))
		scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_WHITE, HIGHLIGHT_DEFAULT)
		scr.WriteText(t[1])
	}

	scr.MoveTo(23, 0)
	scr.StartFieldExtended(FA_PROTECTED_NORMAL, COLOR_YELLOW, HIGHLIGHT_DEFAULT)
	scr.WriteText(TruncatePad(" Press PF3 to return", SCREEN_COLS))

	scr.MoveTo(23, 0)
	scr.InsertCursor()

	s.send3270(scr.Bytes())

	// Next keypress returns to previous screen
	// We handle this by not changing s.state — next AID will redisplay
}
