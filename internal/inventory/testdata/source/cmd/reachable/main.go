package main

import "golang.org/x/crypto/argon2"

func main() { println(len(argon2.Key([]byte("executed")))) }
