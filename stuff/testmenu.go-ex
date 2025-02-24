package main

import (
	"fmt"

	"github.com/turret-io/go-menu/menu"
)

func cmd1(args ...string) error {
	// Do something
	fmt.Println("Output of cmd1")
	return nil
}

func cmd2(args ...string) error {
	//Do something
	fmt.Println("Output of cmd2")
	return nil
}

func main() {
	commandOptions := []menu.CommandOption{
		menu.CommandOption{"command1", "Runs command1", cmd1},
		menu.CommandOption{"command2", "Runs command2", cmd2},
	}

	menuOptions := menu.NewMenuOptions("'menu' for help > ", 0)

	menu := menu.NewMenu(commandOptions, menuOptions)
	menu.Start()
}
