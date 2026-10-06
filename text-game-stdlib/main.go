package main

import (
	"strings"
)

/*
	код писать в этом файле
	наверняка у вас будут какие-то структуры с методами, глобальные переменные ( тут можно ), функции
*/

type Player struct {
	currentRoom  *Room
	inventory    map[string]*Item
	haveBackpack bool
}

func NewPlayer(room *Room) *Player {
	return &Player{currentRoom: room, inventory: make(map[string]*Item)}
}

type Room struct {
	name             string
	items            map[string]*Item
	itemsOrder       []string
	paths            map[string]*Path
	pathsOrder       []string
	look             func(player *Player) string
	enterDescription string
}

type Item struct {
	name     string
	place    string
	wearable bool
	action   func(where string) string
}

type Path struct {
	name   string
	to     *Room
	locked bool
}

func main() {
	/*
		в этой функции можно ничего не писать,
		но тогда у вас не будет работать через go run main.go
		очень круто будет сделать построчный ввод команд тут, хотя это и не требуется по заданию
	*/

	initGame()

	_ = handleCommand("some command")
}

var kitchen = &Room{name: "кухня"}
var corridor = &Room{name: "коридор"}
var bedroom = &Room{name: "комната"}
var street = &Room{name: "улица"}

var player *Player

func initGame() {
	// kitchen
	kitchenPaths := make(map[string]*Path, 1)
	kitchenItems := make(map[string]*Item, 1)
	kitchenItems["чай"] = &Item{name: "чай", place: "на столе"}

	// corridor
	corridorPaths := make(map[string]*Path, 3)
	corridorItems := make(map[string]*Item, 0)

	// bedroom
	bedroomPaths := make(map[string]*Path, 1)
	bedroomItems := make(map[string]*Item, 3)
	bedroomItems["рюкзак"] = &Item{name: "рюкзак", place: "на стуле", wearable: true}
	bedroomItems["ключи"] = &Item{
		name:  "ключи",
		place: "на столе",
		action: func(where string) string {
			if where == "дверь" {
				corridorPaths["улица"].locked = false
				return "дверь открыта"
			}
			return "не к чему применить"
		},
	}
	bedroomItems["конспекты"] = &Item{name: "конспекты", place: "на столе"}

	// street
	streetPaths := make(map[string]*Path, 1)
	streetItems := make(map[string]*Item, 0)

	// paths initialization
	kitchenPaths["коридор"] = &Path{name: "коридор", to: corridor}
	bedroomPaths["коридор"] = &Path{name: "коридор", to: corridor}
	streetPaths["домой"] = &Path{name: "домой", to: corridor}
	corridorPaths["комната"] = &Path{name: "комната", to: bedroom}
	corridorPaths["кухня"] = &Path{name: "кухня", to: kitchen}
	corridorPaths["улица"] = &Path{name: "улица", to: street, locked: true}

	// pathsOrderInitialization
	kitchenPathsOrder := make([]string, 1)
	kitchenPathsOrder[0] = "коридор"

	bedroomPathsOrder := make([]string, 1)
	bedroomPathsOrder[0] = "коридор"

	streetPathsOrder := make([]string, 1)
	streetPathsOrder[0] = "домой"

	corridorPathsOrder := make([]string, 3)
	corridorPathsOrder[0] = "кухня"
	corridorPathsOrder[1] = "комната"
	corridorPathsOrder[2] = "улица"

	// itemsOrderInitialization
	kitchenItemsOrder := make([]string, 1)
	kitchenItemsOrder[0] = "чай"

	bedroomItemsOrder := make([]string, 3)
	bedroomItemsOrder[0] = "ключи"
	bedroomItemsOrder[1] = "конспекты"
	bedroomItemsOrder[2] = "рюкзак"

	corridorItemsOrder := make([]string, 0)
	streetItemsOrder := make([]string, 0)

	// items + paths to structures
	kitchen.paths = kitchenPaths
	kitchen.items = kitchenItems
	kitchen.pathsOrder = kitchenPathsOrder
	kitchen.itemsOrder = kitchenItemsOrder
	kitchen.enterDescription = "кухня, ничего интересного. "

	bedroom.paths = bedroomPaths
	bedroom.items = bedroomItems
	bedroom.pathsOrder = bedroomPathsOrder
	bedroom.itemsOrder = bedroomItemsOrder
	bedroom.enterDescription = "ты в своей комнате. "

	corridor.paths = corridorPaths
	corridor.items = corridorItems
	corridor.itemsOrder = corridorItemsOrder
	corridor.pathsOrder = corridorPathsOrder
	corridor.enterDescription = "ничего интересного. "

	street.paths = streetPaths
	street.items = streetItems
	street.pathsOrder = streetPathsOrder
	street.itemsOrder = streetItemsOrder
	street.enterDescription = "на улице весна. "

	// look function initialization
	kitchen.look = func(player *Player) string {
		res := "ты находишься на кухне, " + getListItemsInRoom(kitchen)
		if !player.haveBackpack {
			res += ", надо собрать рюкзак и идти в универ. "
		} else {
			res += ", надо идти в универ. "
		}

		return res + getListPaths(kitchen)
	}

	bedroom.look = func(player *Player) string {
		return getListItemsInRoom(bedroom) + ". " + getListPaths(bedroom)
	}

	street.look = func(player *Player) string {
		return "idk"
	}

	corridor.look = func(player *Player) string {
		return "ничего интересного. " + getListPaths(corridor)
	}

	// enter functions initialization
	// kitchen.enter = func(player *Player) string {
	// 	if _, ok := player.currentRoom.paths["kitchen"]; !ok {
	// 		return "нет пути в кухня"
	// 	}
	// }

	player = NewPlayer(kitchen)
	/*
		эта функция инициализирует игровой мир - все комнаты
		если что-то было - оно корректно перезатирается
	*/
}

