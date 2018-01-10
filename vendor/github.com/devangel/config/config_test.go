package config_test

import (
	"fmt"
	"github.com/devangel/config"
)

func ExampleNewConfig() {
	conf := config.NewConfig("myapp")
	fmt.Println(conf.Base)
	// Output: myapp
}

func ExampleConfig_Save() {
	conf := config.NewConfig("myapp")
	conf.Save("logins", "user@example.org", "12345")
	password, _ := conf.Load("logins", "user@example.org")
	fmt.Println(password)
	// Output: 12345
}

func ExampleConfig_Load() {
	conf := config.NewConfig("myapp")
	conf.Save("logins", "user@example.org", "12345")
	password, _ := conf.Load("logins", "user@example.org")
	fmt.Println(password)
	// Output: 12345
}

func ExampleConfig_List() {
	conf := config.NewConfig("myapp")
	conf.Save("logins", "user@example.org", "12345")
	conf.Save("logins", "user2@example.org", "12345")
	logins, _ := conf.List("logins")
	fmt.Println(logins)
	// Output: [user2@example.org user@example.org]
}

func ExampleConfig_Delete() {
	conf := config.NewConfig("myapp")
	conf.Save("logins", "user@example.org", "12345")
	conf.Save("logins", "user2@example.org", "12345")
	conf.Delete("logins", "user2@example.org")
	logins, _ := conf.List("logins")
	fmt.Println(logins)
	// Output: [user@example.org]
}
