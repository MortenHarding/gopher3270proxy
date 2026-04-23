package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

// GopherType represents the item type in a gopher menu
type GopherType byte

const (
	TypeText GopherType = '0' // Plain text file
	TypeMenu GopherType = '1' // Gopher menu/directory
	//TypeCSOPhone  GopherType = '2' // CSO phone book server. Disabled
	TypeError     GopherType = '3' // Error
	TypeBinhex    GopherType = '4' // BinHex encoded file
	TypeDOS       GopherType = '5' // DOS binary archive
	TypeUUEncoded GopherType = '6' // UU-encoded file
	TypeSearch    GopherType = '7' // Index-search server
	TypeTelnet    GopherType = '8' // Telnet session
	TypeBinary    GopherType = '9' // Binary file
	TypeMirror    GopherType = '+' // Mirror
	TypeDoc       GopherType = 'd' // Document type e.g. pdf
	TypeGIF       GopherType = 'g' // GIF image
	TypeImage     GopherType = 'I' // Image
	TypeTN3270    GopherType = 'T' // TN3270 session
	TypeHTML      GopherType = 'h' // HTML file
	TypeInfo      GopherType = 'i' // Informational message
	TypeSound     GopherType = 's' // Sound file
)

// GopherItem represents a single item in a gopher menu
type GopherItem struct {
	Type     GopherType
	Display  string
	Selector string
	Host     string
	Port     int
}

// GopherLocation represents a gopher URL
type GopherLocation struct {
	Host     string
	Port     int
	Selector string
	Type     GopherType
}

func (g GopherLocation) String() string {
	sel := g.Selector
	if sel == "" {
		sel = "/"
	}
	return fmt.Sprintf("gopher://%s:%d/%c%s", g.Host, g.Port, g.Type, sel)
}

// FetchGopherMenu fetches and parses a gopher menu
func FetchGopherMenu(host string, port int, selector string) ([]GopherItem, error) {
	conn, err := dialGopher(host, port)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := sendSelector(conn, selector); err != nil {
		return nil, err
	}

	var items []GopherItem
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "." {
			break
		}
		if len(line) == 0 {
			continue
		}

		item := parseGopherLine(line)
		items = append(items, item)
	}
	return items, scanner.Err()
}

// FetchGopherText fetches a gopher text file and returns lines
func FetchGopherText(host string, port int, selector string) ([]string, error) {
	conn, err := dialGopher(host, port)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := sendSelector(conn, selector); err != nil {
		return nil, err
	}

	var lines []string
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "." {
			break
		}
		// Strip leading dot-stuffing per RFC 1436
		if strings.HasPrefix(line, "..") {
			line = line[1:]
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}

// FetchGopherSearch sends a query to a gopher search server and returns menu items
func FetchGopherSearch(host string, port int, selector string, query string) ([]GopherItem, error) {
	conn, err := dialGopher(host, port)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	searchSelector := selector + "\t" + query
	if err := sendSelector(conn, searchSelector); err != nil {
		return nil, err
	}

	var items []GopherItem
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "." {
			break
		}
		if len(line) == 0 {
			continue
		}
		item := parseGopherLine(line)
		items = append(items, item)
	}
	return items, scanner.Err()
}

func dialGopher(host string, port int) (net.Conn, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
	}
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	return conn, nil
}

func sendSelector(conn net.Conn, selector string) error {
	_, err := fmt.Fprintf(conn, "%s\r\n", selector)
	return err
}

// knownGopherTypes is the set of type characters defined by RFC 1436 and common extensions.
var knownGopherTypes = map[GopherType]bool{
	//TypeCSOPhone: true,
	TypeText: true, TypeMenu: true, TypeDoc: true, TypeError: true,
	TypeBinhex: true, TypeDOS: true, TypeUUEncoded: true, TypeSearch: true,
	TypeTelnet: true, TypeBinary: true, TypeMirror: true, TypeGIF: true,
	TypeImage: true, TypeTN3270: true, TypeHTML: true, TypeInfo: true,
	TypeSound: true,
}

func parseGopherLine(line string) GopherItem {
	if len(line) == 0 {
		return GopherItem{Type: TypeInfo, Display: ""}
	}

	itemType := GopherType(line[0])

	// If the first character is not a recognised gopher type, the server sent
	// a raw text line without the leading 'i' prefix. Treat it as TypeInfo.
	if !knownGopherTypes[itemType] {
		return GopherItem{Type: TypeInfo, Display: line}
	}

	rest := line[1:]
	parts := strings.Split(rest, "\t")

	item := GopherItem{Type: itemType}
	if len(parts) > 0 {
		item.Display = parts[0]
	}
	if len(parts) > 1 {
		item.Selector = parts[1]
	}
	if len(parts) > 2 {
		item.Host = parts[2]
	}
	if len(parts) > 3 {
		fmt.Sscanf(parts[3], "%d", &item.Port)
	}
	if item.Port == 0 {
		item.Port = 70
	}
	return item
}

// TypeLabel returns a short label for display
func (t GopherType) TypeLabel() string {
	switch t {
	case TypeText:
		return "TXT"
	case TypeMenu:
		return "DIR"
	case TypeSearch:
		return "SRC"
	case TypeHTML:
		return "WEB"
	case TypeImage, TypeGIF:
		return "IMG"
	case TypeBinary, TypeDOS, TypeBinhex, TypeUUEncoded:
		return "BIN"
	case TypeTelnet, TypeTN3270:
		return "TEL"
	case TypeError:
		return "ERR"
	case TypeDoc:
		return "PDF"
	case TypeInfo:
		return "   "
	default:
		return fmt.Sprintf(" %c ", t)
	}
}

// IsNavigable returns true if this item can be followed
func (t GopherType) IsNavigable() bool {
	switch t {
	case TypeText, TypeMenu, TypeSearch, TypeHTML:
		return true
	}
	return false
}
