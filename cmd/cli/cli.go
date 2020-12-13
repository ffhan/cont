package main

import "cont/cmd"

func main() {
	cmd.Execute()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
