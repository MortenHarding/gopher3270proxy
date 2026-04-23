package main

// Telnet commands
const (
	IAC  = 0xFF
	DONT = 0xFE
	DO   = 0xFD
	WONT = 0xFC
	WILL = 0xFB
	SB   = 0xFA // Subnegotiation begin
	SE   = 0xF0 // Subnegotiation end  ← RFC 854: 0xF0
	EOR  = 0xEF // End of record
	NOP  = 0xF1 // No operation (note: same value as AID_PF1 — only relevant in telnet cmd context)
)

// Telnet options
const (
	OPT_BINARY        = 0x00
	OPT_ECHO          = 0x01
	OPT_SGA           = 0x03 // Suppress Go Ahead
	OPT_TERMINAL_TYPE = 0x18
	OPT_EOR           = 0x19 // End of Record
	OPT_TN3270E       = 0x28 // TN3270E extended
)

// Terminal type subnegotiation
const (
	TELQUAL_IS   = 0x00
	TELQUAL_SEND = 0x01
)

// 3270 command codes
const (
	CMD_W   = 0x01 // Write
	CMD_RB  = 0x02 // Read Buffer
	CMD_NOP = 0x03
	CMD_RM  = 0x06 // Read Modified
	CMD_RMA = 0x0E // Read Modified All
	CMD_EW  = 0x05 // Erase/Write
	CMD_EWA = 0x0D // Erase/Write Alternate
	CMD_EAU = 0x0F // Erase All Unprotected
	CMD_WSF = 0x11 // Write Structured Field
)

// 3270 Write Control Character (WCC)
const (
	WCC_RESET          = 0x00
	WCC_SOUND_ALARM    = 0x04
	WCC_KEYBOARD_RESTORE = 0x02
	WCC_RESET_MDT      = 0x01
)

// 3270 Orders
const (
	ORDER_SF  = 0x1D // Start Field
	ORDER_SFE = 0x29 // Start Field Extended
	ORDER_SBA = 0x11 // Set Buffer Address
	ORDER_SA  = 0x28 // Set Attribute
	ORDER_MF  = 0x2C // Modify Field
	ORDER_IC  = 0x13 // Insert Cursor
	ORDER_PT  = 0x05 // Program Tab
	ORDER_RA  = 0x3C // Repeat to Address
	ORDER_EUA = 0x12 // Erase Unprotected to Address
	ORDER_GE  = 0x08 // Graphic Escape
)

// 3270 Attribute types
const (
	ATTR_3270       = 0xC0
	ATTR_HIGHLIGHTING = 0x41
	ATTR_FOREGROUND   = 0x42
	ATTR_CHARSET      = 0x43
	ATTR_FIELD_VALID  = 0xC0
)

// Field attributes
const (
	FA_PROTECT   = 0x20
	FA_NUMERIC   = 0x10
	FA_DISPLAY   = 0x0C
	FA_INTENSIFY = 0x08
	FA_MDT       = 0x01 // Modified Data Tag

	// Combined
	FA_PROTECTED_NORMAL    = 0x60 // Protected, normal intensity
	FA_PROTECTED_INTENSE   = 0x68 // Protected, high intensity
	FA_PROTECTED_INVISIBLE = 0x6C // Protected, non-display
	FA_UNPROTECTED_NORMAL  = 0x40 // Unprotected, normal
	FA_UNPROTECTED_INTENSE = 0x48 // Unprotected, intensified
)

// Extended highlighting values
const (
	HIGHLIGHT_DEFAULT   = 0x00
	HIGHLIGHT_BLINK     = 0xF1
	HIGHLIGHT_REVERSE   = 0xF2
	HIGHLIGHT_UNDERLINE = 0xF4
	HIGHLIGHT_INTENSE   = 0xF8
)

// Extended foreground colors
const (
	COLOR_DEFAULT = 0x00
	COLOR_BLUE    = 0xF1
	COLOR_RED     = 0xF2
	COLOR_PINK    = 0xF3
	COLOR_GREEN   = 0xF4
	COLOR_TURQ    = 0xF5
	COLOR_YELLOW  = 0xF6
	COLOR_WHITE   = 0xF7
)

