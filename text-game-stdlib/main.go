package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Item struct {
	name     string
	place    string
	wearable bool
}

type Exit struct {
	to     *Room
	locked bool
}

type Interaction struct {
	item    string
	perform func() string
}

type Room struct {
	name      string
	exits     map[string]*Exit
	exitOrder []string
	items     map[string]*Item
	itemOrder []string
	actions   map[string]Interaction
	onEnter   func(*Player) string
	onLook    func(*Player, *Room) string
}

type Player struct {
	currentRoom  *Room
	items        map[string]*Item
	haveBackpack bool
}

type commandHandler func([]string) string

var (
	kitchen, street, corridor, bedroom *Room
	player                             *Player
	commands                           map[string]commandHandler
)

func main() {
	initGame()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		fmt.Println(handleCommand(scanner.Text()))
	}
}

func NewPlayer() *Player {
	return &Player{
		currentRoom: kitchen,
		items:       make(map[string]*Item),
	}
}

func initGame() {
	kitchen = &Room{
		name:      "кухня",
		items:     make(map[string]*Item),
		itemOrder: []string{"чай"},
	}
	bedroom = &Room{
		name:      "комната",
		items:     make(map[string]*Item),
		itemOrder: []string{"ключи", "конспекты", "рюкзак"},
	}
	corridor = &Room{name: "коридор", items: make(map[string]*Item)}
	street = &Room{name: "улица", items: make(map[string]*Item)}

	kitchen.items["чай"] = &Item{name: "чай", place: "столе"}
	bedroom.items["ключи"] = &Item{name: "ключи", place: "столе"}
	bedroom.items["конспекты"] = &Item{name: "конспекты", place: "столе"}
	bedroom.items["рюкзак"] = &Item{name: "рюкзак", place: "стуле", wearable: true}

	kitchen.exits = map[string]*Exit{"коридор": {to: corridor}}
	kitchen.exitOrder = []string{"коридор"}
	bedroom.exits = map[string]*Exit{"коридор": {to: corridor}}
	bedroom.exitOrder = []string{"коридор"}
	corridor.exits = map[string]*Exit{
		"кухня":   {to: kitchen},
		"комната": {to: bedroom},
		"улица":   {to: street, locked: true},
	}
	corridor.exitOrder = []string{"кухня", "комната", "улица"}
	street.exits = map[string]*Exit{"домой": {to: corridor}}
	street.exitOrder = []string{"домой"}
	corridor.actions = map[string]Interaction{
		"дверь": {
			item: "ключи",
			perform: func() string {
				corridor.exits["улица"].locked = false
				return "дверь открыта"
			},
		},
	}

	kitchen.onEnter = func(*Player) string { return "кухня, ничего интересного" }
	kitchen.onLook = func(p *Player, room *Room) string {
		goal := "надо собрать рюкзак и идти в универ"
		if p.haveBackpack {
			goal = "надо идти в универ"
		}
		contents := describeItems(room)
		if contents == "" {
			contents = "ничего интересного"
		}
		return "ты находишься на кухне, " + contents + ", " + goal
	}
	bedroom.onEnter = func(*Player) string { return "ты в своей комнате" }
	bedroom.onLook = func(_ *Player, room *Room) string {
		if len(room.items) == 0 {
			return "пустая комната"
		}
		return describeItems(room)
	}
	corridor.onEnter = func(*Player) string { return "ничего интересного" }
	corridor.onLook = func(*Player, *Room) string { return "ничего интересного" }
	street.onEnter = func(*Player) string { return "на улице весна" }
	street.onLook = func(*Player, *Room) string { return "на улице весна" }

	player = NewPlayer()
	commands = map[string]commandHandler{
		"осмотреться": lookAround,
		"идти":        move,
		"взять":       take,
		"надеть":      wear,
		"применить":   use,
	}
}

func handleCommand(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "неизвестная команда"
	}

	handler, ok := commands[parts[0]]
	if !ok {
		return "неизвестная команда"
	}
	return handler(parts[1:])
}

func lookAround(args []string) string {
	if len(args) != 0 {
		return "неизвестная команда"
	}
	room := player.currentRoom
	return room.onLook(player, room) + ". " + describeExits(room)
}

func move(args []string) string {
	if len(args) != 1 {
		return "неизвестная команда"
	}

	exit, ok := player.currentRoom.exits[args[0]]
	if !ok {
		return "нет пути в " + args[0]
	}
	if exit.locked {
		return "дверь закрыта"
	}

	player.currentRoom = exit.to
	return exit.to.onEnter(player) + ". " + describeExits(exit.to)
}

func take(args []string) string {
	if len(args) != 1 {
		return "неизвестная команда"
	}
	if !player.haveBackpack {
		return "некуда класть"
	}

	name := args[0]
	item, ok := player.currentRoom.items[name]
	if !ok || item.wearable {
		return "нет такого"
	}
	delete(player.currentRoom.items, name)
	player.items[name] = item
	return "предмет добавлен в инвентарь: " + name
}

func wear(args []string) string {
	if len(args) != 1 {
		return "неизвестная команда"
	}

	name := args[0]
	item, ok := player.currentRoom.items[name]
	if !ok || !item.wearable {
		return "нет такого"
	}
	delete(player.currentRoom.items, name)
	player.haveBackpack = true
	return "вы надели: " + name
}

func use(args []string) string {
	if len(args) != 2 {
		return "неизвестная команда"
	}

	itemName, target := args[0], args[1]
	if _, ok := player.items[itemName]; !ok {
		return "нет предмета в инвентаре - " + itemName
	}
	action, ok := player.currentRoom.actions[target]
	if !ok || action.item != itemName {
		return "не к чему применить"
	}
	return action.perform()
}

func describeExits(room *Room) string {
	return "можно пройти - " + strings.Join(room.exitOrder, ", ")
}

func describeItems(room *Room) string {
	byPlace := make(map[string][]string)
	placeOrder := make([]string, 0)
	for _, name := range room.itemOrder {
		item, ok := room.items[name]
		if !ok {
			continue
		}
		if _, seen := byPlace[item.place]; !seen {
			placeOrder = append(placeOrder, item.place)
		}
		byPlace[item.place] = append(byPlace[item.place], item.name)
	}

	parts := make([]string, 0, len(placeOrder))
	for _, place := range placeOrder {
		parts = append(parts, "на "+place+": "+strings.Join(byPlace[place], ", "))
	}
	return strings.Join(parts, ", ")
}