func handleCommand(command string) string {
	args := strings.Split(command, " ")
	cmd := args[0]

	switch cmd {
	case "осмотреться":
		return player.currentRoom.look(player)
	case "идти":
		if len(args) < 2 || args[1] == "" {
			return "не хватает аругмента"
		}
		return enter(player, args[1])
	case "надеть":
		if len(args) < 2 || args[1] == "" {
			return "не хватает аругмента"
		}
		return wear(player, args[1])
	case "взять":
		if len(args) < 2 || args[1] == "" {
			return "не хватает аругмента"
		}
		return take(player, args[1])
	case "применить":
		if len(args) < 3 || args[1] == "" || args[2] == "" {
			return "не хватает аругментов"
		}
		return use(player, args[1], args[2])
	default:
		return "неизвестная команда"
	}
	/*
		данная функция принимает команду от "пользователя"
		и наверняка вызывает какой-то другой метод или функцию у "мира" - списка комнат
	*/
}

func getListPaths(room *Room) string {
	paths := room.pathsOrder
	res := "можно пройти - "
	temp := strings.Join(paths, ", ")

	return res + temp
}

func getListItemsInRoom(room *Room) string {
	items := room.itemsOrder
	placeToItems := make(map[string][]string, len(items))
	placeOrder := make([]string, 0)

	for _, i := range items {
		cur, ok := room.items[i]
		if !ok {
			continue
		}

		place := cur.place
		if _, ok := placeToItems[place]; !ok {
			placeToItems[place] = make([]string, 0)
			placeOrder = append(placeOrder, place)
		}
		placeToItems[place] = append(placeToItems[place], cur.name)
	}

	temp := make([]string, 0, len(placeOrder))

	for i := range placeOrder {
		list := placeToItems[placeOrder[i]]

		temp = append(temp, placeOrder[i]+": "+strings.Join(list, ", "))
	}

	res := strings.Join(temp, ", ")
	if res == "" {
		return "пустая комната"
	}
	return res
}

func enter(player *Player, pathName string) string {
	v, ok := player.currentRoom.paths[pathName]
	if !ok {
		return "нет пути в " + pathName
	}

	if v.locked {
		return "дверь закрыта"
	}

	player.currentRoom = v.to

	return v.to.enterDescription + getListPaths(player.currentRoom)
}

func wear(player *Player, itemName string) string {
	v, ok := player.currentRoom.items[itemName]
	if !ok {
		return "нет такого"
	}

	if !v.wearable {
		return "нельзя надеть"
	}

	delete(player.currentRoom.items, itemName)
	if itemName == "рюкзак" {
		player.haveBackpack = true
	}
	return "вы надели: " + itemName
}

func take(player *Player, itemName string) string {
	if !player.haveBackpack {
		return "некуда класть"
	}

	v, ok := player.currentRoom.items[itemName]
	if !ok {
		return "нет такого"
	}

	player.inventory[itemName] = v
	delete(player.currentRoom.items, itemName)
	return "предмет добавлен в инвентарь: " + itemName
}

func use(player *Player, who string, where string) string {
	item, ok := player.inventory[who]
	if !ok {
		return "нет предмета в инвентаре - " + who
	}

	if item.action == nil {
		return "не к чему применить"
	}
	return item.action(where)
}
