package main

import (
	"fmt"
	"strings"
)

// Screen3270 builds a 3270 datastream buffer
type Screen3270 struct {
	buf []byte
}

// NewScreen starts a new Erase/Write command
func NewScreen(alternate bool) *Screen3270 {
	s := &Screen3270{}
	if alternate {
		s.buf = append(s.buf, CMD_EWA)
	} else {
		s.buf = append(s.buf, CMD_EW)
	}
	// WCC: reset + keyboard restore + reset MDT
	s.buf = append(s.buf, WCC_RESET|WCC_KEYBOARD_RESTORE)
	return s
}

// MoveTo sets buffer address
func (s *Screen3270) MoveTo(row, col int) *Screen3270 {
	s.buf = append(s.buf, ORDER_SBA)
	s.buf = append(s.buf, bufferAddress(row, col)...)
	return s
}

// StartField sets a field attribute at current position
func (s *Screen3270) StartField(attr byte) *Screen3270 {
	s.buf = append(s.buf, ORDER_SF, attr)
	return s
}

// StartFieldExtended sets extended field attributes (color, highlight)
func (s *Screen3270) StartFieldExtended(fieldAttr byte, color byte, highlight byte) *Screen3270 {
	s.buf = append(s.buf, ORDER_SFE)
	count := byte(0)
	pairs := []byte{}
	if fieldAttr != 0 {
		pairs = append(pairs, ATTR_3270, fieldAttr)
		count++
	}
	if color != COLOR_DEFAULT {
		pairs = append(pairs, ATTR_FOREGROUND, color)
		count++
	}
	if highlight != HIGHLIGHT_DEFAULT {
		pairs = append(pairs, ATTR_HIGHLIGHTING, highlight)
		count++
	}
	s.buf = append(s.buf, count)
	s.buf = append(s.buf, pairs...)
	return s
}

// InsertCursor places the cursor at the current position
func (s *Screen3270) InsertCursor() *Screen3270 {
	s.buf = append(s.buf, ORDER_IC)
	return s
}

// WriteEBCDIC writes raw EBCDIC bytes
func (s *Screen3270) WriteEBCDIC(b []byte) *Screen3270 {
	s.buf = append(s.buf, b...)
	return s
}

// WriteText writes text into the 3270 datastream.
//
// TN3270 encoding is client-dependent:
//   - Strictly compliant clients (IBM PCOMM, Attachmate) expect EBCDIC from the server.
//   - Many modern clients (MochaSoft, some x3270 builds) do their own translation
//     and expect ASCII passthrough — sending EBCDIC causes double-translation garbage.
//
// The asciiMode flag (set via -ascii CLI flag) selects the encoding at startup.
// Default is EBCDIC (correct per RFC 1576); use -ascii for MochaSoft and similar.
func (s *Screen3270) WriteText(text string) *Screen3270 {
	if asciiMode {
		for i := 0; i < len(text); i++ {
			c := text[i]
			if c > 0x7E {
				c = 0x20 // replace non-ASCII with space
			}
			s.buf = append(s.buf, c)
		}
	} else {
		s.buf = append(s.buf, toEBCDIC(text)...)
	}
	return s
}

// WriteTextAt moves to row/col and writes text
func (s *Screen3270) WriteTextAt(row, col int, text string) *Screen3270 {
	return s.MoveTo(row, col).WriteText(text)
}

// WriteProtectedAt writes protected (non-editable) text at row/col
func (s *Screen3270) WriteProtectedAt(row, col int, text string, color byte) *Screen3270 {
	s.MoveTo(row, col)
	s.StartFieldExtended(FA_PROTECTED_NORMAL, color, HIGHLIGHT_DEFAULT)
	s.WriteText(text)
	return s
}

// WriteIntenseAt writes intensified protected text
func (s *Screen3270) WriteIntenseAt(row, col int, text string, color byte) *Screen3270 {
	s.MoveTo(row, col)
	s.StartFieldExtended(FA_PROTECTED_INTENSE, color, HIGHLIGHT_DEFAULT)
	s.WriteText(text)
	return s
}

// InputFieldAt creates an input field at row/col with given width
func (s *Screen3270) InputFieldAt(row, col int, width int) *Screen3270 {
	s.MoveTo(row, col)
	s.StartFieldExtended(FA_UNPROTECTED_NORMAL, COLOR_WHITE, HIGHLIGHT_DEFAULT)
	// Fill with spaces to set field width
	s.WriteText(strings.Repeat(" ", width))
	return s
}

// RepeatChar fills from current to address with a character
func (s *Screen3270) RepeatToAddr(row, col int, ch byte) *Screen3270 {
	s.buf = append(s.buf, ORDER_RA)
	s.buf = append(s.buf, bufferAddress(row, col)...)
	s.buf = append(s.buf, ch)
	return s
}

// Bytes returns the complete datastream
func (s *Screen3270) Bytes() []byte {
	return s.buf
}

// WrapText wraps a string to fit within width columns, returning lines
func WrapText(text string, width int) []string {
	if len(text) <= width {
		return []string{text}
	}
	var lines []string
	for len(text) > width {
		// Try to break at a space
		breakAt := width
		for i := width; i > width/2; i-- {
			if text[i] == ' ' {
				breakAt = i
				break
			}
		}
		lines = append(lines, text[:breakAt])
		text = strings.TrimLeft(text[breakAt:], " ")
	}
	if len(text) > 0 {
		lines = append(lines, text)
	}
	return lines
}

// TruncatePad truncates or pads a string to exactly n chars
func TruncatePad(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

// FormatStatusBar builds the bottom status bar string
func FormatStatusBar(location string, page, totalPages int) string {
	right := fmt.Sprintf("Page %d/%d  PF3=Back PF7=Up PF8=Dn", page, totalPages)
	left := TruncatePad("URL: "+location, SCREEN_COLS-len(right)-1)
	return left + right
}
