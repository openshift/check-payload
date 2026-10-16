package main

import "golang.org/x/crypto/argon2"

func unused() []byte { return argon2.Key([]byte("not executed")) }

func main() {}