// AID (Attention Identifier) keys
const (
	AID_NO_AID   = 0x60
	AID_ENTER    = 0x7D
	AID_PF1      = 0xF1
	AID_PF2      = 0xF2
	AID_PF3      = 0xF3
	AID_PF4      = 0xF4
	AID_PF5      = 0xF5
	AID_PF6      = 0xF6
	AID_PF7      = 0xF7
	AID_PF8      = 0xF8
	AID_PF9      = 0xF9
	AID_PF10     = 0x7A
	AID_PF11     = 0x7B
	AID_PF12     = 0x7C
	AID_PF13     = 0xC1
	AID_PF14     = 0xC2
	AID_PF15     = 0xC3
	AID_PA1      = 0x6C
	AID_PA2      = 0x6E
	AID_PA3      = 0x6B
	AID_CLEAR    = 0x6D
	AID_SYSREQ   = 0xF0
)

// Screen dimensions
const (
	SCREEN_COLS = 80
	SCREEN_ROWS = 24
	SCREEN_SIZE = SCREEN_COLS * SCREEN_ROWS
)

// bufferAddress encodes a 3270 buffer address from row/col (0-based).
// Clamps the address to the valid screen range (0 to SCREEN_SIZE-1) so that
// an off-by-one in layout code produces a visible artefact at the screen edge
// rather than a hard "SBA address > maximum" error in the client.
func bufferAddress(row, col int) []byte {
	addr := row*SCREEN_COLS + col
	if addr < 0 {
		addr = 0
	}
	if addr >= SCREEN_SIZE {
		addr = SCREEN_SIZE - 1
	}
	// 12-bit address split into two 6-bit halves, each encoded per the
	// IBM 3270 address code table (high bits 01xxxxxx).
	hi := (addr >> 6) & 0x3F
	lo := addr & 0x3F
	return []byte{encode3270Addr(hi), encode3270Addr(lo)}
}

// encode3270Addr converts a 6-bit value (0–63) to its 3270 wire encoding.
//
// The IBM 3270 spec defines a 64-entry code table that maps each 6-bit value
// to a specific byte.  The table is NOT simply 0x40+v — it follows a pattern
// derived from EBCDIC character codes:
//
//	 0–15  → 0x40, 0xC1–0xC9, 0xD1–0xD9          (space, A–I, J–R)
//	16–31  → 0xE2–0xE9, 0xF0–0xF9                 (S–Z, 0–9)  — note: skips some
//	32–63  → fills remaining slots
//
// The canonical table used by all 3270 implementations:
var addr3270Table = [64]byte{
	0x40, 0xC1, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7,
	0xC8, 0xC9, 0x4A, 0x4B, 0x4C, 0x4D, 0x4E, 0x4F,
	0x50, 0xD1, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7,
	0xD8, 0xD9, 0x5A, 0x5B, 0x5C, 0x5D, 0x5E, 0x5F,
	0x60, 0x61, 0xE2, 0xE3, 0xE4, 0xE5, 0xE6, 0xE7,
	0xE8, 0xE9, 0x6A, 0x6B, 0x6C, 0x6D, 0x6E, 0x6F,
	0xF0, 0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7,
	0xF8, 0xF9, 0x7A, 0x7B, 0x7C, 0x7D, 0x7E, 0x7F,
}

func encode3270Addr(v int) byte {
	if v < 0 || v > 63 {
		return 0x40 // safe fallback
	}
	return addr3270Table[v]
}

// decode3270Addr decodes two bytes of 3270 buffer address to a linear offset.
// Reverses encode3270Addr by looking up each byte in the table.
func decode3270Addr(b1, b2 byte) int {
	return (addr6bit(b1) << 6) | addr6bit(b2)
}

// addr6bit recovers the 6-bit value from a 3270-encoded address byte.
func addr6bit(b byte) int {
	for i, v := range addr3270Table {
		if v == b {
			return i
		}
	}
	// Fallback: strip high 2 bits (handles clients that use simple 0x40|v encoding)
	return int(b & 0x3F)
}
