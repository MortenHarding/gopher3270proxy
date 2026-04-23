package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

// SessionState tracks what the session is currently displaying
type SessionState int

const (
	StateMenu   SessionState = iota // Showing a gopher menu
	StateText                       // Showing a text document
	StateSearch                     // Waiting for search input
	StateHelp                       // Showing help screen
)

// HistoryEntry is a breadcrumb for back navigation
type HistoryEntry struct {
	Location GopherLocation
	Page     int
}

// Session represents a single TN3270 client connection
type Session struct {
	conn    net.Conn
	host    string
	port    int
	verbose bool

	// TN3270 negotiation state
	termType   string
	binaryMode bool
	eorMode    bool
	negotiated bool

	// Navigation state
	state   SessionState
	history []HistoryEntry
	current GopherLocation

	// Display state for menus
	menuItems []GopherItem
	menuPage  int
	menuRows  int // navigable rows per page

	// Display state for text
	textLines []string
	textPage  int

	// Search state
	searchItem  *GopherItem
	searchQuery string
	searchInput []byte // EBCDIC input buffer

	// Selected menu item index (0-based among navigable items)
	selectedIdx int

	// Input buffer for URL bar (future use)
	inputBuf string
}

const (
	menuContentRows = 18 // rows available for menu content (24 - header - status - padding)
	textContentRows = 20 // rows for text content
)

func NewSession(conn net.Conn, host string, port int, verbose bool) *Session {
	return &Session{
		conn:    conn,
		host:    host,
		port:    port,
		verbose: verbose,
		current: GopherLocation{Host: host, Port: port, Selector: "", Type: TypeMenu},
	}
}

func (s *Session) Run() {
	defer s.conn.Close()
	defer log.Printf("[%s] Session ended", s.conn.RemoteAddr())

	if err := s.negotiate(); err != nil {
		log.Printf("[%s] Negotiation failed: %v", s.conn.RemoteAddr(), err)
		return
	}
	log.Printf("[%s] Negotiation complete, terminal: %s", s.conn.RemoteAddr(), s.termType)

	// Load the initial gopher menu
	if err := s.navigateTo(s.current); err != nil {
		s.showError(fmt.Sprintf("Cannot connect: %v", err))
	}

	// Main input loop
	for {
		s.conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		aid, cursorAddr, fields, err := s.readAID()
		if err != nil {
			if err != io.EOF {
				log.Printf("[%s] Read error: %v", s.conn.RemoteAddr(), err)
			}
			return
		}
		if s.verbose {
			log.Printf("[%s] AID=0x%02X cursor=%d fields=%d", s.conn.RemoteAddr(), aid, cursorAddr, len(fields))
		}
		s.handleAID(aid, cursorAddr, fields)
	}
}

