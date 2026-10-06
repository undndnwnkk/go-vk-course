# Text Adventure Game in Go

A simple text-based adventure game written in Go.

The project was created as a homework assignment to practice basic Go concepts such as structs, maps, functions, methods of modeling application state, and command parsing.

## About

The player moves between rooms, interacts with items, collects objects, and uses them to progress through the game.

The game world contains several rooms:

- Kitchen
- Corridor
- Bedroom
- Street

The player starts in the kitchen and needs to collect the necessary items before leaving the house.

## Available Commands

The game supports the following commands:

```text
осмотреться
идти <location>
взять <item>
надеть <item>
применить <item> <target>
```

Examples:

```text
осмотреться
идти коридор
идти комната
надеть рюкзак
взять ключи
взять конспекты
применить ключи дверь
идти улица
```

## Game Mechanics

The game keeps track of mutable state, including:

- the player's current room;
- the player's inventory;
- whether the backpack is equipped;
- items remaining in each room;
- available paths between rooms;
- locked and unlocked paths.

Rooms and items contain their own data and behavior where appropriate.

For example, the keys contain an action that can unlock the door, while paths keep track of whether they are currently locked.

## Project Structure

```text
.
├── main.go
├── main_test.go
└── README.md
```

`main.go` contains the game implementation.

`main_test.go` contains the provided test scenarios and must not be modified.

## Running the Tests

Run:

```bash
go test -v
```

The implementation passes all provided test cases.

## Running the Game

The main game logic is exposed through:

```go
initGame()
handleCommand(command string)
```

`initGame()` resets and initializes the game world.

`handleCommand()` parses a player's command, performs the corresponding action, updates the game state when necessary, and returns the result as a string.

## What I Practiced

This project helped me practice:

- Go structs
- maps and slices
- pointers
- functions as struct fields
- closures
- mutable application state
- string processing
- command parsing
- modeling relationships between objects
- separating generic game mechanics from object-specific behavior
- writing code against automated tests

## Testing

The behavior of the game is verified through sequences of commands that test both successful actions and error cases, such as:

- trying to move to an unavailable room;
- trying to leave through a locked door;
- taking an item without a backpack;
- taking an item that does not exist;
- using an item that is not in the inventory;
- applying an item to an invalid target;
- changing room state after taking or equipping items.

## Language

The source code is written in Go.

The game commands and responses are in Russian because they are defined by the assignment tests.