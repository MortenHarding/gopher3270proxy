# Gopher3270 Proxy

A TN3270 gateway that lets IBM 3270 terminals and emulators browse Gopherspace (RFC 1436).
Connect any TN3270 client to the proxy and navigate Gopher menus, read text documents,
and run searches — all rendered on a classic 80×24 mainframe screen.

```
gopher://gopherspace.dk ──► Gopher3270 Proxy :3270 ──► TN3270 Client
```

---

## Features

- Full TN3270/Telnet option negotiation (binary mode, EOR, terminal type)
- EBCDIC ↔ ASCII translation with complete 256-entry lookup tables
- Color-coded menu items — directories (cyan), text (green), search (yellow), errors (red)
- Editable URL bar on the menu screen for direct Gopher URL entry
- Paged text viewer with line wrapping and tab expansion
- Search server support (Gopher type `7`)
- Full navigation history stack — PF3 goes back correctly through multiple hops, restoring page position
- PF1 help screen with key reference and item type legend
- PF12 returns to the configured home server
- Structured logging with timestamps and client IP
- ASCII passthrough mode (`-ascii`) for clients that do their own EBCDIC translation (MochaSoft, etc.)
- Graceful timeout handling — 10-second connect timeout, 5-minute session idle timeout

---

## Requirements

- Go 1.21 or later
- A TN3270 client (IBM PCOMM, Attachmate, x3270, MochaSoft, Vista TN3270, etc.)

---

## Building

```bash
git clone <repo-url>
cd gopher3270proxy
go build -o gopher3270proxy .
```

---

## Running

