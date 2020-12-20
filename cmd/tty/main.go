package main

import (
	"cont/tty"
	"io"
	"log"
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Command("bash")

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	go io.Copy(inW, os.Stdin)
	go io.Copy(os.Stdout, outR)

	err := tty.Start(cmd, inR, outW)
	if err != nil {
		panic(err)
	}

	if err = cmd.Wait(); err != nil {
		log.Println(err)
		return
	}
}
