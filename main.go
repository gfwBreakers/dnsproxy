package main

func main() {
	if err := Init(); err != nil {
		return
	}
	if conf.Mode == "server" {
		Server()
	} else {
		Client()
	}
}