```bash
./gopher3270proxy [options]
```

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `-host` | `gopherspace.dk` | Default Gopher host to connect to on startup |
| `-port` | `70` | Default Gopher port |
| `-listen` | `3270` | TCP port to listen for TN3270 connections |
| `-log` | *(stdout)* | Log file path |
| `-v` | `false` | Verbose logging (logs AID codes, cursor positions, field counts) |
| `-ascii` | `false` | Send ASCII instead of EBCDIC (see [Encoding](#encoding) below) |

### Example

```bash
./gopher3270proxy \
  -host gopherspace.dk \
  -port 70 \
  -listen 3270 \
  -log /var/log/gopher3270.log \
  -v
```

### Running as a Docker container

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o gopher3270proxy .

FROM alpine:latest
COPY --from=builder /app/gopher3270proxy /usr/local/bin/
EXPOSE 3270
CMD ["gopher3270proxy", "-host", "gopherspace.dk", "-listen", "3270", "-log", "/var/log/gopher3270proxy.log"]
```

```bash
docker build -t gopher3270proxy .
docker run -d -p 3270:3270 \
  -v /var/log:/var/log \
  gopher3270proxy
```

---

## Connecting

Point your TN3270 client at the proxy host on port 3270 (or whichever `-listen` port you chose).
The proxy will negotiate terminal type, enter binary+EOR mode, and immediately load the
configured home Gopher server.

### Client compatibility

| Client | Mode | Notes |
|--------|------|-------|
| IBM PCOMM | EBCDIC (default) | Full support |
| Attachmate EXTRA! | EBCDIC (default) | Full support |
| x3270 / c3270 | EBCDIC (default) | Full support |
| MochaSoft TN3270 | `-ascii` flag | Does its own EBCDIC translation |
| Vista TN3270 | EBCDIC (default) | Full support |

---

## Key Bindings

| Key | Action |
|-----|--------|
| **Enter** | Open selected menu item / type URL in URL bar / scroll text down |
| **PF1** | Show help screen |
| **PF3** | Go back in navigation history |
| **PF7** | Page up |
| **PF8** | Page down |
| **PF12** | Return to home server |
| **Clear** | Refresh current screen |
| **PA1 / PA2** | Refresh current screen |

---

## Screen Layout

### Menu screen

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Gopher 3270 Proxy                                                   PF1=Help │  ← Header (cyan)
│ gopher://gopherspace.dk:70/1                                                 │  ← URL bar (editable, green)
│ ────────────────────────────────────────────────────────────────────────────  │  ← Separator (blue)
│  (DIR) Welcome to Gopherspace                                                │  ← Menu items (rows 3–20)
│  (TXT) About this server                                                     │
│  (SRC) Search Gopherspace                                                    │
│  ...                                                                         │
│                                                                              │
│ ────────────────────────────────────────────────────────────────────────────  │  ← Separator (blue)
│  Gopherspace  Page 1/3   Enter=Open/Go  PF3=Back  PF7=PgUp  PF8=PgDn  ...  │  ← Status bar (yellow)
└──────────────────────────────────────────────────────────────────────────────┘
```

The URL bar (row 1) is an editable input field. You can overtype the current URL and press
Enter to navigate directly to any Gopher address. Accepted formats:

```
gopher://host:port/TypeSelector
gopher://host/TypeSelector
host:port/TypeSelector
host/selector
```

### Text viewer

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Gopher3270  gopher://gopherspace.dk:70/0/about.txt                          │  ← Header (cyan)
│ ────────────────────────────────────────────────────────────────────────────  │
│ Document text content...                                                     │  ← Content (rows 2–21, green)
│ Long lines are automatically wrapped to 79 columns.                          │
│ Tabs are expanded to 4 spaces.                                               │
│ ...                                                                          │
│ ────────────────────────────────────────────────────────────────────────────  │
│  Text Document   Page 1/4   PF3=Back  PF7=PgUp  PF8=PgDn  Enter=Next       │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Search screen

Activated when you select a type `7` (index-search) item. Enter your query and press Enter.
PF3 cancels and returns to the menu.

---

## Item Type Indicators

| Indicator | Gopher Type | Color |
|-----------|-------------|-------|
| `(DIR)` | `1` — Menu / directory | Cyan |
| `(TXT)` | `0` — Text document | Green |
| `(SRC)` | `7` — Search server | Yellow |
| `(BIN)` | `5`/`4`/`6`/`9` — Binary/archive | White |
| `(IMG)` | `I`/`g` — Image / GIF | White |
| `(TEL)` | `8`/`T` — Telnet / TN3270 | White |
| `(PDF)` | `d` — Document (PDF etc.) | White |
| `(ERR)` | `3` — Error message | Red (intense) |
| *(blank)* | `i` — Informational / ASCII art | White |

Binary, image, telnet, and PDF items are listed but cannot be opened in the terminal —
attempting to open one displays a descriptive error message.

---

## Encoding

TN3270 encoding is client-dependent:

- **Default (EBCDIC):** Strictly compliant clients (IBM PCOMM, Attachmate, x3270) expect the
  server to send EBCDIC. This is correct per RFC 1576 and is the default behaviour.
- **ASCII mode (`-ascii`):** Some modern clients (MochaSoft and similar) perform their own
  EBCDIC translation. Sending EBCDIC to these clients produces double-translation garbage.
  Use `-ascii` for these clients — the proxy will send raw ASCII bytes instead.

If you see garbled characters on screen, try toggling the `-ascii` flag.

---

## Architecture

| File | Responsibility |
|------|---------------|
| `main.go` | CLI flag parsing, TCP listener, goroutine dispatch |
| `session.go` | Per-connection state machine: TN3270 negotiation, AID input handling, navigation history, page management |
| `tn3270.go` | All TN3270/Telnet constants, AID codes, 3270 buffer address encoding/decoding |
| `ebcdic.go` | Full 256-entry EBCDIC ↔ ASCII translation tables and conversion functions |
| `gopher.go` | Gopher protocol client: fetch menus, text files, and search results; Gopher item type definitions |
| `screen.go` | 3270 datastream builder — color, highlight, field attributes, text wrapping, truncate/pad helpers |
| `render.go` | Screen renderers: menu, text viewer, search prompt, help, and error screens |

### Session state machine

Each TN3270 connection runs in its own goroutine and progresses through four states:

```
StateMenu ──Enter(navigable item)──► StateMenu   (another menu)
         ──Enter(text item)────────► StateText
         ──Enter(search item)──────► StateSearch
         ──PF1──────────────────────► StateHelp
         ──PF3──────────────────────► (previous state via history stack)
         ──PF12─────────────────────► StateMenu   (home server)

StateText ──PF3/PF7/PF8/Enter──────► (navigate / page)
          ──PF3───────────────────── ► (pop history → previous state)

StateSearch ──Enter───────────────── ► StateMenu  (search results)
            ──PF3─────────────────── ► StateMenu  (cancel)

StateHelp ──any key────────────────► (previous state, no history push)
```

### Navigation history

The history stack (`[]HistoryEntry`) stores `GopherLocation` + page offset. `pushHistory()`
is called before every navigation that changes location. `navigateBack()` (PF3) pops the
stack and restores both the location and the exact page position the user was on.

### 3270 buffer addresses

Buffer addresses use the IBM 3270 64-entry code table (not simply `0x40 + value`). The
`bufferAddress()` / `encode3270Addr()` / `decode3270Addr()` functions in `tn3270.go`
implement the canonical encoding. Addresses are clamped to the valid screen range
(0 – 1919 for an 80×24 display) so layout bugs produce visible artefacts rather than
client-side errors.

---

## Logging

All log lines include a timestamp and the client's remote address:

```
2024/01/15 14:23:01.123456 Gopher3270 proxy listening on 0.0.0.0:3270
2024/01/15 14:23:05.456789 New connection from 192.168.1.42:54321
2024/01/15 14:23:05.789012 [192.168.1.42:54321] Negotiation complete, terminal: IBM-3279-2-E
2024/01/15 14:23:06.012345 [192.168.1.42:54321] Navigating to gopher://gopherspace.dk:70/1
2024/01/15 14:23:06.345678 [192.168.1.42:54321] Session ended
```

With `-v`, each AID response is also logged:

```
2024/01/15 14:23:07.000000 [192.168.1.42:54321] AID=0xF8 cursor=240 fields=1
```

---

## Protocol Notes

- Gopher connections use a 10-second dial timeout and a 30-second read deadline (RFC 1436).
- Dot-stuffed lines (`..` at start) are unescaped per RFC 1436 § 3.
- The end-of-menu marker (`.` on a line by itself) terminates menu and text fetches.
- Search queries are sent as `selector\tquery\r\n` per RFC 1436 § 3.7.
- TN3270 datastream IAC bytes are doubled (escaped) on send and collapsed on receive.
- The proxy tolerates incomplete TN3270 negotiation — some minimal clients don't complete
  the full handshake but work fine once binary+EOR mode is established.

---

## License

MIT