// negotiate performs TN3270 telnet option negotiation
func (s *Session) negotiate() error {
	// Send our initial WILL/DO proposals
	s.sendTelnet(WILL, OPT_EOR)
	s.sendTelnet(WILL, OPT_BINARY)
	s.sendTelnet(DO, OPT_BINARY)
	s.sendTelnet(DO, OPT_TERMINAL_TYPE)
	s.sendTelnet(DO, OPT_EOR)

	deadline := time.Now().Add(15 * time.Second)
	buf := make([]byte, 512)

	for time.Now().Before(deadline) {
		s.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := s.conn.Read(buf)
		if err != nil {
			return fmt.Errorf("read during negotiation: %w", err)
		}
		data := buf[:n]
		done, err := s.processNegotiation(data)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
	// Accept even if we didn't complete perfectly — some clients are minimal
	return nil
}

// processNegotiation handles incoming telnet negotiation bytes
// Returns true when we consider negotiation complete
func (s *Session) processNegotiation(data []byte) (bool, error) {
	i := 0
	for i < len(data) {
		if data[i] != IAC {
			i++
			continue
		}
		if i+1 >= len(data) {
			break
		}
		cmd := data[i+1]
		switch cmd {
		case WILL:
			if i+2 >= len(data) {
				break
			}
			opt := data[i+2]
			switch opt {
			case OPT_TERMINAL_TYPE:
				// Ask for terminal type
				s.sendSB(OPT_TERMINAL_TYPE, []byte{TELQUAL_SEND})
			case OPT_EOR:
				s.eorMode = true
				s.sendTelnet(DO, OPT_EOR)
			case OPT_BINARY:
				s.binaryMode = true
			}
			i += 3
		case DO:
			if i+2 >= len(data) {
				break
			}
			opt := data[i+2]
			switch opt {
			case OPT_EOR:
				s.eorMode = true
			case OPT_BINARY:
				s.binaryMode = true
			case OPT_TERMINAL_TYPE:
				// Fine
			}
			i += 3
		case WONT, DONT:
			i += 3
		case SB:
			// Subnegotiation
			end := i + 2
			for end < len(data)-1 && !(data[end] == IAC && data[end+1] == SE) {
				end++
			}
			if end+1 < len(data) {
				sbData := data[i+2 : end]
				s.handleSB(sbData)
				i = end + 2
			} else {
				i = len(data)
			}
		case SE:
			i += 2
		case EOR:
			i += 2
			// EOR received — negotiation round-trip done
			s.negotiated = true
			return true, nil
		default:
			i += 2
		}
	}
	// If we have terminal type and binary+eor modes, consider done
	if s.termType != "" && s.binaryMode && s.eorMode {
		s.negotiated = true
		return true, nil
	}
	return false, nil
}

func (s *Session) handleSB(data []byte) {
	if len(data) < 2 {
		return
	}
	opt := data[0]
	qual := data[1]
	if opt == OPT_TERMINAL_TYPE && qual == TELQUAL_IS {
		s.termType = strings.ToUpper(string(data[2:]))
		log.Printf("[%s] Terminal type: %s", s.conn.RemoteAddr(), s.termType)
	}
}

// sendTelnet sends a 3-byte IAC command
func (s *Session) sendTelnet(cmd, opt byte) {
	s.conn.Write([]byte{IAC, cmd, opt})
}

// sendSB sends an IAC SB ... IAC SE subnegotiation
func (s *Session) sendSB(opt byte, data []byte) {
	buf := []byte{IAC, SB, opt}
	buf = append(buf, data...)
	buf = append(buf, IAC, SE)
	s.conn.Write(buf)
}

// send3270 sends a 3270 datastream wrapped in telnet EOR
func (s *Session) send3270(data []byte) error {
	// Escape IAC bytes in the datastream
	escaped := make([]byte, 0, len(data)+8)
	for _, b := range data {
		escaped = append(escaped, b)
		if b == IAC {
			escaped = append(escaped, IAC) // double IAC to escape
		}
	}
	// Wrap with IAC EOR
	escaped = append(escaped, IAC, EOR)
	_, err := s.conn.Write(escaped)
	return err
}

// readAID reads a 3270 AID response from the client
// Returns: AID byte, cursor address, map of field addr->EBCDIC data
func (s *Session) readAID() (byte, int, map[int][]byte, error) {
	// Read raw bytes until IAC EOR
	var raw []byte
	buf := make([]byte, 1)
	for {
		s.conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		_, err := io.ReadFull(s.conn, buf)
		if err != nil {
			return 0, 0, nil, err
		}
		b := buf[0]
		if b == IAC {
			// Read next byte
			_, err = io.ReadFull(s.conn, buf)
			if err != nil {
				return 0, 0, nil, err
			}
			next := buf[0]
			if next == EOR {
				break // end of 3270 record
			} else if next == IAC {
				raw = append(raw, IAC) // escaped IAC
			}
			// Other telnet commands during data — ignore
		} else {
			raw = append(raw, b)
		}
	}

	if len(raw) < 3 {
		// Just an AID with no cursor (e.g., PA keys or short response)
		if len(raw) >= 1 {
			return raw[0], 0, nil, nil
		}
		return AID_NO_AID, 0, nil, nil
	}

	aid := raw[0]
	cursorAddr := decode3270Addr(raw[1], raw[2])

	// Parse modified fields
	fields := make(map[int][]byte)
	i := 3
	for i < len(raw) {
		if raw[i] == ORDER_SBA {
			if i+2 >= len(raw) {
				break
			}
			fieldAddr := decode3270Addr(raw[i+1], raw[i+2])
			i += 3
			// Collect field data until next order or end
			var fieldData []byte
			for i < len(raw) && raw[i] != ORDER_SBA && raw[i] != ORDER_SF && raw[i] != ORDER_IC {
				fieldData = append(fieldData, raw[i])
				i++
			}
			fields[fieldAddr] = fieldData
		} else {
			i++
		}
	}

	return aid, cursorAddr, fields, nil
}

// handleAID processes an AID from the client and updates display
func (s *Session) handleAID(aid byte, cursorAddr int, fields map[int][]byte) {
	switch s.state {
	case StateSearch:
		s.handleSearchAID(aid, fields)
		return
	}

	switch aid {
	case AID_ENTER:
		s.handleEnter(cursorAddr, fields)
	case AID_PF3:
		s.navigateBack()
	case AID_PF7:
		s.pageUp()
	case AID_PF8:
		s.pageDown()
	case AID_PF1:
		s.showHelpScreen()
	case AID_PF12:
		// Quit / return to home
		s.history = nil
		s.navigateTo(GopherLocation{Host: s.host, Port: s.port, Selector: "", Type: TypeMenu})
	case AID_CLEAR:
		s.redisplay()
	case AID_PA1, AID_PA2:
		// PA keys — refresh
		s.redisplay()
	}
}

// handleEnter processes Enter key — follow selected menu item, submit URL bar, or scroll text
func (s *Session) handleEnter(cursorAddr int, fields map[int][]byte) {
	if s.state == StateText {
		// In text mode, Enter scrolls down
		s.pageDown()
		return
	}

	row := cursorAddr / SCREEN_COLS

	// Row 1 is the editable URL bar. The unprotected field starts at col 0
	// (field attribute) with text from col 1 onward; the client reports the
	// field address as col 1 (first data cell).
	const urlFieldAddr = 1*SCREEN_COLS + 1
	if urlData, ok := fields[urlFieldAddr]; ok && len(urlData) > 0 {
		var raw string
		if asciiMode {
			raw = strings.TrimSpace(strings.Map(func(r rune) rune {
				if r >= 0x20 && r <= 0x7E {
					return r
				}
				return -1
			}, string(urlData)))
		} else {
			raw = strings.TrimSpace(fromEBCDIC(urlData))
		}
		if raw != "" && raw != s.current.String() {
			loc, err := parseGopherURL(raw)
			if err != nil {
				s.showError(fmt.Sprintf("Invalid URL: %v", err))
				return
			}
			s.pushHistory()
			s.navigateTo(loc)
			return
		}
	}

	// Otherwise navigate by cursor row into the menu
	const headerRows = 3
	if row < headerRows {
		return
	}
	itemIdx := s.menuPage*menuContentRows + (row - headerRows)
	if itemIdx < 0 || itemIdx >= len(s.menuItems) {
		return
	}
	item := s.menuItems[itemIdx]
	if item.Type.IsNavigable() {
		s.followItem(item)
	}
}

// followItem navigates to a gopher item
func (s *Session) followItem(item GopherItem) {
	loc := GopherLocation{
		Host:     item.Host,
		Port:     item.Port,
		Selector: item.Selector,
		Type:     item.Type,
	}
	switch item.Type {
	case TypeMenu:
		s.pushHistory()
		s.navigateTo(loc)
	case TypeText:
		s.pushHistory()
		s.navigateTo(loc)
	case TypeSearch:
		s.searchItem = &item
		s.state = StateSearch
		s.searchQuery = ""
		s.showSearchScreen(item)
	default:
		s.showError(fmt.Sprintf("Cannot display type '%c' (%s) in terminal", item.Type, item.Type.TypeLabel()))
	}
}

// parseGopherURL parses a gopher URL string into a GopherLocation.
// Accepts "gopher://host:port/TypeSelector" or bare "host/selector" forms.
func parseGopherURL(raw string) (GopherLocation, error) {
	loc := GopherLocation{Port: 70, Type: TypeMenu}

	// Strip scheme
	s := raw
	if strings.HasPrefix(strings.ToLower(s), "gopher://") {
		s = s[len("gopher://"):]
	}

	// Split host[:port] from /TypeSelector
	slashIdx := strings.Index(s, "/")
	var hostPort, path string
	if slashIdx >= 0 {
		hostPort = s[:slashIdx]
		path = s[slashIdx+1:] // everything after the first /
	} else {
		hostPort = s
		path = ""
	}

	if hostPort == "" {
		return loc, fmt.Errorf("missing host")
	}

	// Parse optional port
	if colonIdx := strings.LastIndex(hostPort, ":"); colonIdx >= 0 {
		portStr := hostPort[colonIdx+1:]
		p, err := strconv.Atoi(portStr)
		if err == nil && p > 0 {
			loc.Port = p
			loc.Host = hostPort[:colonIdx]
		} else {
			loc.Host = hostPort
		}
	} else {
		loc.Host = hostPort
	}

	// First character of path is the item type; rest is the selector
	if len(path) > 0 {
		t := GopherType(path[0])
		if knownGopherTypes[t] {
			loc.Type = t
			loc.Selector = path[1:]
		} else {
			// No type prefix — treat as menu with the full path as selector
			loc.Selector = "/" + path
		}
	}

	return loc, nil
}

// navigateTo fetches and displays a gopher location
func (s *Session) navigateTo(loc GopherLocation) error {
	s.current = loc
	log.Printf("[%s] Navigating to %s", s.conn.RemoteAddr(), loc)

	type result struct {
		menuItems []GopherItem
		textLines []string
		err       error
	}
	ch := make(chan result, 1)

	switch loc.Type {
	case TypeMenu, 0:
		go func() {
			items, err := FetchGopherMenu(loc.Host, loc.Port, loc.Selector)
			ch <- result{menuItems: items, err: err}
		}()
	case TypeText:
		go func() {
			lines, err := FetchGopherText(loc.Host, loc.Port, loc.Selector)
			ch <- result{textLines: lines, err: err}
		}()
	default:
		s.showError(fmt.Sprintf("Unsupported type: %c", loc.Type))
		return nil
	}

	select {
	case r := <-ch:
		if r.err != nil {
			s.showError(fmt.Sprintf("Fetch error: %v", r.err))
			return r.err
		}
		switch loc.Type {
		case TypeMenu, 0:
			s.menuItems = r.menuItems
			s.menuPage = 0
			s.state = StateMenu
			s.renderMenu()
		case TypeText:
			s.textLines = r.textLines
			s.textPage = 0
			s.state = StateText
			s.renderText()
		}
		return nil
	case <-time.After(10 * time.Second):
		err := fmt.Errorf("host %s did not respond within 10 seconds", loc.Host)
		s.showError(err.Error())
		return err
	}
}

func (s *Session) pushHistory() {
	entry := HistoryEntry{Location: s.current}
	if s.state == StateMenu {
		entry.Page = s.menuPage
	} else {
		entry.Page = s.textPage
	}
	s.history = append(s.history, entry)
}

func (s *Session) navigateBack() {
	if len(s.history) == 0 {
		s.showMessage("Already at top of history")
		return
	}
	last := s.history[len(s.history)-1]
	s.history = s.history[:len(s.history)-1]
	s.navigateTo(last.Location)
	// Restore page position
	if s.state == StateMenu {
		s.menuPage = last.Page
		s.renderMenu()
	} else if s.state == StateText {
		s.textPage = last.Page
		s.renderText()
	}
}

func (s *Session) pageUp() {
	switch s.state {
	case StateMenu:
		if s.menuPage > 0 {
			s.menuPage--
			s.renderMenu()
		}
	case StateText:
		if s.textPage > 0 {
			s.textPage--
			s.renderText()
		}
	}
}

func (s *Session) pageDown() {
	switch s.state {
	case StateMenu:
		totalPages := s.menuTotalPages()
		if s.menuPage < totalPages-1 {
			s.menuPage++
			s.renderMenu()
		}
	case StateText:
		totalPages := s.textTotalPages()
		if s.textPage < totalPages-1 {
			s.textPage++
			s.renderText()
		}
	}
}

func (s *Session) menuTotalPages() int {
	// Count ALL items — render paginates by total item index, not just navigable ones.
	total := len(s.menuItems)
	if total == 0 {
		return 1
	}
	pages := total / menuContentRows
	if total%menuContentRows > 0 {
		pages++
	}
	return pages
}

func (s *Session) textTotalPages() int {
	if len(s.textLines) == 0 {
		return 1
	}
	pages := len(s.textLines) / textContentRows
	if len(s.textLines)%textContentRows > 0 {
		pages++
	}
	return pages
}

func (s *Session) redisplay() {
	switch s.state {
	case StateMenu:
		s.renderMenu()
	case StateText:
		s.renderText()
	}
}

// handleSearchAID handles input on the search screen
func (s *Session) handleSearchAID(aid byte, fields map[int][]byte) {
	switch aid {
	case AID_PF3:
		// Cancel search
		s.state = StateMenu
		s.searchItem = nil
		s.renderMenu()
		return
	case AID_ENTER:
		// Read the search field (at row 9, col 22 — matches showSearchScreen MoveTo(9,22))
		searchFieldAddr := 9*SCREEN_COLS + 23
		if data, ok := fields[searchFieldAddr]; ok {
			// Decode search input: in ASCII mode client sends ASCII; otherwise EBCDIC
			if asciiMode {
				s.searchQuery = strings.TrimSpace(strings.Map(func(r rune) rune {
					if r >= 0x20 && r <= 0x7E {
						return r
					}
					return -1
				}, string(data)))
			} else {
				s.searchQuery = strings.TrimSpace(fromEBCDIC(data))
			}
		}
		if s.searchQuery == "" {
			return
		}
		item := s.searchItem
		s.pushHistory()
		items, err := FetchGopherSearch(item.Host, item.Port, item.Selector, s.searchQuery)
		if err != nil {
			s.showError(fmt.Sprintf("Search error: %v", err))
			return
		}
		s.menuItems = items
		s.menuPage = 0
		s.state = StateMenu
		s.current = GopherLocation{Host: item.Host, Port: item.Port, Selector: item.Selector + "?" + s.searchQuery, Type: TypeMenu}
		s.renderMenu()
	case AID_CLEAR, AID_PA1:
		s.state = StateMenu
		s.renderMenu()
	}
}
