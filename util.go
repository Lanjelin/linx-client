package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/minio/sha256-simd"
)

var stdinReader = bufio.NewReader(os.Stdin)

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getInput(query string, allowBlank bool) (input string) {
	for {
		fmt.Print(query + ": ")
		line, err := stdinReader.ReadString('\n')
		checkErr(err)

		input = strings.TrimSpace(line)
		if input == "" && !allowBlank {
			continue
		}

		return
	}
}

func sha256sum(r io.Reader) string {
	hasher := sha256.New()

	_, err := io.Copy(hasher, r)
	if err == nil {
		return hex.EncodeToString(hasher.Sum(nil))
	} else {
		return ""
	}
}
