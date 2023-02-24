package main

import (
	"crypto/sha256"
	"fmt"
	"os"
)

func main() {
    h := sha256.New()

    content, err := os.ReadFile("./helloworld.txt")
    if err != nil {
        fmt.Println(err)
        os.Exit(0)
    }
    fmt.Println("File Content:", string(content))


    h.Write([]byte(content))
    sha256hashstring := fmt.Sprintf("%x", h.Sum(nil))
    fmt.Println("Hash: ", sha256hashstring)
}
