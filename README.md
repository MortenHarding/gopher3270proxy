File breakdown:

main.go — CLI flags (-host, -port, -listen, -log, -v), accepts connections, spawns goroutines
tn3270.go — All TN3270/telnet constants, AID codes, buffer address encoding/decoding
ebcdic.go — Full EBCDIC↔ASCII translation tables (required for all 3270 I/O)
gopher.go — Gopher protocol client: fetches menus, text files, and search results
screen.go — 3270 datastream builder with color/highlight/field helpers
session.go — Telnet negotiation, AID input handling, navigation state machine, history stack
render.go — Screen renderers for menu, text viewer, search prompt, help, and error screens

To build and run on your server:
go build -o gopher3270 .
./gopher3270 -host gopherspace.dk -port 70 -listen 3270 -log /var/log/gopher3270.log -v
Key improvements over the existing executable:

Proper structured logging with timestamps and client IP
Full EBCDIC translation (not just ASCII pass-through)
Color-coded menu items (DIR=cyan, TXT=green, SRC=yellow, ERR=red)
Paged text viewer with line wrapping
Search server support (type 7)
Full navigation history stack (PF3 goes back correctly through multiple hops)
PF1 help screen with key reference
PF12 returns to your home server
