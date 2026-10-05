# Text Adventure Game in Go

This repository contains the first homework assignment for a Go programming course. It is a small MUD-style text adventure in which the player explores rooms, collects items, and interacts with the game world by entering commands.

The complete assignment specification is available in [`hw1.md`](hw1.md).

## Objective

The player starts in the kitchen. To leave the house and go to university, the player needs to:

1. Go to the bedroom.
2. Put on the backpack.
3. Pick up the keys and lecture notes.
4. Return to the corridor.
5. Unlock the door with the keys.
6. Go outside.

## Available Commands

Commands are entered one per line. A command name and its arguments are separated by spaces.

| Command | Description | Example |
| --- | --- | --- |
| `осмотреться` | Inspect the current room, its items, and available exits | `осмотреться` |
| `идти <place>` | Move to an adjacent room | `идти коридор` |
| `взять <item>` | Put an item into the backpack | `взять ключи` |
| `надеть <item>` | Put on a wearable item | `надеть рюкзак` |
| `применить <item> <object>` | Use an inventory item on an object | `применить ключи дверь` |

The commands are intentionally written in Russian because the assignment tests expect exact Russian input and output. Unknown commands and invalid actions return an explanatory message without changing the game state.

## Requirements

- Go 1.20 or newer

No third-party dependencies are required.

## Running the Game

Start the interactive version with:

```bash
go run .
```

Then enter commands one per line, for example:

```text
осмотреться
идти коридор
идти комната
осмотреться
надеть рюкзак
взять ключи
```

To stop the program, send an end-of-file signal: press `Ctrl+D` on Linux or macOS, or `Ctrl+Z` followed by `Enter` on Windows.

## Testing

Run the test suite with:

```bash
go test -v
```

Run the standard Go static analysis tool with:

```bash
go vet ./...
```

## Architecture

The game world is represented by several small data structures:

- `Room` stores items, exits, and room-specific behavior;
- `Exit` connects two rooms and may be locked;
- `Item` describes an object and its position in a room;
- `Player` stores the current room and inventory;
- `Interaction` defines how an inventory item affects an object in a particular room.

Command handlers are stored in a map of functions. Item and exit display order is stored separately from the maps, ensuring deterministic output despite Go's unspecified map iteration order.

The `initGame` function constructs the world and its initial state. Calling it again resets all progress, allowing each test scenario to run independently.

## Project Structure

```text
.
├── go.mod        # Go module definition
├── hw1.md        # original assignment specification
├── main.go       # game implementation
├── main_test.go  # test scenarios
└── README.md     # project documentation
```
